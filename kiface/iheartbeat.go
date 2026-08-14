package kiface

type IHeartbeatChecker interface {
	SetOnRemoteNotAlive(OnRemoteNotAlive)
	SetHeartBeatMsgFunc(HeartBeatMsgFunc)
	SetHeartbeatFunc(HeartBeatFunc)
	BindRouter(uint32, IRouter)
	BindRouterSlices(uint32, ...RouterHandler)
	Start()
	Stop()
	SendHeartBeatMsg() error
	BindConn(IConnection)
	Clone() IHeartbeatChecker
	MsgID() uint32
	Router() IRouter
	RouterSlices() []RouterHandler
}

// HeartBeatMsgFunc 自定义的心跳检测消息处理方法
type HeartBeatMsgFunc func(IConnection) []byte

// HeartBeatFunc 用户自定义心跳函数
type HeartBeatFunc func(IConnection) error

// OnRemoteNotAlive 用户自定义远程链接不存活时的处理方法
type OnRemoteNotAlive func(IConnection)

type HeartBeatOption struct {
	MakeMsg          HeartBeatMsgFunc
	OnRemoteNotAlive OnRemoteNotAlive
	HeartBeatMsgID   uint32
	Router           IRouter
	IRouterSlices    []RouterHandler
}

// HeartBeatDefaultMsgID 心跳检测消息默认ID
const (
	HeartBeatDefaultMsgID uint32 = 99999
)
