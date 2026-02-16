package apis

import (
	"fmt"
	"zinx/mmo_game_zinx/core"
	"zinx/mmo_game_zinx/pb"
	"zinx/ziface"
	"zinx/znet"

	"google.golang.org/protobuf/proto"
)

// WorldChat 世界聊天的 路由业务
type WorldChat struct {
	znet.BaseRouter
}

func (wc *WorldChat) Handle(request ziface.IRequest) {
	// 1.解析客户端传递进来的 proto 协议
	protoMsg := &pb.Talk{}
	err := proto.Unmarshal(request.GetData(), protoMsg)
	if err != nil {
		fmt.Println("Talk proto Unmarshal error :", err)
	}

	// 2.当前的聊天数据属于哪个玩家
	pid, err := request.GetConnection().GetProperty("pid")
	if err != nil {
		fmt.Println("Talk GetProperty error :", err)
	}

	// 3. 根据 pid 得到对应的 player 对象
	player := core.WorldMgrObj.GetPlayerByPid(pid.(int32))

	// 4. 广播给全部在线的玩家
	player.Talk(protoMsg.GetContent())
}
