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

type HelloZinxRouter struct {
	znet.BaseRouter
}

// Handle TEST
func (r *PingRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call Router Handle")
	// 读取客户端的数据，再回写 ping
	fmt.Println("ID:", request.GetMsgID(), ",Data: ", string(request.GetData()))

	err := request.GetConnection().SendMsg(200, []byte("ping"))
	if err != nil {
		fmt.Println(err)
	}

}

// Handle TEST Hello zINX
func (r *HelloZinxRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call HelloRouter Handle")
	// 读取客户端的数据，再回写 Hello Zinx
	fmt.Println("ID:", request.GetMsgID(), ",Data: ", string(request.GetData()))

	err := request.GetConnection().SendMsg(201, []byte("Hello Zinx!"))
	if err != nil {
		fmt.Println(err)
	}

}

func main() {
	// 1.创建一个server句柄，使用Zinx的Api
	s := znet.NewServer()
	// 2.添加自定义的Router功能
	s.AddRouter(0, &PingRouter{})
	s.AddRouter(1, &HelloZinxRouter{})
	// 3.启动server
	s.Serve()
}
