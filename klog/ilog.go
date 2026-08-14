// Package klog provides the Kinz logging contract and a log/slog-based
// default implementation. It is the logging seam: business code can inject
// any ILogger via klog.SetDefault or a server option.
package klog

import "log/slog"

// Level is a log severity level (alias of slog.Level).
type Level = slog.Level

// Predefined severity levels.
const (
	LevelDebug = slog.LevelDebug // -4
	LevelInfo  = slog.LevelInfo  // 0
	LevelWarn  = slog.LevelWarn  // 4
	LevelError = slog.LevelError // 8
)

// ILogger is the framework's logging contract.
type ILogger interface {
	// Debug logs at debug level.
	Debug(msg string, args ...any)
	// Info logs at info level.
	Info(msg string, args ...any)
	// Warn logs at warn level.
	Warn(msg string, args ...any)
	// Error logs at error level.
	Error(msg string, args ...any)
	// InfoF logs a printf-formatted message at info level (legacy compatibility).
	InfoF(format string, args ...any)
	// ErrorF logs a printf-formatted message at error level (legacy compatibility).
	ErrorF(format string, args ...any)
	// With returns a logger with structured fields attached.
	With(fields ...any) ILogger
}
