package knet

import (
	"errors"
	"net"
	"sync"
	"time"

	"kinz/kiface"
)

// Connection implements kiface.IConnection. Behavioral implementation lands in Phase 2.
type Connection struct {
	conn   *net.TCPConn
	connID uint64

	property     map[string]interface{}
	propertyLock sync.RWMutex
}

// NewConnection wraps a TCP connection. Lifecycle registration lands in Phase 2.
func NewConnection(conn *net.TCPConn, connID uint64) *Connection {
	return &Connection{
		conn:     conn,
		connID:   connID,
		property: make(map[string]interface{}),
	}
}

// Start implements kiface.IConnection. Phase 2.
func (c *Connection) Start() {}

// Stop implements kiface.IConnection. Phase 2 (graceful, idempotent).
func (c *Connection) Stop() { _ = c.conn.Close() }

// GetConn implements kiface.IConnection.
func (c *Connection) GetConn() *net.TCPConn { return c.conn }

// GetConnID implements kiface.IConnection.
func (c *Connection) GetConnID() uint64 { return c.connID }

// GetRemoteAddr implements kiface.IConnection.
func (c *Connection) GetRemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// LocalAddr implements kiface.IConnection.
func (c *Connection) LocalAddr() net.Addr { return c.conn.LocalAddr() }

// SendMsg implements kiface.IConnection. Phase 2.
func (c *Connection) SendMsg(msgID uint32, data []byte) error { return kiface.ErrNotImplemented }

// IsAlive implements kiface.IConnection. Phase 2.
func (c *Connection) IsAlive(timeout time.Duration) bool { return true }

// SetHeartBeat implements kiface.IConnection. Phase 2.
func (c *Connection) SetHeartBeat(hb kiface.IHeartbeatChecker) {}

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
