package knet

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

// reconnectPolicy configures automatic reconnection: exponential backoff with
// jitter. A zero value yields the defaults (500ms initial, 5s max, x2).
type reconnectPolicy struct {
	initial    time.Duration
	max        time.Duration
	multiplier float64
}

func defaultReconnect() reconnectPolicy {
	return reconnectPolicy{
		initial:    500 * time.Millisecond,
		max:        5 * time.Second,
		multiplier: 2,
	}
}

// backoffValue returns the deterministic backoff for the given attempt number.
func (p reconnectPolicy) backoffValue(attempt int) time.Duration {
	d := float64(p.initial)
	for i := 0; i < attempt && d < float64(p.max); i++ {
		d *= p.multiplier
	}
	if d > float64(p.max) {
		d = float64(p.max)
	}
	return time.Duration(d)
}

// delay returns the jittered backoff (50%..100% of the deterministic value).
func (p reconnectPolicy) delay(attempt int) time.Duration {
	d := float64(p.backoffValue(attempt))
	return time.Duration(d * (0.5 + rand.Float64()*0.5))
}

// Client implements kiface.IClient: dials, manages one connection with
// automatic reconnection (exponential backoff + jitter), and reuses the
// Server's connection lifecycle via the connHost abstraction.
type Client struct {
	cfg        *kconf.Config
	name       string
	codec      kiface.ICodec
	tlsConfig  *tls.Config
	msgHandler kiface.IMsgHandle
	connMgr    kiface.IConnManager
	hbTemplate kiface.IHeartbeatChecker

	reconnect reconnectPolicy
	connID    atomic.Uint64

	onConnStart func(kiface.IConnection)
	onConnStop  func(kiface.IConnection)

	mu       sync.Mutex
	started  bool
	stopping bool
	cancel   context.CancelFunc
	conn     kiface.IConnection
	connGone chan struct{}
}

// NewClient creates a TCP client for the given address and applies opts.
func NewClient(ip string, port int, opts ...ClientOption) kiface.IClient {
	cfg := kconf.Default()
	cfg.Host = ip
	cfg.Port = port
	c := &Client{
		cfg:       cfg,
		name:      "KinzClient",
		reconnect: defaultReconnect(),
		connMgr:   NewConnManager(1),
		connGone:  make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.msgHandler == nil {
		c.msgHandler = NewMsgHandler(c.cfg.WorkerPoolSize, c.cfg.MaxWorkerTaskLen)
	}
	if c.codec == nil {
		c.codec = NewTLVPackWithOrder(binary.LittleEndian, c.cfg.MaxPacketSize)
	}
	return c
}

// WithReconnect configures the reconnection backoff.
func WithReconnect(initial, max time.Duration, multiplier float64) ClientOption {
	return func(c kiface.IClient) {
		if cl, ok := c.(*Client); ok {
			if initial > 0 {
				cl.reconnect.initial = initial
			}
			if max > 0 {
				cl.reconnect.max = max
			}
			if multiplier > 1 {
				cl.reconnect.multiplier = multiplier
			}
		}
	}
}

// WithTLSClient enables TLS for the client connection.
func WithTLSClient(cfg *tls.Config) ClientOption {
	return func(c kiface.IClient) {
		if cl, ok := c.(*Client); ok {
			cl.tlsConfig = cfg
		}
	}
}

// Start implements kiface.IClient: dials once; on success, manages the
// connection with automatic reconnection until Stop. Returns the dial error.
func (c *Client) Start() error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return errors.New("kinz: client already started")
	}
	c.started = true
	c.stopping = false
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.mu.Unlock()

	if _, err := c.dial(ctx); err != nil {
		c.mu.Lock()
		c.started = false
		c.cancel = nil
		c.mu.Unlock()
		return err
	}
	go c.run(ctx)
	return nil
}

// Stop implements kiface.IClient: stops reconnecting and closes the connection.
func (c *Client) Stop() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.started = false
	c.stopping = true
	cancel := c.cancel
	c.cancel = nil
	conn := c.conn
	c.mu.Unlock()

	cancel() // stops the run loop (and any in-flight dial)
	if conn != nil {
		conn.Stop()
	}
	c.connMgr.ClearConn()
}

// Restart implements kiface.IClient: Stop then Start (best-effort on the
// initial dial error).
func (c *Client) Restart() {
	c.Stop()
	_ = c.Start()
}

// dial establishes one connection and starts it.
func (c *Client) dial(ctx context.Context) (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	d := net.Dialer{}
	var raw net.Conn
	var err error
	if c.tlsConfig != nil {
		raw, err = tls.DialWithDialer(&d, "tcp", addr, c.tlsConfig)
	} else {
		raw, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, err
	}

	conn := NewConnection(c, raw, c.connID.Add(1), c.codec.Clone(), c.msgHandler, c.cfg)

	c.mu.Lock()
	if c.stopping {
		c.mu.Unlock()
		_ = raw.Close()
		return nil, errors.New("kinz: client stopping")
	}
	c.conn = conn
	c.mu.Unlock()

	if err := c.connMgr.Add(conn); err != nil {
		_ = raw.Close()
		return nil, err
	}
	conn.Start()
	return conn, nil
}

// run manages the connection until ctx is done, reconnecting with backoff.
func (c *Client) run(ctx context.Context) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.connGone:
			if c.isStopping() {
				return
			}
			delay := c.reconnect.delay(attempt)
			attempt++
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if _, err := c.dial(ctx); err != nil {
				continue // dial failed; retry with a longer backoff
			}
			attempt = 0
		}
	}
}

func (c *Client) isStopping() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopping
}

// Conn implements kiface.IClient.
func (c *Client) Conn() kiface.IConnection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

// AddRouterSlices implements kiface.IClient.
func (c *Client) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return c.msgHandler.AddRouterSlices(msgID, handlers...)
}

// SetOnConnStart implements kiface.IClient.
func (c *Client) SetOnConnStart(f func(kiface.IConnection)) { c.onConnStart = f }

// SetOnConnStop implements kiface.IClient.
func (c *Client) SetOnConnStop(f func(kiface.IConnection)) { c.onConnStop = f }

// GetOnConnStart implements kiface.IClient.
func (c *Client) GetOnConnStart() func(kiface.IConnection) { return c.onConnStart }

// GetOnConnStop implements kiface.IClient.
func (c *Client) GetOnConnStop() func(kiface.IConnection) { return c.onConnStop }

// SetCodec implements kiface.IClient.
func (c *Client) SetCodec(codec kiface.ICodec) { c.codec = codec }

// GetCodec implements kiface.IClient.
func (c *Client) GetCodec() kiface.ICodec { return c.codec }

// GetMsgHandler implements kiface.IClient.
func (c *Client) GetMsgHandler() kiface.IMsgHandle { return c.msgHandler }

// StartHeartBeat implements kiface.IClient: enables heartbeat sending.
func (c *Client) StartHeartBeat(interval time.Duration) {
	c.StartHeartBeatWithOption(interval, nil)
}

// StartHeartBeatWithOption implements kiface.IClient.
func (c *Client) StartHeartBeatWithOption(interval time.Duration, option *kiface.HeartBeatOption) {
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
		if len(option.IRouterSlices) > 0 {
			tpl.BindRouterSlices(option.HeartBeatMsgID, option.IRouterSlices...)
		}
	}
	c.hbTemplate = tpl
}

// SetName implements kiface.IClient.
func (c *Client) SetName(name string) { c.name = name }

// GetName implements kiface.IClient.
func (c *Client) GetName() string { return c.name }

// connHost implementation ---------------------------------------------------

// GetHeartBeat implements connHost.
func (c *Client) GetHeartBeat() kiface.IHeartbeatChecker { return c.hbTemplate }

// CallOnConnStart implements connHost.
func (c *Client) CallOnConnStart(conn kiface.IConnection) {
	if c.onConnStart != nil {
		c.onConnStart(conn)
	}
}

// CallOnConnStop implements connHost: fires user hook and signals reconnection.
func (c *Client) CallOnConnStop(conn kiface.IConnection) {
	if c.onConnStop != nil {
		c.onConnStop(conn)
	}
	select {
	case c.connGone <- struct{}{}:
	default:
	}
}

// GetConnMgr implements connHost.
func (c *Client) GetConnMgr() kiface.IConnManager { return c.connMgr }
