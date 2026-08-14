package knet

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/kpool"
)

// readChunkSize is the pooled read-buffer size per connection.
const readChunkSize = 4096

// connection states.
const (
	stateCreated uint32 = iota
	stateRunning
	stateClosed
)

// Connection implements kiface.IConnection: a reader and a writer goroutine, a
// buffered write queue with timeout, liveness tracking, and key-value
// properties. Stop is idempotent and safe from any goroutine.
type Connection struct {
	server  connHost
	conn    net.Conn
	connID  uint64
	codec   kiface.ICodec
	msgHdl  kiface.IMsgHandle
	cfg     *kconf.Config
	metrics *connMetrics

	msgChan      chan []byte
	done         chan struct{}
	closeOnce    sync.Once
	state        atomic.Uint32
	wg           sync.WaitGroup
	lastActivity atomic.Int64

	// started records that Start has launched the reader/writer goroutines.
	// wg.Add happens in NewConnection (before the object is published to any
	// goroutine), so Stop's wg.Wait never races with an Add; started only
	// decides whether there is anything to wait for.
	started atomic.Bool

	hb   kiface.IHeartbeatChecker
	hbMu sync.Mutex // guards hb (Start writes, stopOnce reads)

	property     map[string]interface{}
	propertyLock sync.RWMutex
}

// NewConnection wraps a net.Conn with the host's codec (a per-connection
// clone) and the message handler. The caller registers it in the host's
// ConnManager before calling Start.
func NewConnection(host connHost, conn net.Conn, connID uint64,
	codec kiface.ICodec, msgHdl kiface.IMsgHandle, cfg *kconf.Config,
	metrics *connMetrics) *Connection {
	c := &Connection{
		server:   host,
		conn:     conn,
		connID:   connID,
		codec:    codec,
		msgHdl:   msgHdl,
		cfg:      cfg,
		metrics:  metrics,
		msgChan:  make(chan []byte, cfg.WriteQueueSize),
		done:     make(chan struct{}),
		property: make(map[string]interface{}),
	}
	c.state.Store(stateCreated)
	// Count both goroutines up front: NewConnection happens-before any other
	// goroutine can reach Stop, so wg.Add can never race with wg.Wait.
	c.wg.Add(2)
	c.touch()
	return c
}

// Start implements kiface.IConnection: binds heartbeat, starts reader/writer.
func (c *Connection) Start() {
	if !c.state.CompareAndSwap(stateCreated, stateRunning) {
		return
	}
	if tpl := c.server.GetHeartBeat(); tpl != nil {
		hb := tpl.Clone()
		hb.BindConn(c)
		c.hbMu.Lock()
		// A concurrent Stop may have closed the connection while we cloned
		// the template; in that case skip the heartbeat entirely so its
		// goroutine cannot leak (stopOnce has already run and won't stop it).
		if c.state.Load() == stateRunning {
			c.hb = hb
			hb.Start()
		}
		c.hbMu.Unlock()
	}
	c.started.Store(true)
	go c.startReader()
	go c.startWriter()
	c.server.CallOnConnStart(c)
}

func (c *Connection) startReader() {
	defer c.wg.Done()
	defer c.stopOnce()

	buf := kpool.Get(readChunkSize)
	defer kpool.Put(buf)

	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			return
		}
		c.touch()
		if c.metrics != nil {
			c.metrics.bytesIn.Add(uint64(n))
		}
		msgs, derr := c.codec.Decode(buf[:n])
		if derr != nil {
			klog.L().Warn("decode error, closing connection", "connID", c.connID, "err", derr)
			return
		}
		for _, msg := range msgs {
			if c.metrics != nil {
				c.metrics.msgsRecv.Inc()
			}
			c.msgHdl.SendMsgToTaskQueue(NewRequest(c, msg))
		}
	}
}

func (c *Connection) startWriter() {
	defer c.wg.Done()
	for {
		select {
		case data := <-c.msgChan:
			_ = c.conn.SetWriteDeadline(time.Now().Add(time.Duration(c.cfg.WriteTimeout)))
			if _, err := c.conn.Write(data); err != nil {
				klog.L().Warn("write error, closing connection", "connID", c.connID, "err", err)
				c.stopOnce()
				return
			}
			if c.metrics != nil {
				c.metrics.bytesOut.Add(uint64(len(data)))
			}
		case <-c.done:
			return
		}
	}
}

// stopOnce performs the one-time cleanup without waiting for goroutines:
// hooks, heartbeat stop, socket close, done signal, ConnManager removal.
func (c *Connection) stopOnce() {
	c.closeOnce.Do(func() {
		c.state.Store(stateClosed)
		c.server.CallOnConnStop(c)
		c.hbMu.Lock()
		hb := c.hb
		c.hb = nil
		c.hbMu.Unlock()
		if hb != nil {
			hb.Stop()
		}
		if c.metrics != nil {
			c.metrics.active.Dec()
			c.metrics.closed.Inc()
		}
		_ = c.conn.Close()
		close(c.done)
		c.server.GetConnMgr().Remove(c)
	})
}

// Stop implements kiface.IConnection: idempotent graceful close that waits for
// the reader/writer goroutines to exit. When Stop wins the race against a
// Start that never launched the goroutines (or a Start that was cancelled by
// this Stop), there is nothing to wait for.
func (c *Connection) Stop() {
	c.stopOnce()
	if c.started.Load() {
		c.wg.Wait()
	}
}

// GetConn implements kiface.IConnection.
func (c *Connection) GetConn() net.Conn { return c.conn }

// GetConnID implements kiface.IConnection.
func (c *Connection) GetConnID() uint64 { return c.connID }

// GetRemoteAddr implements kiface.IConnection.
func (c *Connection) GetRemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// LocalAddr implements kiface.IConnection.
func (c *Connection) LocalAddr() net.Addr { return c.conn.LocalAddr() }

// SendMsg implements kiface.IConnection: packs and queues the message with a
// write timeout; returns ErrConnClosed when the connection is not running.
func (c *Connection) SendMsg(msgID uint32, data []byte) error {
	if c.state.Load() != stateRunning {
		return kiface.ErrConnClosed
	}
	wire, err := c.codec.Pack(NewMessage(msgID, data))
	if err != nil {
		return err
	}
	timeout := time.Duration(c.cfg.WriteTimeout)
	if timeout > 0 {
		select {
		case c.msgChan <- wire:
			if c.metrics != nil {
				c.metrics.msgsSent.Inc()
			}
			return nil
		case <-c.done:
			return kiface.ErrConnClosed
		case <-time.After(timeout):
			return fmt.Errorf("%w: write queue full", kiface.ErrTimeout)
		}
	}
	select {
	case c.msgChan <- wire:
		if c.metrics != nil {
			c.metrics.msgsSent.Inc()
		}
		return nil
	case <-c.done:
		return kiface.ErrConnClosed
	}
}

// IsAlive implements kiface.IConnection: reports whether any message was
// received within timeout.
func (c *Connection) IsAlive(timeout time.Duration) bool {
	if timeout <= 0 {
		return true
	}
	last := c.lastActivity.Load()
	if last == 0 {
		return true
	}
	return time.Since(time.Unix(0, last)) <= timeout
}

// touch refreshes the liveness timestamp (called on any received data).
func (c *Connection) touch() {
	c.lastActivity.Store(time.Now().UnixNano())
}

// SetHeartBeat implements kiface.IConnection.
func (c *Connection) SetHeartBeat(hb kiface.IHeartbeatChecker) { c.hb = hb }

// SetProperty implements kiface.IConnection.
func (c *Connection) SetProperty(key string, value interface{}) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	c.property[key] = value
}

// GetProperty implements kiface.IConnection.
func (c *Connection) GetProperty(key string) (interface{}, error) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	if v, ok := c.property[key]; ok {
		return v, nil
	}
	return nil, errors.New("kinz: no property found")
}

// RemoveProperty implements kiface.IConnection.
func (c *Connection) RemoveProperty(key string) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	delete(c.property, key)
}
