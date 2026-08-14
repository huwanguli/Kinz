package knet

import (
	"context"
	"time"

	"kinz/kiface"
)

// Option customizes a Server at construction time.
type Option func(*Server)

// Server implements kiface.IServer. Behavioral implementation lands in Phase 2.
type Server struct {
	name       string
	connMgr    kiface.IConnManager
	msgHandler kiface.IMsgHandle
	packet     kiface.IDataPack
	decoder    kiface.IDecoder

	onConnStart func(kiface.IConnection)
	onConnStop  func(kiface.IConnection)
	heartbeat   kiface.IHeartbeatChecker
}

// NewServer creates a Server with default configuration and applies opts.
func NewServer(opts ...Option) kiface.IServer {
	s := &Server{
		name:       "KinzServer",
		connMgr:    NewConnManager(0),
		msgHandler: NewMsgHandler(),
		packet:     NewDataPack(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run implements kiface.IServer. Phase 2.
func (s *Server) Run(ctx context.Context) error { return kiface.ErrNotImplemented }

// Shutdown implements kiface.IServer. Phase 2.
func (s *Server) Shutdown(ctx context.Context) error { return nil }

// Serve implements kiface.IServer. Phase 2.
func (s *Server) Serve(ctx context.Context) error { return kiface.ErrNotImplemented }

// Name implements kiface.IServer.
func (s *Server) Name() string { return s.name }

// AddRouter implements kiface.IServer. Phase 2.
func (s *Server) AddRouter(msgID uint32, router kiface.IRouter) error {
	return kiface.ErrNotImplemented
}

// AddRouterSlices implements kiface.IServer. Phase 2.
func (s *Server) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Group implements kiface.IServer. Phase 2.
func (s *Server) Group(start, end uint32, handlers ...kiface.RouterHandler) (kiface.IGroupRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Use implements kiface.IServer. Phase 2.
func (s *Server) Use(handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// GetConnMgr implements kiface.IServer.
func (s *Server) GetConnMgr() kiface.IConnManager { return s.connMgr }

// SetOnConnStart implements kiface.IServer.
func (s *Server) SetOnConnStart(f func(kiface.IConnection)) { s.onConnStart = f }

// SetOnConnStop implements kiface.IServer.
func (s *Server) SetOnConnStop(f func(kiface.IConnection)) { s.onConnStop = f }

// GetOnConnStart implements kiface.IServer.
func (s *Server) GetOnConnStart() func(kiface.IConnection) { return s.onConnStart }

// GetOnConnStop implements kiface.IServer.
func (s *Server) GetOnConnStop() func(kiface.IConnection) { return s.onConnStop }

// CallOnConnStart implements kiface.IServer.
func (s *Server) CallOnConnStart(conn kiface.IConnection) {
	if s.onConnStart != nil {
		s.onConnStart(conn)
	}
}

// CallOnConnStop implements kiface.IServer.
func (s *Server) CallOnConnStop(conn kiface.IConnection) {
	if s.onConnStop != nil {
		s.onConnStop(conn)
	}
}

// SetPacket implements kiface.IServer.
func (s *Server) SetPacket(pack kiface.IDataPack) { s.packet = pack }

// GetPacket implements kiface.IServer.
func (s *Server) GetPacket() kiface.IDataPack { return s.packet }

// SetDecoder implements kiface.IServer.
func (s *Server) SetDecoder(decoder kiface.IDecoder) { s.decoder = decoder }

// GetDecoder implements kiface.IServer.
func (s *Server) GetDecoder() kiface.IDecoder { return s.decoder }

// AddInterceptor implements kiface.IServer.
func (s *Server) AddInterceptor(interceptor kiface.IInterceptor) {
	s.msgHandler.AddInterceptor(interceptor)
}

// GetMsgHandler implements kiface.IServer.
func (s *Server) GetMsgHandler() kiface.IMsgHandle { return s.msgHandler }

// StartHeartBeat implements kiface.IServer. Phase 2.
func (s *Server) StartHeartBeat(interval time.Duration) {}

// SetHeartBeatWithOption implements kiface.IServer. Phase 2.
func (s *Server) SetHeartBeatWithOption(interval time.Duration, option *kiface.HeartBeatOption) {}

// GetHeartBeat implements kiface.IServer.
func (s *Server) GetHeartBeat() kiface.IHeartbeatChecker { return s.heartbeat }
