package ziface

// IRouter 路由的抽象接口
// 路由里的数据都是Request
type IRouter interface {
	// PreHandle 处理conn业务之前的钩子方法Hook
	PreHandle(request IRequest)

	// Handle 处理Conn业务的主方法Hook
	Handle(request IRequest)

	// PostHandle 处理Conn业务之后的方法Hook
	PostHandle(request IRequest)
}
