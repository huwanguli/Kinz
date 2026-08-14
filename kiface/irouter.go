package kiface

// IRouter 路由的抽象接口
// 路由里的数据都是Request

type IRouter interface {
	// PreHandle 处理conn业务之前的方法
	PreHandle(request IRequest)
	// Handle 处理Conn业务的主方法
	Handle(request IRequest)
	// PostHandle 处理Conn业务之后的方法
	PostHandle(request IRequest)
}

//After: 新版路由方式

type RouterHandler func(request IRequest)
type IRouterSlices interface {
	// Use 添加全局组件
	Use(Handler ...RouterHandler)

	// AddHandler 添加业务处理器集合
	AddHandler(magId uint32, handler ...RouterHandler)

	// Group 路由群组管理
	Group(start, end uint32, Handler ...RouterHandler) IGroupRouterSlices

	// GetHandlers 获得当前的所有注册在MsgId的处理器集合
	GetHandlers(MsgId uint32) ([]RouterHandler, bool)
}

type IGroupRouterSlices interface {
	Use(Handler ...RouterHandler)
	AddHandler(MsgId uint32, handler ...RouterHandler)
}
