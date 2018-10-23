package grace

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// 在http.Server的基础上增加新功能
type Server struct {
	*http.Server
	GraceListener    net.Listener
	SignalHooks      map[int]map[os.Signal][]func() // int 0/1 信号的前置工作/后置工作, os.Signal是特定的系统信号
	tlsInnerListener *graceListener
	wg               sync.WaitGroup
	sigChan          chan os.Signal
	isChild          bool
	state            uint8
	Network          string
}

// 对http.Server的Serve进行封装, 增加了处理工作
func (srv *Server) Serve() (err error) {
	srv.state = StateRunning
	err = srv.Server.Serve(srv.GraceListener)
	log.Println(syscall.Getpid(), "Waiting for connections to finish...")
	srv.wg.Wait()
	srv.state = StateTerminate
	return
}

// 重写了http.Server的ListenAndServe方法
func (srv *Server) ListenAndServe() (err error) {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}

	go srv.handleSignals()

	l, err := srv.getListener(addr)
	if err != nil {
		log.Println(err)
		return err
	}

	srv.GraceListener = newGraceListener(l, srv)

	if srv.isChild {
		process, err := os.FindProcess(os.Getppid())
		if err != nil {
			log.Println(err)
			return err
		}
		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			return err
		}
	}

	log.Println(os.Getpid(), srv.Addr)
	return srv.Serve()
}

// ListenAndServeTLS listens on the TCP network address srv.Addr and then calls
// Serve to handle requests on incoming TLS connections.
//
// Filenames containing a certificate and matching private key for the server must
// be provided. If the certificate is signed by a certificate authority, the
// certFile should be the concatenation of the server's certificate followed by the
// CA's certificate.
//
// If srv.Addr is blank, ":https" is used.
func (srv *Server) ListenAndServeTLS(certFile, keyFile string) (err error) {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	if srv.TLSConfig == nil {
		srv.TLSConfig = &tls.Config{}
	}
	if srv.TLSConfig.NextProtos == nil {
		srv.TLSConfig.NextProtos = []string{"http/1.1"}
	}

	srv.TLSConfig.Certificates = make([]tls.Certificate, 1)
	srv.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}

	go srv.handleSignals()

	l, err := srv.getListener(addr)
	if err != nil {
		log.Println(err)
		return err
	}

	srv.tlsInnerListener = newGraceListener(l, srv)
	srv.GraceListener = tls.NewListener(srv.tlsInnerListener, srv.TLSConfig)

	if srv.isChild {
		process, err := os.FindProcess(os.Getppid())
		if err != nil {
			log.Println(err)
			return err
		}
		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			return err
		}
	}
	log.Println(os.Getpid(), srv.Addr)
	return srv.Serve()
}

// ListenAndServeMutualTLS listens on the TCP network address srv.Addr and then calls
// Serve to handle requests on incoming mutual TLS connections.
func (srv *Server) ListenAndServeMutualTLS(certFile, keyFile, trustFile string) (err error) {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	if srv.TLSConfig == nil {
		srv.TLSConfig = &tls.Config{}
	}
	if srv.TLSConfig.NextProtos == nil {
		srv.TLSConfig.NextProtos = []string{"http/1.1"}
	}

	srv.TLSConfig.Certificates = make([]tls.Certificate, 1)
	srv.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}
	srv.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	pool := x509.NewCertPool()
	data, err := ioutil.ReadFile(trustFile)
	if err != nil {
		log.Println(err)
		return err
	}
	pool.AppendCertsFromPEM(data)
	srv.TLSConfig.ClientCAs = pool
	log.Println("Mutual HTTPS")
	go srv.handleSignals()

	l, err := srv.getListener(addr)
	if err != nil {
		log.Println(err)
		return err
	}

	srv.tlsInnerListener = newGraceListener(l, srv)
	srv.GraceListener = tls.NewListener(srv.tlsInnerListener, srv.TLSConfig)

	if srv.isChild {
		process, err := os.FindProcess(os.Getppid())
		if err != nil {
			log.Println(err)
			return err
		}
		err = process.Kill()
		if err != nil {
			return err
		}
	}
	log.Println(os.Getpid(), srv.Addr)
	return srv.Serve()
}

// getListener either opens a new socket to listen on, or takes the acceptor socket
// it got passed when restarted.
func (srv *Server) getListener(laddr string) (l net.Listener, err error) {
	if srv.isChild {
		var ptrOffset uint
		if len(socketPtrOffsetMap) > 0 {
			ptrOffset = socketPtrOffsetMap[laddr]
			log.Println("laddr", laddr, "ptr offset", socketPtrOffsetMap[laddr])
		}

		f := os.NewFile(uintptr(3+ptrOffset), "")
		l, err = net.FileListener(f)
		if err != nil {
			err = fmt.Errorf("net.FileListener error: %v", err)
			return
		}
	} else {
		l, err = net.Listen(srv.Network, laddr)
		if err != nil {
			err = fmt.Errorf("net.Listen error: %v", err)
			return
		}
	}
	return
}

// 处理注册的Signal函数
func (srv *Server) handleSignals() {
	var sig os.Signal

	// signal.Notify(c chan<- os.Signal, sig ...os.Signal)
	// Notify()函数让signal包将输入信号转发到c.
	// 如果没有列出要传递的信号,会将所有输入信号传递到c; 否则只传递列出的输入信号.

	// signal包不会为了向c发送信息而阻塞(即如果发送时c阻塞了, signal包会直接放弃);
	// 调用者应该保证c有足够的缓存空间可以跟上期望的信号频率. 对使用单一信号用于通知的通道,缓存为1就足够了.

	// 注册接受Signal通知的channel
	signal.Notify(
		srv.sigChan,
		hookableSignals...,
	)

	pid := syscall.Getpid()
	for {
		sig = <-srv.sigChan             // 阻塞, 直到特定的Signal产生
		srv.signalHooks(PreSignal, sig) // Before
		switch sig {
		case syscall.SIGHUP:
			log.Println(pid, "Received SIGHUP. forking.")
			err := srv.fork()
			if err != nil {
				log.Println("Fork err:", err)
			}
		case syscall.SIGINT:
			log.Println(pid, "Received SIGINT.")
			srv.shutdown()
		case syscall.SIGTERM:
			log.Println(pid, "Received SIGTERM.")
			srv.shutdown()
		default:
			log.Printf("Received %v: nothing i care about...\n", sig)
		}
		srv.signalHooks(PostSignal, sig) // After
	}
}

func (srv *Server) signalHooks(ppFlag int, sig os.Signal) {
	if _, notSet := srv.SignalHooks[ppFlag][sig]; !notSet {
		return
	}
	for _, f := range srv.SignalHooks[ppFlag][sig] {
		f()
	}
}

func (srv *Server) shutdown() {
	if srv.state != StateRunning {
		return
	}

	srv.state = StateShuttingDown
	// 延时处理机制
	if DefaultTimeout >= 0 {
		go srv.serverTimeout(DefaultTimeout)
	}
	// 关闭listener, 不再接受新的请求
	err := srv.GraceListener.Close()
	if err != nil {
		log.Println(syscall.Getpid(), "Listener.Close() error:", err)
	} else {
		log.Println(syscall.Getpid(), srv.GraceListener.Addr(), "Listener closed.")
	}
}

// 强制服务器在给定的超时时间内关闭 - 无论是否完成未完成的请求.
func (srv *Server) serverTimeout(d time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("WaitGroup at 0", r)
		}
	}()
	if srv.state != StateShuttingDown {
		return
	}
	time.Sleep(d)
	log.Println("[STOP - Hammer Time] Forcefully shutting down parent")

	// 执行wg.Done(), 关闭所有的goroutine, 然后退出.
	for {
		if srv.state == StateTerminate {
			break
		}
		srv.wg.Done()
	}
}

// 生成新的子进程
func (srv *Server) fork() (err error) {
	regLock.Lock()
	defer regLock.Unlock()
	if runningServersForked {
		return
	}
	runningServersForked = true

	var files = make([]*os.File, len(runningServers))
	var orderArgs = make([]string, len(runningServers))
	for _, srvPtr := range runningServers {
		switch srvPtr.GraceListener.(type) {
		case *graceListener:
			files[socketPtrOffsetMap[srvPtr.Server.Addr]] = srvPtr.GraceListener.(*graceListener).File()
		default:
			files[socketPtrOffsetMap[srvPtr.Server.Addr]] = srvPtr.tlsInnerListener.File()
		}
		orderArgs[socketPtrOffsetMap[srvPtr.Server.Addr]] = srvPtr.Server.Addr
	}

	log.Println(files)
	path := os.Args[0]
	var args []string
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg == "-graceful" {
				break
			}
			args = append(args, arg)
		}
	}
	args = append(args, "-graceful")
	if len(runningServers) > 1 {
		args = append(args, fmt.Sprintf(`-socketorder=%s`, strings.Join(orderArgs, ",")))
		log.Println(args)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = files
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Restart: Failed to launch, error: %v", err)
	}

	return
}

// RegisterSignalHook registers a function to be run PreSignal or PostSignal for a given signal.
func (srv *Server) RegisterSignalHook(ppFlag int, sig os.Signal, f func()) (err error) {
	if ppFlag != PreSignal && ppFlag != PostSignal {
		err = fmt.Errorf("Invalid ppFlag argument. Must be either grace.PreSignal or grace.PostSignal")
		return
	}
	for _, s := range hookableSignals {
		if s == sig {
			srv.SignalHooks[ppFlag][sig] = append(srv.SignalHooks[ppFlag][sig], f)
			return
		}
	}
	err = fmt.Errorf("Signal '%v' is not supported", sig)
	return
}
