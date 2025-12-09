package ziface

import "net"

type IConnection interface {
	// Start 启动链接
	Start()

	// Stop 停止链接
	Stop()

	// GetTCPConnection 获取当前链接绑定的socket conn
	GetTCPConnection() *net.TCPConn

	// GetConnID 获取当前链接的ID
	GetConnID() uint32

	// GetRemoteAddr 获取远程的TCP状态 IP Port
	GetRemoteAddr() net.Addr

	// Send 发送数据， 将数据发送给远程的客户端
	Send(data []byte) error
}

// HandleFunc 定义一个处理链接业务的方法
type HandleFunc func(*net.TCPConn, []byte, int) error
