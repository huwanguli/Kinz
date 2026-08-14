package knet

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/kinterceptor"
	"kinz/klog"
)

// Option customizes a Server at construction time.
type Option func(*Server)

// WithConfig replaces the default server configuration.
func WithConfig(cfg *kconf.Config) Option {
	return func(s *Server) { s.cfg = cfg }
}

// WithMaxConn overrides the max connection count.
func WithMaxConn(n int) Option {
	return func(s *Server) { s.cfg.MaxConn = n }
}

// WithName overrides the server name.
func WithName(name string) Option {
	return func(s *Server) { s.cfg.Name = name }
}

// Server implements kiface.IServer with convention-first production defaults:
// heartbeat and max-conn rejection are wired by default, panic recovery and
// graceful shutdown are built in.
type Server struct {
	cfg        *kconf.Config
	connMgr    kiface.IConnManager
	msgHandler kiface.IMsgHandle
	packet     kiface.IDataPack
	decoder    kiface.IDecoder
	hbTemplate kiface.IHeartbeatChecker

	onConnStart func(kiface.IConnection)
	onConnStop  func(kiface.IConnection)

	mu       sync.Mutex
	listener net.Listener
	started  bool
	closed   bool
	connID   atomic.Uint64
}

// NewServer creates a Server with default configuration and applies opts.
func NewServer(opts ...Option) kiface.IServer {
	s := &Server{cfg: kconf.Default()}
	for _, opt := range opts {
		opt(s)
	}
	s.connMgr = NewConnManager(s.cfg.MaxConn)
	s.msgHandler = NewMsgHandler(s.cfg.WorkerPoolSize, s.cfg.MaxWorkerTaskLen)
	s.packet = NewDataPack()
	s.decoder = defaultDecoder(s.cfg)
	return s
}

// defaultDecoder builds the TLV frame decoder: [DataLen:4][MsgID:4][Data].
func defaultDecoder(cfg *kconf.Config) kiface.IDecoder {
	return kinterceptor.NewFrameDecoder(kiface.LengthField{
		Order:               binary.LittleEndian,
		MaxFrameLength:      uint64(cfg.MaxPacketSize) + 8,
		LengthFieldOffset:   0,
		LengthFieldLength:   4,
		LengthAdjustment:    4,
		InitialBytesToStrip: 0,
	})
}

// Run implements kiface.IServer: starts the listener and worker pool, blocks
// until ctx is cancelled, then performs a graceful shutdown and returns nil
// (or the shutdown error).
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("kinz: server already started")
	}
	s.started = true
	s.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	s.msgHandler.StartWorkerPool()

	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		s.acceptLoop(ln)
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := s.Shutdown(shutdownCtx)
	<-acceptDone
	return shutdownErr
}

// Shutdown implements kiface.IServer: stop accepting, drain connections, stop
// the worker pool. Idempotent and bounded by ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	ln := s.listener
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}

	drained := make(chan struct{})
	go func() {
		s.connMgr.ClearConn()
		close(drained)
	}()
	select {
	case <-drained:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.msgHandler.StopWorkerPool(ctx)
	return nil
}

// Serve implements kiface.IServer: canonical entry that runs until ctx is
// cancelled and then shuts down gracefully. Wire signal.NotifyContext in the
// application.
func (s *Server) Serve(ctx context.Context) error {
	return s.Run(ctx)
}

// Name implements kiface.IServer.
func (s *Server) Name() string { return s.cfg.Name }

// Address implements kiface.IServer: the actual listen address, or nil before
// Run binds (Port 0 yields an ephemeral port).
func (s *Server) Address() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return // listener closed by Shutdown
			}
			klog.L().Warn("accept error", "err", err)
			continue
		}
		if err := s.handleConn(conn); err != nil {
			_ = s.rejectConn(conn, err)
		}
	}
}

func (s *Server) handleConn(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("kinz: non-TCP connection")
	}
	c := NewConnection(s, tcpConn, s.connID.Add(1), s.packet, s.decoder.Clone(),
		s.msgHandler, s.cfg)
	if err := s.connMgr.Add(c); err != nil {
		return err // ErrServerFull
	}
	go c.Start()
	return nil
}

// rejectConn sends a server-full message with the cause, then closes the
// connection (write deadline protects against a stuck peer).
func (s *Server) rejectConn(conn net.Conn, cause error) error {
	klog.L().Warn("connection rejected", "remote", conn.RemoteAddr().String(), "cause", cause)
	_ = conn.SetWriteDeadline(time.Now().Add(time.Duration(s.cfg.WriteTimeout)))
	wire, err := s.packet.Pack(NewMessage(kiface.ServerFullMsgID, []byte(cause.Error())))
	if err == nil {
		_, _ = conn.Write(wire)
	}
	return conn.Close()
}

// AddRouter implements kiface.IServer.
func (s *Server) AddRouter(msgID uint32, router kiface.IRouter) error {
	return s.msgHandler.AddRouter(msgID, router)
}

// AddRouterSlices implements kiface.IServer.
func (s *Server) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return s.msgHandler.AddRouterSlices(msgID, handlers...)
}

// Group implements kiface.IServer.
func (s *Server) Group(start, end uint32, handlers ...kiface.RouterHandler) (kiface.IGroupRouterSlices, error) {
	return s.msgHandler.Group(start, end, handlers...)
}

// Use implements kiface.IServer.
func (s *Server) Use(handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return s.msgHandler.Use(handlers...)
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

// StartHeartBeat implements kiface.IServer: enables heartbeat checking with
// default options.
func (s *Server) StartHeartBeat(interval time.Duration) {
	s.SetHeartBeatWithOption(interval, nil)
}

// SetHeartBeatWithOption implements kiface.IServer: stores the heartbeat
// template; each new connection clones and starts it.
func (s *Server) SetHeartBeatWithOption(interval time.Duration, option *kiface.HeartBeatOption) {
	tpl := NewHeartbeatChecker(interval)
	if option != nil {
		if option.MakeMsg != nil {
			tpl.SetHeartBeatMsgFunc(option.MakeMsg)
		}
		if option.OnRemoteNotAlive != nil {
			tpl.SetOnRemoteNotAlive(option.OnRemoteNotAlive)
		}
		if option.Timeout > 0 {
			tpl.SetTimeout(option.Timeout)
		}
		if option.HeartBeatMsgID != 0 {
			tpl.msgID = option.HeartBeatMsgID
		}
		if option.Router != nil {
			tpl.BindRouter(option.HeartBeatMsgID, option.Router)
		}
		if len(option.IRouterSlices) > 0 {
			tpl.BindRouterSlices(option.HeartBeatMsgID, option.IRouterSlices...)
		}
	}
	s.hbTemplate = tpl
	// Route received heartbeat frames somewhere (liveness is tracked by the
	// read path itself; a duplicate registration is tolerated).
	_, _ = s.msgHandler.AddRouterSlices(tpl.MsgID(), HeartBeatDefaultHandle)
}

// GetHeartBeat implements kiface.IServer: the heartbeat template (cloned per
// connection).
func (s *Server) GetHeartBeat() kiface.IHeartbeatChecker { return s.hbTemplate }
