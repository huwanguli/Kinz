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

// Handle TEST
func (r *PingRouter) Handle(request ziface.IRequest) {
	fmt.Println("Call Router Handle")
	// 读取客户端的数据，再回写 ping
	fmt.Println("ID:", request.GetMsgID(), ",Data: ", string(request.GetData()))

	err := request.GetConnection().SendMsg(1, []byte("ping"))
	if err != nil {
		fmt.Println(err)
	}

}

func main() {
	// 1.创建一个server句柄，使用Zinx的Api
	s := znet.NewServer("[zinx V0.5]")
	// 2.添加自定义的Router功能
	s.AddRouter(&PingRouter{})
	// 3.启动server
	s.Serve()
}
