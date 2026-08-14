package knet

import "kinz/kiface"

// HeartBeatChecker implements kiface.IHeartbeatChecker.
// Behavioral implementation lands in Phase 2.
type HeartBeatChecker struct{}

// NewHeartbeatChecker creates a HeartBeatChecker.
func NewHeartbeatChecker() *HeartBeatChecker { return &HeartBeatChecker{} }

// SetOnRemoteNotAlive implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SetOnRemoteNotAlive(f kiface.OnRemoteNotAlive) {}

// SetHeartBeatMsgFunc implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SetHeartBeatMsgFunc(f kiface.HeartBeatMsgFunc) {}

// SetHeartbeatFunc implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SetHeartbeatFunc(f kiface.HeartBeatFunc) {}

// BindRouter implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) BindRouter(msgID uint32, router kiface.IRouter) {}

// BindRouterSlices implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) BindRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) {}

// Start implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) Start() {}

// Stop implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) Stop() {}

// SendHeartBeatMsg implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SendHeartBeatMsg() error { return kiface.ErrNotImplemented }

// BindConn implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) BindConn(conn kiface.IConnection) {}

// Clone implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) Clone() kiface.IHeartbeatChecker { return NewHeartbeatChecker() }

// MsgID implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) MsgID() uint32 { return kiface.HeartBeatDefaultMsgID }

// Router implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) Router() kiface.IRouter { return nil }

// RouterSlices implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) RouterSlices() []kiface.RouterHandler { return nil }
