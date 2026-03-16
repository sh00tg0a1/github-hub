// Package logger 提供结构化日志功能
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

// 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

// 日志级别名称映射
var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// 日志级别颜色映射
var levelColors = map[Level]string{
	DEBUG: "\033[0;36m", // Cyan
	INFO:  "\033[0;32m", // Green
	WARN:  "\033[1;33m", // Yellow
	ERROR: "\033[0;31m", // Red
	FATAL: "\033[1;31m", // Bold Red
}

const (
	ColorReset = "\033[0m"
)

// Config 日志配置
type Config struct {
	// 输出目标，默认 os.Stdout
	Out io.Writer
	// 最低日志级别
	Level Level
	// 是否使用 JSON 格式
	JSONFormat bool
	// 是否启用颜色（仅对文本格式有效）
	EnableColor bool
	// 是否显示调用者信息
	EnableCaller bool
	// 是否显示堆栈跟踪（ERROR 及以上）
	EnableStack bool
}

// Logger 日志记录器
type Logger struct {
	mu            sync.RWMutex
	out           io.Writer
	level         Level
	jsonFormat    bool
	enableColor   bool
	enableCaller  bool
	enableStack   bool
	fields        map[string]interface{}
	requestID     string
}

// NewLogger 创建新的日志记录器
func NewLogger(config Config) *Logger {
	if config.Out == nil {
		config.Out = os.Stdout
	}
	return &Logger{
		out:          config.Out,
		level:        config.Level,
		jsonFormat:   config.JSONFormat,
		enableColor:  config.EnableColor,
		enableCaller: config.EnableCaller,
		enableStack:  config.EnableStack,
		fields:       make(map[string]interface{}),
	}
}

// DefaultLogger 默认日志记录器
var DefaultLogger = NewLogger(Config{
	Level:        INFO,
	EnableColor:  true,
	EnableCaller: true,
	EnableStack:  true,
})

// SetLevel 设置全局日志级别
func SetLevel(level Level) {
	DefaultLogger.mu.Lock()
	DefaultLogger.level = level
	DefaultLogger.mu.Unlock()
}

// SetJSONFormat 设置 JSON 格式
func SetJSONFormat(enable bool) {
	DefaultLogger.mu.Lock()
	DefaultLogger.jsonFormat = enable
	DefaultLogger.mu.Unlock()
}

// SetColor 设置颜色输出
func SetColor(enable bool) {
	DefaultLogger.mu.Lock()
	DefaultLogger.enableColor = enable
	DefaultLogger.mu.Unlock()
}

// WithField 添加全局字段 - 返回新的 Logger 实例
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newLogger := &Logger{
		out:          l.out,
		level:        l.level,
		jsonFormat:   l.jsonFormat,
		enableColor:  l.enableColor,
		enableCaller: l.enableCaller,
		enableStack:  l.enableStack,
		fields:       make(map[string]interface{}),
		requestID:    l.requestID,
	}

	// 复制现有字段
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithRequest 设置请求 ID
func (l *Logger) WithRequest(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

// WithError 添加错误字段
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// log 记录日志 - 内部方法，使用写锁
func (l *Logger) log(level Level, msg string, fields map[string]interface{}, stack string) {
	if level < l.level {
		return
	}

	l.mu.RLock()
	enableCaller := l.enableCaller
	jsonFormat := l.jsonFormat
	requestID := l.requestID
	l.mu.RUnlock()

	now := time.Now()
	record := make(map[string]interface{})

	// 基础字段
	record["timestamp"] = now.Format("2006-01-02T15:04:05.000Z07:00")
	record["level"] = levelNames[level]
	record["message"] = msg

	// 添加请求 ID
	if requestID != "" {
		record["request_id"] = requestID
	}

	// 添加预存字段
	l.mu.RLock()
	for k, v := range l.fields {
		record[k] = v
	}
	l.mu.RUnlock()

	// 添加额外字段
	for k, v := range fields {
		if _, exists := record[k]; !exists {
			record[k] = v
		}
	}

	// 添加调用者信息
	if enableCaller {
		if pc, file, line, ok := runtime.Caller(2); ok {
			record["caller"] = fmt.Sprintf("%s:%d", file, line)
			if fn := runtime.FuncForPC(pc); fn != nil {
				record["function"] = fn.Name()
			}
		}
	}

	// 添加堆栈信息
	if stack != "" {
		record["stack"] = stack
	}

	// 输出日志
	if jsonFormat {
		jsonData, err := json.Marshal(record)
		if err != nil {
			fmt.Fprintf(l.out, "{\"level\":\"ERROR\",\"message\":\"failed to marshal log: %v\"}\n", err)
			return
		}
		fmt.Fprintln(l.out, string(jsonData))
	} else {
		l.logText(level, record)
	}
}

// logText 文本格式日志输出
func (l *Logger) logText(level Level, record map[string]interface{}) {
	l.mu.RLock()
	enableColor := l.enableColor
	l.mu.RUnlock()

	// 颜色
	color := levelColors[level]
	if !enableColor {
		color = ""
	}

	// 时间戳
	timestamp := record["timestamp"].(string)
	date := timestamp[:10]
	clock := timestamp[11:19]

	// 级别
	levelName := record["level"].(string)

	// 消息
	msg := record["message"].(string)

	// 构建输出
	output := fmt.Sprintf("%s[%s %s]%s %-5s | %s",
		color, date, clock, ColorReset, levelName, msg)

	// 添加 request_id
	if requestID, ok := record["request_id"]; ok {
		output += fmt.Sprintf(" | request_id=%v", requestID)
	}

	// 添加其他字段
	for k, v := range record {
		if k == "timestamp" || k == "level" || k == "message" || k == "request_id" {
			continue
		}
		output += fmt.Sprintf(" | %s=%v", k, v)
	}

	fmt.Fprintln(l.out, output)
}

// getStack 获取堆栈信息
func getStack(skip int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])
	// 跳过前几行
	lines := ""
	for i, line := range splitLines(stack) {
		if i < skip+2 || i > skip+10 {
			continue
		}
		lines += "  " + line + "\n"
	}
	return lines
}

func splitLines(s string) []string {
	var lines []string
	line := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(c)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// ==================== 实例方法 ====================

// Debug DEBUG 级别日志
func (l *Logger) Debug(msg string) {
	l.log(DEBUG, msg, nil, "")
}

// Debugf DEBUG 级别格式化日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...), nil, "")
}

// DebugWithFields DEBUG 级别带字段日志
func (l *Logger) DebugWithFields(msg string, fields map[string]interface{}) {
	l.log(DEBUG, msg, fields, "")
}

// Info INFO 级别日志
func (l *Logger) Info(msg string) {
	l.log(INFO, msg, nil, "")
}

// Infof INFO 级别格式化日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...), nil, "")
}

// InfoWithFields INFO 级别带字段日志
func (l *Logger) InfoWithFields(msg string, fields map[string]interface{}) {
	l.log(INFO, msg, fields, "")
}

// Warn WARN 级别日志
func (l *Logger) Warn(msg string) {
	l.log(WARN, msg, nil, "")
}

// Warnf WARN 级别格式化日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...), nil, "")
}

// WarnWithFields WARN 级别带字段日志
func (l *Logger) WarnWithFields(msg string, fields map[string]interface{}) {
	l.log(WARN, msg, fields, "")
}

// Error ERROR 级别日志
func (l *Logger) Error(msg string) {
	l.log(ERROR, msg, nil, getStack(2))
}

// Errorf ERROR 级别格式化日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...), nil, getStack(2))
}

// ErrorWithFields ERROR 级别带字段日志
func (l *Logger) ErrorWithFields(msg string, fields map[string]interface{}) {
	l.log(ERROR, msg, fields, getStack(2))
}

// Fatal FATAL 级别日志，记录后退出
func (l *Logger) Fatal(msg string) {
	l.log(FATAL, msg, nil, getStack(2))
	os.Exit(1)
}

// Fatalf FATAL 级别格式化日志，记录后退出
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...), nil, getStack(2))
	os.Exit(1)
}

// FatalWithFields FATAL 级别带字段日志，记录后退出
func (l *Logger) FatalWithFields(msg string, fields map[string]interface{}) {
	l.log(FATAL, msg, fields, getStack(2))
	os.Exit(1)
}

// ==================== 全局函数 ====================

// Debug DEBUG 级别日志
func Debug(msg string) {
	DefaultLogger.Debug(msg)
}

// Debugf DEBUG 级别格式化日志
func Debugf(format string, args ...interface{}) {
	DefaultLogger.Debugf(format, args...)
}

// DebugWithFields DEBUG 级别带字段日志
func DebugWithFields(msg string, fields map[string]interface{}) {
	DefaultLogger.DebugWithFields(msg, fields)
}

// Info INFO 级别日志
func Info(msg string) {
	DefaultLogger.Info(msg)
}

// Infof INFO 级别格式化日志
func Infof(format string, args ...interface{}) {
	DefaultLogger.Infof(format, args...)
}

// InfoWithFields INFO 级别带字段日志
func InfoWithFields(msg string, fields map[string]interface{}) {
	DefaultLogger.InfoWithFields(msg, fields)
}

// Warn WARN 级别日志
func Warn(msg string) {
	DefaultLogger.Warn(msg)
}

// Warnf WARN 级别格式化日志
func Warnf(format string, args ...interface{}) {
	DefaultLogger.Warnf(format, args...)
}

// WarnWithFields WARN 级别带字段日志
func WarnWithFields(msg string, fields map[string]interface{}) {
	DefaultLogger.WarnWithFields(msg, fields)
}

// Error ERROR 级别日志
func Error(msg string) {
	DefaultLogger.Error(msg)
}

// Errorf ERROR 级别格式化日志
func Errorf(format string, args ...interface{}) {
	DefaultLogger.Errorf(format, args...)
}

// ErrorWithFields ERROR 级别带字段日志
func ErrorWithFields(msg string, fields map[string]interface{}) {
	DefaultLogger.ErrorWithFields(msg, fields)
}

// Fatal FATAL 级别日志
func Fatal(msg string) {
	DefaultLogger.Fatal(msg)
}

// Fatalf FATAL 级别格式化日志
func Fatalf(format string, args ...interface{}) {
	DefaultLogger.Fatalf(format, args...)
}

// WithField 添加全局字段
func WithField(key string, value interface{}) *Logger {
	return DefaultLogger.WithField(key, value)
}

// WithFields 添加多个全局字段
func WithFields(fields map[string]interface{}) *Logger {
	l := DefaultLogger
	newLogger := &Logger{
		out:          l.out,
		level:        l.level,
		jsonFormat:   l.jsonFormat,
		enableColor:  l.enableColor,
		enableCaller: l.enableCaller,
		enableStack:  l.enableStack,
		fields:       make(map[string]interface{}),
		requestID:    l.requestID,
	}

	l.mu.RLock()
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	l.mu.RUnlock()

	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// WithRequest 设置请求 ID
func WithRequest(requestID string) *Logger {
	return DefaultLogger.WithRequest(requestID)
}

// WithError 添加错误字段
func WithError(err error) *Logger {
	return DefaultLogger.WithError(err)
}

// Printf 兼容标准 log.Printf
func Printf(format string, args ...interface{}) {
	DefaultLogger.Infof(format, args...)
}

// Println 兼容标准 log.Println
func Println(args ...interface{}) {
	msg := fmt.Sprint(args...)
	DefaultLogger.Info(msg)
}

// StdLogger 返回一个标准库的 log.Logger
func StdLogger() *log.Logger {
	return log.New(&logWriter{logger: DefaultLogger}, "", 0)
}

// logWriter 实现 io.Writer 接口
type logWriter struct {
	logger *Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}
