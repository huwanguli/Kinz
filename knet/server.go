package knet

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/kmetrics"
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

// WithTLS enables TLS for accepted connections with the given config.
func WithTLS(cfg *tls.Config) Option {
	return func(s *Server) { s.tlsConfig = cfg }
}

// Server implements kiface.IServer with convention-first production defaults:
// heartbeat and max-conn rejection are wired by default, panic recovery and
// graceful shutdown are built in.
type Server struct {
	cfg        *kconf.Config
	connMgr    kiface.IConnManager
	msgHandler kiface.IMsgHandle
	codec      kiface.ICodec
	hbTemplate kiface.IHeartbeatChecker
	tlsConfig  *tls.Config

	metrics       *kmetrics.Registry
	connsTotal    *kmetrics.Counter
	connsActive   *kmetrics.Gauge
	connsClosed   *kmetrics.Counter
	connsRejected *kmetrics.Counter

	metricsListener net.Listener

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
	s.codec = NewTLVPackWithOrder(binary.LittleEndian, s.cfg.MaxPacketSize)

	s.metrics = kmetrics.NewRegistry()
	s.connsTotal = s.metrics.Counter(mConnTotal, "Total connections accepted")
	s.connsActive = s.metrics.Gauge(mConnActive, "Currently active connections")
	s.connsClosed = s.metrics.Counter(mConnClosed, "Connections closed")
	s.connsRejected = s.metrics.Counter(mConnRejected, "Connections rejected (max reached)")
	if mh, ok := s.msgHandler.(*MsgHandler); ok {
		mh.SetMetrics(newHandlerMetrics(s.metrics))
	}
	return s
}

// GetMetrics implements kiface.IServer.
func (s *Server) GetMetrics() *kmetrics.Registry { return s.metrics }

// AttachMetrics implements kiface.IServer: serves the metrics endpoint
// (/metrics) on addr via the official promhttp handler.
func (s *Server) AttachMetrics(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", s.metrics.Handler())
	srv := &http.Server{Handler: mux}
	s.mu.Lock()
	if s.metricsListener != nil {
		s.mu.Unlock()
		_ = ln.Close()
		return errors.New("kinz: metrics endpoint already attached")
	}
	s.metricsListener = ln
	s.mu.Unlock()
	go func() { _ = srv.Serve(ln) }()
	return nil
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
	if s.tlsConfig != nil {
		ln = tls.NewListener(ln, s.tlsConfig)
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
	metricsLn := s.metricsListener
	s.metricsListener = nil
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	if metricsLn != nil {
		_ = metricsLn.Close()
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
	c := NewConnection(s, conn, s.connID.Add(1), s.codec.Clone(), s.msgHandler, s.cfg,
		newConnMetrics(s.metrics))
	if err := s.connMgr.Add(c); err != nil {
		return err // ErrServerFull
	}
	s.connsTotal.Inc()
	s.connsActive.Inc()
	go c.Start()
	return nil
}

// rejectConn sends a server-full message with the cause, then closes the
// connection (write deadline protects against a stuck peer).
func (s *Server) rejectConn(conn net.Conn, cause error) error {
	s.connsRejected.Inc()
	klog.L().Warn("connection rejected", "remote", conn.RemoteAddr().String(), "cause", cause)
	_ = conn.SetWriteDeadline(time.Now().Add(time.Duration(s.cfg.WriteTimeout)))
	wire, err := s.codec.Pack(NewMessage(kiface.ServerFullMsgID, []byte(cause.Error())))
	if err == nil {
		_, _ = conn.Write(wire)
	}
	return conn.Close()
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

// SetCodec implements kiface.IServer.
func (s *Server) SetCodec(codec kiface.ICodec) { s.codec = codec }

// GetCodec implements kiface.IServer.
func (s *Server) GetCodec() kiface.ICodec { return s.codec }

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
	tpl.SetMetrics(newHeartbeatMetrics(s.metrics))
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
