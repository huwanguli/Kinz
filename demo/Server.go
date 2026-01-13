package main

import (
	"fmt"
	"zinx/ziface"
	"zinx/zlog"
	"zinx/znet"
)

// 基于框架来开发的服务器端应用程序

type PingRouter struct {
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

func main() {
	zlog.L().InfoF("Start Server!")
	// 1.创建一个server
	s := znet.NewServer()
	// 2.配置路由
	s.AddRouter(0, &PingRouter{})
	// 3.启动服务
	s.Serve()
}
