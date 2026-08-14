package knet

import "kinz/kiface"

// Compile-time interface conformance assertions: the whole contract layer is
// verified at build time, so a signature drift fails CI immediately.
var (
	_ kiface.IServer           = (*Server)(nil)
	_ kiface.IConnection       = (*Connection)(nil)
	_ kiface.IMsgHandle        = (*MsgHandler)(nil)
	_ kiface.IConnManager      = (*ConnManager)(nil)
	_ kiface.IHeartbeatChecker = (*HeartBeatChecker)(nil)
	_ kiface.IClient           = (*Client)(nil)
	_ kiface.IDataPack         = (*DataPack)(nil)
	_ kiface.IMessage          = (*Message)(nil)
)
