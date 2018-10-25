//  func handler(w http.ResponseWriter, r *http.Request) {
//	  w.Write([]byte("WORLD!"))
//  }
//
//  func main() {
//      mux := http.NewServeMux()
//      mux.HandleFunc("/hello", handler)
//
//	    err := grace.ListenAndServe("localhost:8080", mux)
//      if err != nil {
//		   log.Println(err)
//	    }
//      log.Println("Server on 8080 stopped")
//	     os.Exit(0)
//    }
package grace

import (
	"flag"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// PreSignal is the position to add filter before signal
	PreSignal = iota
	// PostSignal is the position to add filter after signal
	PostSignal

	// app启动
	StateInit
	// app运行中
	StateRunning
	// app关闭中
	StateShuttingDown
	// app彻底关闭
	StateTerminate
)

var (
	regLock             *sync.Mutex
	runningServers      map[string]*Server // IP地址 : 运行中的Server实例, 默认是1个
	runningServersOrder []string           // 运行中的Server实例, 默认1个

	runningServersForked bool // 热升级Fork状态控制

	// the HTTP read timeout
	DefaultReadTimeOut time.Duration
	// the HTTP Write timeout
	DefaultWriteTimeOut time.Duration
	// the Max HTTP Header size, default is 0, no limit
	DefaultMaxHeaderBytes int
	// the shutdown server's timeout. default is 60s
	DefaultTimeout = 60 * time.Second

	isChild            bool            // 监听打开的文件(after forking)
	socketOrder        string          // socket标记, 传入参数字符串, 用户启动时候输入
	socketPtrOffsetMap map[string]uint // socket标记 : 顺序(从0开始), 默认是1个

	hookableSignals []os.Signal // 需要监听的信号
)

func init() {
	flag.BoolVar(&isChild, "graceful", false, "listen on open fd (after forking)")
	flag.StringVar(&socketOrder, "socketorder", "", "previous initialization order - used when more than one listener was started")

	regLock = &sync.Mutex{}
	runningServers = make(map[string]*Server)
	runningServersOrder = []string{}
	socketPtrOffsetMap = make(map[string]uint)

	hookableSignals = []os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
	}
}

// 可以多次调用, 产生多个Server实例
func NewServer(addr string, handler http.Handler) (srv *Server) {
	regLock.Lock()
	defer regLock.Unlock()

	if !flag.Parsed() {
		flag.Parse()
	}

	// socketOrder, 命令行控制, socketPtrOffsetMap一直保持不变;
	// 程序控制, 每产生一个Server实例, 会动态修改socketPtrOffsetMap
	if len(socketOrder) > 0 {
		for i, addr := range strings.Split(socketOrder, ",") {
			socketPtrOffsetMap[addr] = uint(i)
		}
	} else {
		socketPtrOffsetMap[addr] = uint(len(runningServersOrder))
	}

	srv = &Server{
		wg:      sync.WaitGroup{},
		sigChan: make(chan os.Signal),
		isChild: isChild,
		SignalHooks: map[int]map[os.Signal][]func(){
			PreSignal: {
				syscall.SIGHUP:  {},
				syscall.SIGINT:  {},
				syscall.SIGTERM: {},
			},
			PostSignal: {
				syscall.SIGHUP:  {},
				syscall.SIGINT:  {},
				syscall.SIGTERM: {},
			},
		},
		state:   StateInit, // app init
		Network: "tcp",     // 网络协议
	}
	srv.Server = &http.Server{}
	srv.Server.Addr = addr
	srv.Server.ReadTimeout = DefaultReadTimeOut
	srv.Server.WriteTimeout = DefaultWriteTimeOut
	srv.Server.MaxHeaderBytes = DefaultMaxHeaderBytes
	srv.Server.Handler = handler

	// 运行Server实例
	runningServersOrder = append(runningServersOrder, addr)
	runningServers[addr] = srv

	return
}

func ListenAndServe(addr string, handler http.Handler) error {
	server := NewServer(addr, handler)
	return server.ListenAndServe()
}

func ListenAndServeTLS(addr string, certFile string, keyFile string, handler http.Handler) error {
	server := NewServer(addr, handler)
	return server.ListenAndServeTLS(certFile, keyFile)
}
