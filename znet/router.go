package znet

import "zinx/ziface"

// BaseRouter 实现router时先嵌入该基类，根据需要对该基类方法进行重写
type BaseRouter struct{}

// 之所以BaseRouter的方法都为空，是因为有些router不需要实现全部三个方法
// 按需要实现方法即可

// PreHandle 处理 conn 业务之前的钩子方法Hook
func (br *BaseRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理Conn业务的主方法Hook
func (br *BaseRouter) Handle(request ziface.IRequest) {}

// PostHandle 处理Conn业务之后的方法Hook
func (br *BaseRouter) PostHandle(request ziface.IRequest) {}
