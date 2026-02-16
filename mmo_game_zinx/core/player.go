package core

import (
	"fmt"
	"math/rand"
	"sync"
	"zinx/mmo_game_zinx/pb"
	"zinx/ziface"

	"google.golang.org/protobuf/proto"
)

type Player struct {
	Pid  int32
	Conn ziface.IConnection // 当前玩家连接（用于和客户端的连接）
	X    float32            // 平面的 X 坐标
	Y    float32            // 高度
	Z    float32            // 平面的 Y 坐标
	V    float32            // 旋转的角度
}

/*
	PlayerID 生成器
*/

var PidGen int32 = 1
var IdLock sync.Mutex

// NewPlayer 创建一个玩家的方法
func NewPlayer(conn ziface.IConnection) *Player {
	// 生成一个玩家 ID
	IdLock.Lock()
	id := PidGen
	PidGen++
	IdLock.Unlock()
	// 创建一个玩家对象
	p := &Player{
		Pid:  id,
		Conn: conn,
		X:    float32(160 + rand.Intn(10)),
		Y:    0,
		Z:    float32(140 + rand.Intn(20)),
		V:    0, // 角度为0
	}
	return p
}

// SendMsg 提供一个发送给客户端消息方法 主要是将 PB 数据序列化调用sendMsg
func (p *Player) SendMsg(msgId uint32, data proto.Message) {
	// 将 proto 序列化 转换为二进制
	msg, err := proto.Marshal(data)
	if err != nil {
		fmt.Println("marshal msg err:", err)
		return
	}
	// 将二进制文件通过 zinx sendMsg 方法发送给客户端
	if p.Conn == nil {
		fmt.Println("Conn is nil")
		return
	}
	err = p.Conn.SendMsg(msgId, msg)
	if err != nil {
		fmt.Println(" player send msg err:", err)
		return
	}
	return
}

// SyncPid 告知客户端玩家 Pid 同步已经生成的玩家 ID 给客户端
func (p *Player) SyncPid() {
	// 组建 MsgID：0 的 proto 数据
	protoMsg := &pb.SyncPid{
		Pid: p.Pid,
	}
	// 将消息发送给客户端
	p.SendMsg(1, protoMsg)
}

// BroadCastStartPosition 广播玩家自己的出生地点
func (p *Player) BroadCastStartPosition() {
	protoMsg := &pb.BroadCast{
		Pid: p.Pid,
		Tp:  2,
		Data: &pb.BroadCast_P{
			// Position
			P: &pb.Position{
				X: p.X,
				Y: p.Y,
				Z: p.Z,
				V: p.V,
			},
		},
	}

	p.SendMsg(200, protoMsg)
}

// Talk 玩家广播世界聊天消息
func (p *Player) Talk(content string) {
	// 1.组建 MsgID：200 的 proto 数据
	protoMsg := &pb.BroadCast{
		Pid: p.Pid,
		Tp:  1,
		Data: &pb.BroadCast_Content{
			Content: content,
		},
	}
	// 2.得到当前所有在线玩家
	players := WorldMgrObj.GetAllPlayers()

	// 3.向所有的玩家（包括自己）发送消息
	for _, player := range players {
		player.SendMsg(200, protoMsg)
	}
}

// SyncSurrounding 同步玩家上线的位置消息
func (p *Player) SyncSurrounding() {
	// 1.获取当前玩家周围的玩家
	pids := WorldMgrObj.AoiMgr.GetPidsByPos(p.X, p.Z)
	players := make([]*Player, 0, len(pids))
	for _, pid := range pids {
		players = append(players, WorldMgrObj.GetPlayerByPid(int32(pid)))
	}

	// 2.将当前玩家的位置信息通过MsgID：200发给周围的玩家
	// 2.1 组建广播消息
	protoMsg := &pb.BroadCast{
		Pid: p.Pid,
		Tp:  2,
		Data: &pb.BroadCast_P{
			P: &pb.Position{
				X: p.X,
				Y: p.Y,
				Z: p.Z,
				V: p.V,
			},
		},
	}

	// 2.2 分别给周围全部玩家发送
	for _, player := range players {
		player.SendMsg(200, protoMsg)
	}

	// 3. 将周围的全部玩家的位置信息发给当前的玩家MsgID：202
}
