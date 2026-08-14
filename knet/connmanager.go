package knet

import (
	"sync"

	"kinz/kiface"
)

// ConnManager implements kiface.IConnManager with a mutex-protected map and a
// max-connection limit (maxConn <= 0 means unlimited).
type ConnManager struct {
	mu          sync.RWMutex
	connections map[uint64]kiface.IConnection
	maxConn     int
}

// NewConnManager creates a ConnManager with the given limit.
func NewConnManager(maxConn int) *ConnManager {
	return &ConnManager{
		connections: make(map[uint64]kiface.IConnection),
		maxConn:     maxConn,
	}
}

// Add implements kiface.IConnManager. Returns ErrServerFull at the limit.
func (cm *ConnManager) Add(conn kiface.IConnection) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.maxConn > 0 && len(cm.connections) >= cm.maxConn {
		return kiface.ErrServerFull
	}
	cm.connections[conn.GetConnID()] = conn
	return nil
}

// Remove implements kiface.IConnManager.
func (cm *ConnManager) Remove(conn kiface.IConnection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.connections, conn.GetConnID())
}

// Get implements kiface.IConnManager.
func (cm *ConnManager) Get(connID uint64) (kiface.IConnection, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if conn, ok := cm.connections[connID]; ok {
		return conn, nil
	}
	return nil, kiface.ErrConnNotFound
}

// Len implements kiface.IConnManager.
func (cm *ConnManager) Len() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// Range implements kiface.IConnManager: iterates live connections; f returns
// false to stop. Connections are snapshotted under the lock and invoked
// outside it, so f may call back into the manager without deadlocking.
func (cm *ConnManager) Range(f func(connID uint64, conn kiface.IConnection) bool) {
	cm.mu.RLock()
	conns := make([]kiface.IConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	cm.mu.RUnlock()
	for _, conn := range conns {
		if !f(conn.GetConnID(), conn) {
			return
		}
	}
}

// ClearConn implements kiface.IConnManager. Stops connections outside the lock
// so Stop() may call Remove() without deadlocking.
func (cm *ConnManager) ClearConn() {
	cm.mu.Lock()
	conns := make([]kiface.IConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	cm.connections = make(map[uint64]kiface.IConnection)
	cm.mu.Unlock()
	for _, conn := range conns {
		conn.Stop()
	}
}
