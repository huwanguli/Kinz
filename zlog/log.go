package zlog

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	errorLog = log.New(os.Stdout, "", 0)
	infoLog  = log.New(os.Stdout, "", 0)
	loggers  = []*log.Logger{errorLog, infoLog}
	mu       sync.Mutex
)

// log levels
const (
	InfoLevel = iota
	ErrorLevel
	Disabled
)

// SetLevel 保持原有逻辑不变
func SetLevel(level int) {
	mu.Lock()
	defer mu.Unlock()

	for _, logger := range loggers {
		logger.SetOutput(os.Stdout)
	}

	if ErrorLevel < level {
		errorLog.SetOutput(io.Discard)
	}
	if InfoLevel < level {
		infoLog.SetOutput(io.Discard)
	}
}

// getCallerInfo 解析调用栈，返回「文件夹名/文件名:行号」格式的字符串
func getCallerInfo() string {
	// 栈帧跳过规则：
	// 0: getCallerInfo 自身
	// 1: InfoF/ErrorF 方法
	// 2: 业务代码调用 InfoF/ErrorF 的位置（目标位置）
	_, fullFile, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown/unknown:0"
	}

	// 解析全路径：提取 父文件夹名 + 文件名
	dir := filepath.Dir(fullFile)       // 获取文件所在的文件夹全路径
	dirName := filepath.Base(dir)       // 提取文件夹名（最后一级）
	fileName := filepath.Base(fullFile) // 提取文件名

	// 拼接成「文件夹名/文件名:行号」
	return dirName + "/" + fileName + ":" + strconv.Itoa(line)
}

var logInstance ILogger = &LogDefault{
	info:  infoLog,
	error: errorLog,
}

type LogDefault struct {
	info  *log.Logger
	error *log.Logger
}

func (l LogDefault) InfoF(format string, args ...interface{}) {
	// 手动拼接日志前缀：颜色 + 级别 + 时间 + 文件夹/文件:行号
	now := time.Now().Format("2006/01/02 15:04:05") // 对齐原 log.LstdFlags 时间格式
	callerInfo := getCallerInfo()
	prefix := "\033[34m[info ]\033[0m " + now + " " + callerInfo + ": "
	l.info.Printf(prefix+format, args...)
}

func (l LogDefault) ErrorF(format string, args ...interface{}) {
	now := time.Now().Format("2006/01/02 15:04:05")
	callerInfo := getCallerInfo()
	prefix := "\033[31m[error]\033[0m " + now + " " + callerInfo + ": "
	l.error.Printf(prefix+format, args...)
}

func L() ILogger {
	return logInstance
}

// 兼容旧日志：同步修改 Infof/Errorf 保证行号和文件夹名正确
var (
	Infof = func(format string, args ...interface{}) {
		now := time.Now().Format("2006/01/02 15:04:05")
		callerInfo := getCallerInfo()
		prefix := "\033[34m[info ]\033[0m " + now + " " + callerInfo + ": "
		infoLog.Printf(prefix+format, args...)
	}
	Errorf = func(format string, args ...interface{}) {
		now := time.Now().Format("2006/01/02 15:04:05")
		callerInfo := getCallerInfo()
		prefix := "\033[31m[error]\033[0m " + now + " " + callerInfo + ": "
		errorLog.Printf(prefix+format, args...)
	}
)
