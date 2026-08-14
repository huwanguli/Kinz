package kiface

import (
	"net"
	"time"
)

// IConnection wraps a single TCP connection with a reader and a writer
// goroutine, liveness tracking, and key-value properties.
type IConnection interface {
	// Start begins the read and write goroutines of this connection.
	Start()
	// Stop gracefully closes the connection. It is idempotent and safe to
	// call from multiple goroutines.
	Stop()
	// GetConn returns the underlying TCP connection.
	GetConn() *net.TCPConn
	// GetConnID returns the connection id (monotonically increasing).
	GetConnID() uint64
	// GetRemoteAddr returns the remote peer address.
	GetRemoteAddr() net.Addr
	// LocalAddr returns the local address of the connection.
	LocalAddr() net.Addr
	// SendMsg packs msgID/data with the server packet format and queues it for
	// writing. It returns ErrConnClosed when the connection is closed.
	SendMsg(msgID uint32, data []byte) error
	// IsAlive reports whether any message was received within timeout.
	IsAlive(timeout time.Duration) bool
	// SetHeartBeat binds a heartbeat checker to this connection.
	SetHeartBeat(hb IHeartbeatChecker)

	// SetProperty attaches a key-value property to the connection.
	SetProperty(key string, value interface{})
	// GetProperty returns the value for key, or an error when absent.
	GetProperty(key string) (interface{}, error)
	// RemoveProperty deletes the property for key.
	RemoveProperty(key string)
}
