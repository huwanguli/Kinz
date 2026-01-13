package ziface

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
