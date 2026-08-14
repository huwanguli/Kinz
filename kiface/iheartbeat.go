package kiface

import "time"

// HeartBeatDefaultMsgID is the default heartbeat message id.
const HeartBeatDefaultMsgID uint32 = 99999

// ServerFullMsgID is the message id sent to a connection rejected because the
// server reached its max-connection limit.
const ServerFullMsgID uint32 = 0xFFFFFFFE

// HeartBeatMsgFunc builds the payload of a heartbeat message.
type HeartBeatMsgFunc func(conn IConnection) []byte

// HeartBeatFunc sends a heartbeat message; a non-nil error marks the peer dead.
type HeartBeatFunc func(conn IConnection) error

// OnRemoteNotAlive handles a peer that did not stay alive.
type OnRemoteNotAlive func(conn IConnection)

// HeartBeatOption customizes heartbeat behavior.
type HeartBeatOption struct {
	MakeMsg          HeartBeatMsgFunc
	OnRemoteNotAlive OnRemoteNotAlive
	HeartBeatMsgID   uint32
	IRouterSlices    []RouterHandler
	// Timeout is the liveness threshold; when no message is received within
	// Timeout, OnRemoteNotAlive fires. Zero uses 3 * interval.
	Timeout time.Duration
}

// IHeartbeatChecker tracks liveness of one connection and sends heartbeats.
type IHeartbeatChecker interface {
	SetOnRemoteNotAlive(OnRemoteNotAlive)
	SetHeartBeatMsgFunc(HeartBeatMsgFunc)
	SetHeartbeatFunc(HeartBeatFunc)
	BindRouterSlices(msgID uint32, handlers ...RouterHandler)
	Start()
	Stop()
	SendHeartBeatMsg() error
	BindConn(conn IConnection)
	Clone() IHeartbeatChecker
	MsgID() uint32
	RouterSlices() []RouterHandler
}
