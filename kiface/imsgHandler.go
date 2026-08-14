package kiface

// 消息管理抽象层

type IMsgHandle interface {
	// AddRouter DoMsgHandler 调度/执行对应的 Router 消息处理方法 TODEL:不在这里进行调用
	// DoMsgHandler(request IRequest)

	// AddRouter 添加路由器
	AddRouter(msgId uint32, router IRouter)

	// AddRouterSlices 新版路由方法
	AddRouterSlices(msgId uint32, handler ...RouterHandler) IRouterSlices
	Group(start, end uint32, Handlers ...RouterHandler) IGroupRouterSlices
	Use(Handler ...RouterHandler) IRouterSlices
	// StartWorkerPool 启动 Worker 工作池
	StartWorkerPool()
	// SendMsgToTaskQueue 将对应的消息交给对应的 Worker 进行业务处理
	SendMsgToTaskQueue(request IRequest)
	Execute(request IRequest) //执行责任链上的拦截器方法
	AddInterceptor(interceptor IInterceptor)
	SetHeadInterceptor(interceptor IInterceptor)
}
