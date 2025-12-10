package ziface

// 消息管理抽象层

type IMsgHandle interface {
	// DoMsgHandler 调度/执行对应的Router消息处理方法
	DoMsgHandler(request IRequest)
	// AddRouter 添加路由器
	AddRouter(msgId uint32, router IRouter)
}
