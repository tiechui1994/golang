package socket

/**
socket:
	常用的Socket类型: 流式Socket(SOCK_STREAM) 和 数据报式Socket(SOCK_DGRAM)
	流式Socket(SOCK_STREAM): TCP
	数据报式Socket(SOCK_DGRAM): UDP


网络中进程直接通信:
	解决的问题是如何唯一标识一个进程? 本地通信可以使用进程PID来唯一标示一个进程.
	网络层的ip地址唯一标识网络中的主机, 传输层的"协议+端口"可以唯一标识主机中的进程, 因此,
	(ip, 协议, 端口) 唯一标识一个进程

*/
