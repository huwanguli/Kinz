package apis

import (
	"fmt"
	"zinx/mmo_game_zinx/core"
	"zinx/mmo_game_zinx/pb"
	"zinx/ziface"
	"zinx/znet"

	"google.golang.org/protobuf/proto"
)

type MoveApi struct {
	znet.BaseRouter
}

func (m *MoveApi) Handle(request ziface.IRequest) {
	// 1.解析客户端传进来的 proto 数据
	protoMsg := &pb.Position{}
	err := proto.Unmarshal(request.GetData(), protoMsg)
	if err != nil {
		fmt.Println("Move proto Unmarshal err:", err)
		return
	}
	// 2. 得到当前发送位置的是哪个玩家
	pid, err := request.GetConnection().GetProperty("pid")
	if err != nil {
		fmt.Println("Move GetProperty err:", err)
		return
	}
	fmt.Printf("Player pid = %d, move(%f,%f,%f,%f)\n", pid, protoMsg.X, protoMsg.Y, protoMsg.Z, protoMsg.V)
	// 3. 给其他玩家进行当前玩家的位置信息广播
	player := core.WorldMgrObj.GetPlayerByPid(pid.(int32))
	// 广播并更新当前玩家的坐标
	player.UpdatePos(protoMsg.X, protoMsg.Y, protoMsg.Z, protoMsg.V)
}
