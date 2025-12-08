package main

import "zinx/znet"

// 基于Zinx框架来开发的服务器端应用程序

func main() {
	// 1.创建一个server句柄，使用Zinx的Api
	s := znet.NewServer("[zinx V0.1]")
	// 2.启动server
	s.Serve()
}
