// Package kiface defines the contract layer of the Kinz framework.
// Implementations live in knet; business code depends on these interfaces only.
package kiface

import "errors"

// Sentinel errors returned by the framework. Wrap them with %w to preserve identity.
var (
	// ErrServerClosed reports that the server is already shut down.
	ErrServerClosed = errors.New("kinz: server is closed")
	// ErrConnClosed reports that the connection is closed.
	ErrConnClosed = errors.New("kinz: connection is closed")
	// ErrTooLargePacket reports that a packet exceeds the configured max size.
	ErrTooLargePacket = errors.New("kinz: packet exceeds max size")
	// ErrServerFull reports that the server reached its max connection count.
	ErrServerFull = errors.New("kinz: server reached max connections")
	// ErrProtocol reports a malformed or unsupported wire-protocol payload.
	ErrProtocol = errors.New("kinz: protocol error")
	// ErrTimeout reports that an operation exceeded its deadline.
	ErrTimeout = errors.New("kinz: operation timed out")
	// ErrConnNotFound reports that a connection id is not registered.
	ErrConnNotFound = errors.New("kinz: connection not found")
	// ErrMsgIDRegistered reports a duplicate message-id registration.
	ErrMsgIDRegistered = errors.New("kinz: message id already registered")
	// ErrNotImplemented is a placeholder error for methods implemented in later phases.
	ErrNotImplemented = errors.New("kinz: not implemented")
)
