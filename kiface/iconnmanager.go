package kiface

// IConnManager tracks live connections and enforces the max-connection limit.
type IConnManager interface {
	// Add registers a connection. Returns ErrServerFull when the limit is reached.
	Add(conn IConnection) error
	// Remove deregisters a connection.
	Remove(conn IConnection)
	// Get returns the connection with connID, or ErrConnNotFound.
	Get(connID uint64) (IConnection, error)
	// Len returns the number of live connections.
	Len() int
	// Range iterates live connections (f returns false to stop). Safe to call
	// connection methods from f.
	Range(f func(connID uint64, conn IConnection) bool)
	// ClearConn stops and removes all connections.
	ClearConn()
}
