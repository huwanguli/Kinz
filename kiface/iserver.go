package kiface

import (
	"context"
	"time"
)

// IServer is the top-level server contract. Convention-first: a Server created
// with NewServer() runs with production-safe defaults (heartbeat, max-conn
// rejection, panic recovery, graceful shutdown); extension happens at the seams
// (codec, decoder, interceptors, routers, logger, metrics).
type IServer interface {
	// Run starts accepting connections and blocks until ctx is cancelled or a
	// fatal error occurs. It returns the fatal error, or nil after shutdown.
	Run(ctx context.Context) error
	// Shutdown gracefully stops the server: stop accepting, drain connections,
	// stop the worker pool, and release pooled resources, bounded by ctx.
	Shutdown(ctx context.Context) error
	// Serve runs Run and, once ctx is cancelled, performs a graceful Shutdown.
	Serve(ctx context.Context) error

	// Name returns the server name.
	Name() string

	// AddRouter registers a classic three-stage IRouter for msgID.
	// Returns ErrMsgIDRegistered when msgID is already registered.
	AddRouter(msgID uint32, router IRouter) error
	// AddRouterSlices registers function-style handlers for msgID.
	AddRouterSlices(msgID uint32, handlers ...RouterHandler) (IRouterSlices, error)
	// Group scopes handlers to every msgID in the inclusive range [start, end].
	Group(start, end uint32, handlers ...RouterHandler) (IGroupRouterSlices, error)
	// Use registers global middleware handlers applied to every message.
	Use(handlers ...RouterHandler) (IRouterSlices, error)

	// GetConnMgr returns the connection manager.
	GetConnMgr() IConnManager

	// SetOnConnStart / SetOnConnStop register connection lifecycle hooks.
	SetOnConnStart(func(IConnection))
	SetOnConnStop(func(IConnection))
	GetOnConnStart() func(IConnection)
	GetOnConnStop() func(IConnection)
	// CallOnConnStart / CallOnConnStop invoke the hooks (framework-internal use).
	CallOnConnStart(conn IConnection)
	CallOnConnStop(conn IConnection)

	// SetPacket / GetPacket configure the wire-format pack implementation.
	SetPacket(pack IDataPack)
	GetPacket() IDataPack
	// SetDecoder / GetDecoder configure the frame decoder (TCP sticky/half packets).
	SetDecoder(decoder IDecoder)
	GetDecoder() IDecoder
	// AddInterceptor appends a middleware interceptor to the request pipeline.
	AddInterceptor(interceptor IInterceptor)
	// GetMsgHandler returns the message dispatch module.
	GetMsgHandler() IMsgHandle

	// StartHeartBeat enables heartbeat checking with default options.
	StartHeartBeat(interval time.Duration)
	// SetHeartBeatWithOption enables heartbeat with custom options.
	SetHeartBeatWithOption(interval time.Duration, option *HeartBeatOption)
	// GetHeartBeat returns the heartbeat checker template.
	GetHeartBeat() IHeartbeatChecker
}
