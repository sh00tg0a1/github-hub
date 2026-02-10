package main

import (
	"flag"
	"net/http"
	"os"

	"github-hub/internal/quality/api"
	"github-hub/internal/quality/logger"
	"github-hub/internal/quality/storage"
)

func main() {
	// 解析命令行参数
	var (
		addr       = flag.String("addr", ":5001", "服务器监听地址")
		dbDSN      = flag.String("db", "", "MySQL数据库连接字符串 (必需)")
		logLevel   = flag.String("log-level", "info", "日志级别: debug, info, warn, error")
		jsonFormat = flag.Bool("log-json", false, "使用 JSON 格式日志")
		noColor    = flag.Bool("log-no-color", false, "禁用彩色日志输出")
	)
	flag.Parse()

	// 配置日志系统
	level := logger.INFO
	switch *logLevel {
	case "debug":
		level = logger.DEBUG
	case "info":
		level = logger.INFO
	case "warn":
		level = logger.WARN
	case "error":
		level = logger.ERROR
	}

	logger.SetLevel(level)
	logger.SetJSONFormat(*jsonFormat)
	logger.SetColor(!*noColor)

	logger.Info("Starting Quality Server")
	logger.Infof("Version: %s", "1.0.0")
	logger.Infof("Log level: %s", *logLevel)

	// 检查数据库连接字符串
	if *dbDSN == "" {
		logger.Fatal("MySQL database connection string is required. Use -db flag to provide it.")
	}

	// 创建 MySQL 存储
	store, err := storage.NewMySQLStorage(*dbDSN)
	if err != nil {
		logger.ErrorWithFields("Failed to create MySQL storage", map[string]interface{}{
			"error": err.Error(),
			"dsn":   *dbDSN,
		})
		os.Exit(1)
	}
	logger.Info("MySQL storage initialized successfully")

	// 创建质量引擎服务器
	server, err := api.NewServerWithStorage(store)
	if err != nil {
		logger.ErrorWithFields("Failed to create server", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// 创建HTTP多路复用器
	mux := http.NewServeMux()

	// 添加日志中间件
	handler := logger.LoggingMiddleware(mux)

	// 注册路由
	server.RegisterRoutes(mux)

	// 启动服务器
	logger.Infof("Server starting on %s", *addr)
	logger.Infof("Webhook endpoint: http://localhost%s/webhook", *addr)
	logger.Infof("API endpoint: http://localhost%s/api", *addr)
	logger.Info("Ready to accept requests")

	if err := http.ListenAndServe(*addr, handler); err != nil {
		logger.ErrorWithFields("Failed to start server", map[string]interface{}{
			"error": err.Error(),
			"addr":  *addr,
		})
		os.Exit(1)
	}
}
