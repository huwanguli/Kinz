package main

import (
	"fmt"
	"zinx/ziface"
	"zinx/znet"
)

// 基于Zinx框架来开发的服务器端应用程序

type PingRouter struct {
	znet.BaseRouter
}

// PreHandle TEST
func (r *PingRouter) PreHandle(request ziface.IRequest) {
	fmt.Println("Call Router PreHandle")
	_, err := request.GetConnection().GetTCPConnection().Write([]byte("before ping..\n"))
	if err != nil {
		fmt.Println("call router PreHandle err:", err)
	}
}

// Handle TEST
func (r *PingRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call Router Handle")
	_, err := request.GetConnection().GetTCPConnection().Write([]byte("ping..ping..ping..\n"))
	if err != nil {
		fmt.Println("call router Handle err:", err)
	}
}

// PostHandle TEST
func (r *PingRouter) PostHandle(request ziface.IRequest) {
	fmt.Println("Call Router PostHandle")
	_, err := request.GetConnection().GetTCPConnection().Write([]byte("After ping..\n"))
	if err != nil {
		fmt.Println("call router PostHandle err:", err)
	}
}

func main() {
	// 1.创建一个server句柄，使用Zinx的Api
	s := znet.NewServer()
	// 2.添加自定义的Router功能 （Ping test）
	s.AddRouter(&PingRouter{})
	// 3.启动server
	s.Serve()
}
