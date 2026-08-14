package knet

import (
	"sync"
	"time"

	"kinz/kiface"
	"kinz/klog"
)

// HeartBeatChecker implements kiface.IHeartbeatChecker: periodically checks the
// bound connection's liveness and sends heartbeat messages. Liveness is based
// on any received message (the read path refreshes an atomic timestamp).
type HeartBeatChecker struct {
	interval time.Duration
	timeout  time.Duration

	mu      sync.Mutex
	started bool
	quit    chan struct{}

	makeMsg          kiface.HeartBeatMsgFunc
	onRemoteNotAlive kiface.OnRemoteNotAlive
	beatFunc         kiface.HeartBeatFunc

	msgID        uint32
	routerSlices []kiface.RouterHandler

	connMu sync.RWMutex
	conn   kiface.IConnection
}

// NewHeartbeatChecker creates a checker with default behavior: heartbeat
// message with HeartBeatDefaultMsgID, default payload, graceful conn.Stop on
// timeout. timeout defaults to 3 * interval.
func NewHeartbeatChecker(interval time.Duration) *HeartBeatChecker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &HeartBeatChecker{
		interval:         interval,
		timeout:          3 * interval,
		makeMsg:          makeDefaultMsg,
		onRemoteNotAlive: notAliveDefaultFunc,
		msgID:            kiface.HeartBeatDefaultMsgID,
	}
}

// SetTimeout overrides the liveness threshold.
func (h *HeartBeatChecker) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		h.timeout = timeout
	}
}

// SetOnRemoteNotAlive implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) SetOnRemoteNotAlive(f kiface.OnRemoteNotAlive) {
	if f != nil {
		h.onRemoteNotAlive = f
	}
}

// SetHeartBeatMsgFunc implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) SetHeartBeatMsgFunc(f kiface.HeartBeatMsgFunc) {
	if f != nil {
		h.makeMsg = f
	}
}

// SetHeartbeatFunc implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) SetHeartbeatFunc(f kiface.HeartBeatFunc) {
	if f != nil {
		h.beatFunc = f
	}
}

// BindRouterSlices implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) BindRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) {
	if len(handlers) > 0 {
		h.msgID = msgID
		h.routerSlices = append(h.routerSlices, handlers...)
	}
}

// BindConn implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) BindConn(conn kiface.IConnection) {
	h.connMu.Lock()
	h.conn = conn
	h.connMu.Unlock()
	conn.SetHeartBeat(h)
}

// Start implements kiface.IHeartbeatChecker (idempotent).
func (h *HeartBeatChecker) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.started = true
	h.quit = make(chan struct{})
	go h.loop()
}

// Stop implements kiface.IHeartbeatChecker (idempotent).
func (h *HeartBeatChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.started {
		return
	}
	h.started = false
	close(h.quit)
}

func (h *HeartBeatChecker) loop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.check()
		case <-h.quit:
			return
		}
	}
}

// check evaluates liveness: dead -> OnRemoteNotAlive; alive -> send heartbeat
// (custom beatFunc when set, default message otherwise).
func (h *HeartBeatChecker) check() {
	h.connMu.RLock()
	conn := h.conn
	h.connMu.RUnlock()
	if conn == nil {
		return
	}
	if !conn.IsAlive(h.timeout) {
		h.onRemoteNotAlive(conn)
		return
	}
	if h.beatFunc != nil {
		if err := h.beatFunc(conn); err != nil {
			klog.L().Warn("heartbeat beatFunc failed", "err", err)
		}
		return
	}
	if err := h.SendHeartBeatMsg(); err != nil {
		klog.L().Warn("send heartbeat failed", "err", err)
	}
}

// SendHeartBeatMsg implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) SendHeartBeatMsg() error {
	h.connMu.RLock()
	conn := h.conn
	h.connMu.RUnlock()
	if conn == nil {
		return nil
	}
	return conn.SendMsg(h.msgID, h.makeMsg(conn))
}

// Clone implements kiface.IHeartbeatChecker: a fresh, unbound checker with the
// same configuration (for per-connection binding).
func (h *HeartBeatChecker) Clone() kiface.IHeartbeatChecker {
	c := &HeartBeatChecker{
		interval:         h.interval,
		timeout:          h.timeout,
		makeMsg:          h.makeMsg,
		onRemoteNotAlive: h.onRemoteNotAlive,
		beatFunc:         h.beatFunc,
		msgID:            h.msgID,
	}
	c.routerSlices = append(c.routerSlices, h.routerSlices...)
	return c
}

// MsgID implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) MsgID() uint32 { return h.msgID }

// RouterSlices implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) RouterSlices() []kiface.RouterHandler { return h.routerSlices }

// makeDefaultMsg builds the default heartbeat payload.
func makeDefaultMsg(conn kiface.IConnection) []byte {
	return []byte("heartbeat")
}

// notAliveDefaultFunc gracefully closes the dead connection.
func notAliveDefaultFunc(conn kiface.IConnection) {
	klog.L().Info("remote not alive, closing", "remote", conn.GetRemoteAddr().String())
	conn.Stop()
}

// HeartBeatDefaultHandle logs a received heartbeat. Liveness is tracked by the
// read path itself (any message refreshes the connection timestamp).
func HeartBeatDefaultHandle(req kiface.IRequest) {
	klog.L().Debug("heartbeat received", "remote", req.GetConnection().GetRemoteAddr().String())
}
