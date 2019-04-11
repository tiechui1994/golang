package httpd

/**
HTTP Server的实现:

核心接口:

type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}

1.ServeHTTP() 需要写入响应头(Header) 和 数据到ResponseWriter, 然后返回一个signal告知request当前请求
已经完成.

2.在完成ServeHTTP调用之后, 向 ResponseWriter写入数据 或 从Request.Body读取数据是非法的.

3.根据HTTP客户端软件, HTTP协议版本以及客户端和Go服务器之间的任何中介, 一旦向ResponseWriter写入内容之后, 就
无法再从Request.Body中读取数据. 谨慎的处理程序应首先读取Request.Body, 然后写入数据.

4. 对于request,除了读取body的内容,handler尽量不要修改request的内容.
*/

/*
type ResponseWriter interface {
	// 在
	Header() Header
	Write([]byte) (int, error)
	WriteHeader(int)
}

type Flusher interface {
	Flush() //
}

type Hijacker interface {
	Hijack() (net.Conn, *bufio.ReadWriter, error)
}

type CloseNotifier interface {
	CloseNotify() <-chan bool
}


*/
