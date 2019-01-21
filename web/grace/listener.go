package grace

import (
	"net"
	"os"
	"syscall"
	"time"
)

// 监听, 在net.Listener的基础上增加控制
type graceListener struct {
	net.Listener
	stop    chan error
	stopped bool
	server  *Server
}

func newGraceListener(l net.Listener, srv *Server) (el *graceListener) {
	el = &graceListener{
		Listener: l,
		stop:     make(chan error),
		server:   srv,
	}
	// 为Close()做准备
	go func() {
		<-el.stop // 后台监听, 调用Close()之后, 解除阻塞
		el.stopped = true
		el.stop <- el.Listener.Close()
	}()
	return
}

// 只接受TCP协议传输的请求
func (gl *graceListener) Accept() (c net.Conn, err error) {
	tc, err := gl.Listener.(*net.TCPListener).AcceptTCP()
	if err != nil {
		return
	}

	// 长连接
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)

	c = &graceConn{
		Conn:   tc,
		server: gl.server,
	}

	gl.server.wg.Add(1) // 接受一次请求
	return
}

// 首次调用Close(), 返回的结果是Listener.Close()的结果
// 非首次调用, 直接返回 syscall.EINVAL
func (gl *graceListener) Close() error {
	if gl.stopped {
		return syscall.EINVAL
	}
	gl.stop <- nil   // 解除启动时的阻塞
	return <-gl.stop // 阻塞返回
}

// 获取Listener的文件描述符, 每一个Listener都有一个唯一的fd
func (gl *graceListener) File() *os.File {
	// returns a dup(2) - FD_CLOEXEC flag *not* set
	tl := gl.Listener.(*net.TCPListener)
	fl, _ := tl.File()
	return fl
}
