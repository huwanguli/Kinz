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
	// AddRouter registers a classic router for msgID.
	AddRouter(msgID uint32, router IRouter) error

	// SetOnConnStart / SetOnConnStop register connection lifecycle hooks.
	SetOnConnStart(func(IConnection))
	SetOnConnStop(func(IConnection))
	GetOnConnStart() func(IConnection)
	GetOnConnStop() func(IConnection)

	// SetPacket / GetPacket configure the wire-format pack implementation.
	SetPacket(IDataPack)
	GetPacket() IDataPack
	// SetDecoder configures the frame decoder.
	SetDecoder(IDecoder)
	// AddInterceptor appends a middleware interceptor.
	AddInterceptor(IInterceptor)
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
