package kiface

import "time"

// IClient is the TCP client contract (full implementation lands in Phase 3).
type IClient interface {
	// Start connects to the server and blocks until Stop is called.
	// Returns the connect error (with retries applied by the implementation).
	Start() error
	// Stop disconnects and disables auto-reconnect.
	Stop()
	// Restart stops the current session and starts a new one.
	Restart()

	// Conn returns the current connection (nil when disconnected).
	Conn() IConnection
	// AddRouterSlices registers function-style handlers for msgID.
	AddRouterSlices(msgID uint32, handlers ...RouterHandler) (IRouterSlices, error)

	// SetOnConnStart / SetOnConnStop register connection lifecycle hooks.
	SetOnConnStart(func(IConnection))
	SetOnConnStop(func(IConnection))
	GetOnConnStart() func(IConnection)
	GetOnConnStop() func(IConnection)

	// SetCodec / GetCodec configure the wire codec (framing + serialization).
	SetCodec(ICodec)
	GetCodec() ICodec
	// GetMsgHandler returns the message dispatch module.
	GetMsgHandler() IMsgHandle

	// StartHeartBeat enables heartbeat sending with default options.
	StartHeartBeat(interval time.Duration)
	// StartHeartBeatWithOption enables heartbeat with custom options.
	StartHeartBeatWithOption(interval time.Duration, option *HeartBeatOption)

	// SetName / GetName manage the client name.
	SetName(string)
	GetName() string
}
