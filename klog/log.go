package klog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Options configures a Logger.
type Options struct {
	// Level is the minimum severity to emit (default Info).
	Level Level
	// JSON selects the JSON handler instead of the text handler.
	JSON bool
	// AddSource includes the caller file:line in each record.
	AddSource bool
	// Output is the destination (default os.Stdout).
	Output io.Writer
}

// Logger is the default ILogger implementation backed by log/slog.
// Its level is dynamic: SetLevel takes effect immediately.
type Logger struct {
	l        *slog.Logger
	levelVar *slog.LevelVar
}

// New creates a Logger from opts.
func New(opts Options) *Logger {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.Level(opts.Level))
	handlerOpts := &slog.HandlerOptions{Level: levelVar, AddSource: opts.AddSource}

	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(opts.Output, handlerOpts)
	} else {
		handler = slog.NewTextHandler(opts.Output, handlerOpts)
	}
	return &Logger{l: slog.New(handler), levelVar: levelVar}
}

// SetLevel adjusts the minimum severity dynamically.
func (lg *Logger) SetLevel(level Level) { lg.levelVar.Set(slog.Level(level)) }

// Debug implements ILogger.
func (lg *Logger) Debug(msg string, args ...any) { lg.l.Debug(msg, args...) }

// Info implements ILogger.
func (lg *Logger) Info(msg string, args ...any) { lg.l.Info(msg, args...) }

// Warn implements ILogger.
func (lg *Logger) Warn(msg string, args ...any) { lg.l.Warn(msg, args...) }

// Error implements ILogger.
func (lg *Logger) Error(msg string, args ...any) { lg.l.Error(msg, args...) }

// InfoF implements ILogger.
func (lg *Logger) InfoF(format string, args ...any) {
	lg.l.Info(fmt.Sprintf(format, args...))
}

// ErrorF implements ILogger.
func (lg *Logger) ErrorF(format string, args ...any) {
	lg.l.Error(fmt.Sprintf(format, args...))
}

// With implements ILogger.
func (lg *Logger) With(fields ...any) ILogger {
	return &Logger{l: lg.l.With(fields...), levelVar: lg.levelVar}
}

var defaultLogger ILogger = New(Options{})

// L returns the package-level default logger.
func L() ILogger { return defaultLogger }

// SetDefault replaces the package-level default logger.
func SetDefault(l ILogger) { defaultLogger = l }

// Package-level convenience delegates to L().
func Debug(msg string, args ...any)     { L().Debug(msg, args...) }
func Info(msg string, args ...any)      { L().Info(msg, args...) }
func Warn(msg string, args ...any)      { L().Warn(msg, args...) }
func Error(msg string, args ...any)     { L().Error(msg, args...) }
func InfoF(format string, args ...any)  { L().InfoF(format, args...) }
func ErrorF(format string, args ...any) { L().ErrorF(format, args...) }
