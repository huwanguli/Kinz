package main

import (
	"fmt"
	"zinx/ziface"
	"zinx/znet"
)

// 基于 Zinx 框架来开发的服务器端应用程序

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

// Handle TEST Hello Zinx
func (r *HelloZinxRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call HelloRouter Handle")

	// 读取客户端的数据，再回写 Hello Zinx
	fmt.Println("ID:", request.GetMsgID(), ",Data: ", string(request.GetData()))

	err := request.GetConnection().SendMsg(201, []byte("Hello!"))
	if err != nil {
		fmt.Println(err)
	}

}

// DoConnectionBegin 创建链接之后执行的 Hook 函数
func DoConnectionBegin(conn ziface.IConnection) {
	fmt.Println("DoConnectionBegin begin is Called...")
	err := conn.SendMsg(202, []byte("DoConnectionBegin"))
	if err != nil {
		return
	}
	// 给当前的链接设置一些属性
	conn.SetProperty("name", "Kanon")
	conn.SetProperty("email", "1845990348@qq.com")
}

// DoConnectionLost 链接断开前执行的Hook函数
func DoConnectionLost(conn ziface.IConnection) {
	fmt.Println("DoConnectionLost  is Called...")
	fmt.Println("Conn ID:", conn.GetConnID(), "isLost")
	if value, err := conn.GetProperty("name"); err == nil {
		fmt.Println(value)
	}
	if value, err := conn.GetProperty("email"); err == nil {
		fmt.Println(value)
	}
}

func main() {
	// 1.创建一个server句柄，使用Zinx的Api
	s := znet.NewServer()

	// 测试注册链接的 Hook 函数
	s.SetOnConnStart(DoConnectionBegin)
	s.SetOnConnStop(DoConnectionLost)

	// 2.添加自定义的 Router 功能
	s.AddRouter(0, &PingRouter{})
	s.AddRouter(1, &HelloZinxRouter{})

	// 3.启动 server
	s.Serve()
}
