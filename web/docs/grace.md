## grace解析

```
数据结构:
Server {
    *http.Server
    GraceListener    net.Listener
    tlsInnerListener *graceListener
}

graceConn {
    net.Conn
    server *Server
}

graceListener {
	net.Listener
	server  *Server
}
```

### 数据结构说明
```
graceListener, 重写了 Accept() 方法, 建立的连接都属于长连接(3 min)

graceConn, 重写了 Close() 方法

graceListener和graceConn持有Server对象,主要是对这个Server的wg的控制
```