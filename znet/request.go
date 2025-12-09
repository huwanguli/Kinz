package znet

import "zinx/ziface"

type Request struct {
	// 已经和客户端建立好的链接
	Conn ziface.IConnection

	// 客户端请求的数据
	msg ziface.IMessage
}

// GetConnection 得到当前的链接
func (r *Request) GetConnection() ziface.IConnection {
	return r.Conn
}

// GetData 得到当前的数据
func (r *Request) GetData() []byte {
	return r.msg.GetData()
}

// GetMsgID 得到当前的数据ID
func (r *Request) GetMsgID() uint32 {
	return r.msg.GetMsgId()
}
