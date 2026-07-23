package aibot

// logger.go 对应 Node src/logger.ts：DefaultLogger 默认日志实现。

import (
	"fmt"
	"io"
	"os"
	"time"
)

// DefaultLogger 默认日志实现，对应 Node src/logger.ts 的 DefaultLogger。
//
// 带有日志级别和时间戳的输出，格式：[{time}] [{prefix}] [{LEVEL}] {message} {args}。
// 默认 prefix 为 "AiBotSDK"，输出到 os.Stderr；可通过选项注入 nowFunc 与 writer 便于测试。
type DefaultLogger struct {
	prefix  string           // 日志前缀，对应 Node DefaultLogger.prefix，默认 "AiBotSDK"
	nowFunc func() time.Time // 时间源，默认 time.Now；测试可注入固定时间
	writer  io.Writer        // 输出目标，默认 os.Stderr
}

// DefaultLoggerOption 配置 DefaultLogger 的选项（构造时注入）。
type DefaultLoggerOption func(*DefaultLogger)

// WithLoggerPrefix 设置日志前缀。
func WithLoggerPrefix(prefix string) DefaultLoggerOption {
	return func(l *DefaultLogger) {
		l.prefix = prefix
	}
}

// WithLoggerNowFunc 设置时间源，便于测试注入固定时间。
func WithLoggerNowFunc(nowFunc func() time.Time) DefaultLoggerOption {
	return func(l *DefaultLogger) {
		l.nowFunc = nowFunc
	}
}

// WithLoggerWriter 设置输出目标，便于测试捕获日志。
func WithLoggerWriter(w io.Writer) DefaultLoggerOption {
	return func(l *DefaultLogger) {
		l.writer = w
	}
}

// NewDefaultLogger 构造默认日志实现，对应 Node new DefaultLogger(prefix)。
//
// prefix 为空时使用默认值 "AiBotSDK"。可通过 opts 注入 nowFunc / writer。
func NewDefaultLogger(prefix string, opts ...DefaultLoggerOption) *DefaultLogger {
	if prefix == "" {
		prefix = "AiBotSDK"
	}
	l := &DefaultLogger{
		prefix:  prefix,
		nowFunc: time.Now,
		writer:  os.Stderr,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// formatTime 返回 ISO 8601 时间字符串，对应 Node DefaultLogger.formatTime()（toISOString）。
func (l *DefaultLogger) formatTime() string {
	now := time.Now()
	if l.nowFunc != nil {
		now = l.nowFunc()
	}
	return now.UTC().Format(time.RFC3339Nano)
}

// log 写入一条日志，统一格式化。
func (l *DefaultLogger) log(level, message string, args ...any) {
	w := l.writer
	if w == nil {
		w = os.Stderr
	}
	line := fmt.Sprintf("[%s] [%s] [%s] %s", l.formatTime(), l.prefix, level, message)
	if len(args) > 0 {
		line += " " + fmt.Sprint(args...)
	}
	fmt.Fprintln(w, line)
}

// Debug 输出 DEBUG 级别日志。
func (l *DefaultLogger) Debug(message string, args ...any) {
	l.log("DEBUG", message, args...)
}

// Info 输出 INFO 级别日志。
func (l *DefaultLogger) Info(message string, args ...any) {
	l.log("INFO", message, args...)
}

// Warn 输出 WARN 级别日志。
func (l *DefaultLogger) Warn(message string, args ...any) {
	l.log("WARN", message, args...)
}

// Error 输出 ERROR 级别日志。
func (l *DefaultLogger) Error(message string, args ...any) {
	l.log("ERROR", message, args...)
}
