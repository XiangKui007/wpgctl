// Package util 提供跨模块通用工具（日志、路径、校验辅助等）。
package util

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

const (
	// LevelDebug 调试。
	LevelDebug Level = iota
	// LevelInfo 信息。
	LevelInfo
	// LevelWarn 警告。
	LevelWarn
	// LevelError 错误。
	LevelError
)

var (
	logMu     sync.Mutex
	logLevel  = LevelInfo
	logWriter io.Writer = os.Stderr
)

// SetLevel 设置全局日志级别。
func SetLevel(l Level) {
	logMu.Lock()
	defer logMu.Unlock()
	logLevel = l
}

// SetWriter 设置日志输出目标。
func SetWriter(w io.Writer) {
	logMu.Lock()
	defer logMu.Unlock()
	logWriter = w
}

func logf(level Level, tag, format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	if level < logLevel {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(logWriter, "%s [%s] %s %s\n", ts, tag, levelName(level), msg)
}

func levelName(l Level) string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Debugf 输出调试日志。
func Debugf(format string, args ...interface{}) { logf(LevelDebug, "wpgctl", format, args...) }

// Infof 输出信息日志。
func Infof(format string, args ...interface{}) { logf(LevelInfo, "wpgctl", format, args...) }

// Warnf 输出警告日志。
func Warnf(format string, args ...interface{}) { logf(LevelWarn, "wpgctl", format, args...) }

// Errorf 输出错误日志。
func Errorf(format string, args ...interface{}) { logf(LevelError, "wpgctl", format, args...) }

// Successf 输出成功提示（绿色语义，终端友好）。
func Successf(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(logWriter, "%s [wpgctl] OK    %s\n", ts, msg)
}
