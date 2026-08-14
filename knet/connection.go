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
	server  kiface.IServer
	conn    *net.TCPConn
	connID  uint64
	packet  kiface.IDataPack
	decoder kiface.IDecoder
	msgHdl  kiface.IMsgHandle
	cfg     *kconf.Config

	msgChan      chan []byte
	done         chan struct{}
	closeOnce    sync.Once
	state        atomic.Uint32
	wg           sync.WaitGroup
	lastActivity atomic.Int64

	hb kiface.IHeartbeatChecker

	property     map[string]interface{}
	propertyLock sync.RWMutex
}

// NewConnection wraps a TCP connection with the server's packet format,
// a per-connection decoder clone, and the message handler. The caller
// registers it in the ConnManager before calling Start.
func NewConnection(server kiface.IServer, conn *net.TCPConn, connID uint64,
	packet kiface.IDataPack, decoder kiface.IDecoder, msgHdl kiface.IMsgHandle,
	cfg *kconf.Config) *Connection {
	c := &Connection{
		server:   server,
		conn:     conn,
		connID:   connID,
		packet:   packet,
		decoder:  decoder,
		msgHdl:   msgHdl,
		cfg:      cfg,
		msgChan:  make(chan []byte, cfg.WriteQueueSize),
		done:     make(chan struct{}),
		property: make(map[string]interface{}),
	}
	c.state.Store(stateCreated)
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
		c.hb = hb
		hb.Start()
	}
	c.wg.Add(2)
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
		frames, derr := c.decoder.Decode(buf[:n])
		if derr != nil {
			klog.L().Warn("decode error, closing connection", "connID", c.connID, "err", derr)
			return
		}
		for _, frame := range frames {
			if err := c.handleFrame(frame); err != nil {
				return
			}
		}
	}
}

// handleFrame parses one complete frame into a message and dispatches it.
func (c *Connection) handleFrame(frame []byte) error {
	msg, err := c.packet.Unpack(frame)
	if err != nil {
		return err
	}
	headLen := c.packet.GetHeadLen()
	if len(frame) < int(headLen) || uint32(len(frame)-int(headLen)) != msg.GetDataLen() {
		return fmt.Errorf("%w: frame length mismatch", kiface.ErrProtocol)
	}
	msg.SetData(frame[headLen:])
	c.msgHdl.SendMsgToTaskQueue(NewRequest(c, msg))
	return nil
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
		if c.hb != nil {
			c.hb.Stop()
		}
		_ = c.conn.Close()
		close(c.done)
		c.server.GetConnMgr().Remove(c)
	})
}

// Stop implements kiface.IConnection: idempotent graceful close that waits for
// the reader/writer goroutines to exit.
func (c *Connection) Stop() {
	c.stopOnce()
	c.wg.Wait()
}

// GetConn implements kiface.IConnection.
func (c *Connection) GetConn() *net.TCPConn { return c.conn }

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
	wire, err := c.packet.Pack(NewMessage(msgID, data))
	if err != nil {
		return err
	}
	timeout := time.Duration(c.cfg.WriteTimeout)
	if timeout > 0 {
		select {
		case c.msgChan <- wire:
			return nil
		case <-c.done:
			return kiface.ErrConnClosed
		case <-time.After(timeout):
			return fmt.Errorf("%w: write queue full", kiface.ErrTimeout)
		}
	}
	select {
	case c.msgChan <- wire:
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
