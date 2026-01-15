package ziface

import (
	"context"
	"net"
)

type IConnection interface {
	// Start 启动链接
	Start()

	// Stop 停止链接
	Stop()

	// Context REPRO : 上下文
	Context() context.Context

	// GetName 等 REPRO : 新增功能
	GetName() string
	GetConnection() net.Conn

	// GetTCPConnection 暂时不增加 Websocket功能
	//GetWsConn() *websocket.Conn
	// 获取当前链接绑定的socket conn // TODEL: 使用 GetConnection()
	GetTCPConnection() net.Conn

	// GetConnID 获取当前链接的ID
	GetConnID() uint64

	GetConnIdStr() string
	GetMsgHandler() IMsgHandle
	GetWorkerID() uint32
	// RemoteAddr 获取远程的TCP状态 IP Port
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	LocalAddrString() string
	RemoteAddrString() string
	// Send 发送数据， 将数据发送给远程的客户端 无缓冲
	Send(data []byte) error
	SendToQueue(data []byte) error // 有缓冲
	// SendMsg 旧方法，无缓冲
	SendMsg(msgID uint32, data []byte) error
	SendBuffMsg(msgID uint32, data []byte) error
	// SetProperty 设置链接属性
	SetProperty(key string, value interface{})

	// GetProperty 获取链接属性
	GetProperty(key string) (interface{}, error)

	// RemoveProperty 移除链接属性
	RemoveProperty(key string)

	IsAlive() bool

	SetHeartBeat(checker IHeartbeatChecker)

	AddCloseCallback(handler, key interface{}, callback func())
	RemoveCloseCallback(handler, key interface{})
	InvokeCloseCallbacks()
}
