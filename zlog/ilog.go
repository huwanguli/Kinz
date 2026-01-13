package zlog

type ILogger interface {
	// InfoF ErrorF 不使用上下文
	InfoF(format string, args ...interface{})
	ErrorF(format string, args ...interface{})
}
