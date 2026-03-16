// Package logger 提供 HTTP 请求日志中间件
package logger

import (
	"fmt"
	"net/http"
	"time"
)

// responseWriter 用于捕获状态码和响应大小
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// LoggingMiddleware HTTP 请求日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 生成请求 ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// 创建响应包装器
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 设置响应头
		w.Header().Set("X-Request-ID", requestID)

		// 记录请求开始（使用基本日志避免中间件自身的性能问题）
		Debugf("HTTP %s %s started | request_id=%s | remote=%s",
			r.Method, r.URL.Path, requestID, r.RemoteAddr)

		// 捕获 panic
		defer func() {
			if err := recover(); err != nil {
				Errorf("Panic in HTTP handler | request_id=%s | error=%v",
					requestID, err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		// 调用下一个处理器
		next.ServeHTTP(rw, r)

		// 计算耗时
		duration := time.Since(start)

		// 记录请求完成
		logLevel := INFO
		if rw.statusCode >= 400 {
			logLevel = ERROR
		} else if duration > time.Second {
			logLevel = WARN
		}

		msg := fmt.Sprintf("HTTP %s %s completed | status=%d | size=%d | duration=%dms",
			r.Method, r.URL.Path, rw.statusCode, rw.size, duration.Milliseconds())

		// 使用带请求 ID 的日志记录器
		logger := WithRequest(requestID)
		logger.log(logLevel, msg, nil, "")
	})
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// RequestIDMiddleware 简单的请求 ID 中间件（只添加 ID，不记录日志）
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}
