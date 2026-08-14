package knet

import "kinz/kiface"

// connHost is the subset of IServer that Connection depends on, so both Server
// and Client can host connections and share the connection lifecycle code.
type connHost interface {
	GetHeartBeat() kiface.IHeartbeatChecker
	CallOnConnStart(kiface.IConnection)
	CallOnConnStop(kiface.IConnection)
	GetConnMgr() kiface.IConnManager
}
