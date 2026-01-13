package ziface

// 消息管理抽象层

type IMsgHandle interface {
	// DoMsgHandler 调度/执行对应的 Router 消息处理方法
	DoMsgHandler(request IRequest)
	// AddRouter 添加路由器
	AddRouter(msgId uint32, router IRouter)
	// StartWorkerPool 启动 Worker 工作池
	StartWorkerPool()
	// SendMsgToTaskQueue 将对应的消息交给对应的 Worker 进行业务处理
	SendMsgToTaskQueue(request IRequest)
}
