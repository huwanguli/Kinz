package main

import (
	"fmt"
	"zinx/mmo_game_zinx/apis"
	"zinx/mmo_game_zinx/core"
	"zinx/ziface"
	"zinx/znet"
)

// OnConnectionAdd 当前客户端建立连接之后的 HOOK 函数
func OnConnectionAdd(conn ziface.IConnection) {
	// 创建一个 Player 对象
	player := core.NewPlayer(conn)

	// 给客户端发送 MsgID 为 1 的消息 : 同步当前 Player 的 ID 给客户端
	player.SyncPid()
	// 给客户端发送 MsgID 为 200 的消息
	player.BroadCastStartPosition()

	// 将该玩家添加到 WorldMgr 中
	core.WorldMgrObj.AddPlayer(player)

	// 将该连接绑定一个 Pid 的属性
	conn.SetProperty("pid", player.Pid)

	// 同步周边玩家，告知其当前玩家已上线， 即广播当前玩家的位置信息
	player.SyncSurrounding()

	fmt.Println("=====> Player Pid = ", player.Pid, "is arrived <=====")
}

func main() {
	// 创建 zinx server 句柄
	s := znet.NewServer()

	// 连接创建和销毁的 HOOK 钩子函数
	s.SetOnConnStart(OnConnectionAdd)

	// 注册一些路由业务
	s.AddRouter(2, &apis.WorldChat{})
	// 启动服务
	s.Serve()
}
