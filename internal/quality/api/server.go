package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github-hub/internal/quality/handlers"
	"github-hub/internal/quality/logger"
	"github-hub/internal/quality/models"
	"github-hub/internal/quality/storage"
)

// Server 质量引擎服务器
type Server struct {
	storage     storage.Storage
	prHandler   *handlers.PRHandler
	pushHandler *handlers.PushHandler
	qualityDir  string
	startTime   time.Time
}

// NewServerWithStorage 使用提供的存储创建新的质量引擎服务器
func NewServerWithStorage(store storage.Storage) (*Server, error) {
	// 创建处理器
	prHandler := handlers.NewPRHandler(store)
	pushHandler := handlers.NewPushHandler(store)

	return &Server{
		storage:     store,
		prHandler:   prHandler,
		pushHandler: pushHandler,
		qualityDir:  "/usr/local/share/quality-data",
		startTime:   time.Now(),
	}, nil
}

// RegisterRoutes 注册路由
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Webhook 端点
	mux.HandleFunc("/webhook", s.handleWebhook)

	// API 端点
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/repositories", s.handleRepositories)
	mux.HandleFunc("/api/mock/events", s.handleMockEvents)
	mux.HandleFunc("/api/mock/simulate/", s.handleMockSimulate)
	mux.HandleFunc("/api/custom-test", s.handleCustomTest)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/check-login", s.handleCheckLogin)
	mux.HandleFunc("/api/status", s.handleStatus)

	// 动态路由处理
	mux.HandleFunc("/api/", s.handleDynamicRoutes)

	// 静态文件 (仅在文件存储模式下)
	if s.qualityDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(filepath.Join(s.qualityDir, "static"))))
	}
}

// handleWebhook 处理Webhook事件
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取事件类型
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	logger.Infof("DEBUG: Received event: %s", eventType)

	// 事件过滤逻辑
	shouldProcess := false

	if eventType == "push" {
		// Push事件过滤：只处理main分支
		shouldProcess = models.ShouldProcessPushEvent(payload)
		if shouldProcess {
			logger.Infof("Processing push event")
		} else {
			logger.Infof("Skipping push event")
		}

	} else if eventType == "pull_request" {
		// PR事件过滤：只处理非main分支合入main分支的事件
		shouldProcess = models.ShouldProcessPREvent(payload)
		if shouldProcess {
			logger.Infof("Processing PR event")
		} else {
			logger.Infof("Skipping PR event")
		}
	}

	if !shouldProcess {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "skipped",
			"event":  eventType,
		})
		return
	}

	// 异步处理事件
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Infof("ERROR: Panic in event processing: %v", r)
			}
		}()

		// 根据事件类型处理
		if eventType == "push" {
			s.pushHandler.Handle(payload)
		} else if eventType == "pull_request" {
			s.prHandler.Handle(payload)
		} else {
			logger.Infof("WARN: Unknown event type: %s", eventType)
		}
	}()

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "received",
		"event":  eventType,
	})
}

// handleEvents 处理事件列表请求
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetEvents(w, r)
	case http.MethodDelete:
		s.handleDeleteAllEvents(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetEvents 处理获取事件列表
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	eventType := r.URL.Query().Get("event_type")
	status := r.URL.Query().Get("status")
	branch := r.URL.Query().Get("branch")
	repository := r.URL.Query().Get("repository")

	// 分页参数
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	// 默认分页参数
	page := 1
	pageSize := 20
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 如果没有过滤条件，使用数据库分页查询（性能优化）
	if eventType == "" && status == "" && branch == "" && repository == "" {
		offset := (page - 1) * pageSize
		events, total, err := s.storage.ListEventsPaginated(offset, pageSize)
		if err != nil {
			http.Error(w, "failed to list events", http.StatusInternalServerError)
			return
		}

		totalPages := (total + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}

		// 格式化响应
		response := map[string]interface{}{
			"success": true,
			"data":    events,
			"pagination": map[string]interface{}{
				"page":        page,
				"page_size":   pageSize,
				"total":       total,
				"total_pages": totalPages,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 如果有过滤条件，使用原有的内存过滤方式
	events, err := s.storage.ListEvents()
	if err != nil {
		http.Error(w, "failed to list events", http.StatusInternalServerError)
		return
	}

	// 过滤事件
	filteredEvents := []*models.GitHubEvent{}
	for _, event := range events {
		// 按事件类型过滤
		if eventType != "" && string(event.EventType) != eventType {
			continue
		}
		// 按状态过滤
		if status != "" && string(event.EventStatus) != status {
			continue
		}
		// 按分支过滤
		if branch != "" && event.Branch != branch {
			continue
		}
		// 按仓库过滤
		if repository != "" && event.Repository != repository {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}

	// 计算分页信息
	totalEvents := len(filteredEvents)
	totalPages := (totalEvents + pageSize - 1) / pageSize
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	// 计算起止索引
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > totalEvents {
		end = totalEvents
	}
	if start > totalEvents {
		start = totalEvents
	}

	// 获取当前页数据
	var pagedEvents []*models.GitHubEvent
	if start < totalEvents {
		pagedEvents = filteredEvents[start:end]
	}

	// 格式化响应
	response := map[string]interface{}{
		"success":     true,
		"data":        pagedEvents,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       totalEvents,
			"total_pages": totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCustomTest 处理自定义测试请求
func (s *Server) handleCustomTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var request struct {
		Payload map[string]interface{} `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// 检查payload是否存在
	if request.Payload == nil {
		http.Error(w, "missing payload", http.StatusBadRequest)
		return
	}

	// 提取事件类型
	eventTypeStr, ok := request.Payload["event_type"].(string)
	if !ok {
		http.Error(w, "missing event_type", http.StatusBadRequest)
		return
	}

	// 构建GitHub Webhook格式的payload
	webhookPayload := map[string]interface{}{}

	// 根据事件类型构建不同的Webhook格式
	switch eventTypeStr {
	case "push":
		// 构建push事件格式
		webhookPayload["ref"] = "refs/heads/" + request.Payload["branch"].(string)
		webhookPayload["repository"] = map[string]interface{}{
			"full_name": request.Payload["repository"].(string),
		}
		webhookPayload["pusher"] = map[string]interface{}{
			"name": request.Payload["pusher"].(string),
		}
		webhookPayload["after"] = request.Payload["commit_sha"].(string)
	case "pull_request":
		// 构建PR事件格式
		webhookPayload["action"] = request.Payload["pr_action"].(string)
		webhookPayload["number"] = toFloat64(request.Payload["pr_number"])
		webhookPayload["pull_request"] = map[string]interface{}{
			"title": request.Payload["pr_title"].(string),
			"user": map[string]interface{}{
				"login": request.Payload["pr_author"].(string),
			},
			"head": map[string]interface{}{
				"ref": request.Payload["source_branch"].(string),
			},
			"base": map[string]interface{}{
				"ref": request.Payload["target_branch"].(string),
			},
		}
		webhookPayload["repository"] = map[string]interface{}{
			"full_name": request.Payload["repository"].(string),
		}
	default:
		http.Error(w, "unsupported event type", http.StatusBadRequest)
		return
	}

	// 事件过滤逻辑
	shouldProcess := false

	if eventTypeStr == "push" {
		// Push事件过滤：只处理main分支
		shouldProcess = models.ShouldProcessPushEvent(webhookPayload)
	} else if eventTypeStr == "pull_request" {
		// PR事件过滤：只处理非main分支合入main分支的事件
		shouldProcess = models.ShouldProcessPREvent(webhookPayload)
	}

	if !shouldProcess {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "skipped",
			"event":   eventTypeStr,
			"message": "事件被跳过（非main分支或不满足处理条件）",
		})
		return
	}

	// 准备事件数据
	eventData := map[string]interface{}{
		"event_type": eventTypeStr,
		"repository": request.Payload["repository"].(string),
	}

	if eventTypeStr == "push" {
		eventData["branch"] = request.Payload["branch"].(string)
		eventData["commit_sha"] = request.Payload["commit_sha"].(string)
		eventData["pusher"] = request.Payload["pusher"].(string)
		eventData["changed_files"] = request.Payload["changed_files"].(string)
	} else if eventTypeStr == "pull_request" {
		eventData["pr_number"] = toInt(request.Payload["pr_number"])
		eventData["pr_action"] = request.Payload["pr_action"].(string)
		eventData["pr_title"] = request.Payload["pr_title"].(string)
		eventData["pr_author"] = request.Payload["pr_author"].(string)
		eventData["source_branch"] = request.Payload["source_branch"].(string)
		eventData["target_branch"] = request.Payload["target_branch"].(string)
	}

	// 创建GitHubEvent
	eventType := models.EventType(eventTypeStr)
	event, err := models.NewGitHubEvent(eventData, eventType)
	if err != nil {
		logger.Infof("ERROR: Error creating event: %v", err)
		http.Error(w, "failed to create event: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 为事件创建质量检查项
	event.QualityChecks = models.CreateChecksForEvent(event.EventID)

	// 保存事件
	if err := s.storage.CreateEvent(event); err != nil {
		logger.Infof("ERROR: Failed to create event: %v", err)
		http.Error(w, "failed to save event", http.StatusInternalServerError)
		return
	}

	logger.Infof("Custom test event created: ID=%d, event_id=%s", event.ID, event.EventID)

	// 返回成功响应
	response := map[string]interface{}{
		"success": true,
		"event_type": eventTypeStr,
		"event_id": event.EventID,
		"message": "自定义测试事件已接收并开始处理",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// toFloat64 安全地将 interface{} 转换为 float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		// 尝试解析字符串
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f
		}
		// 如果是整数字符串，尝试解析为整数
		var i int64
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return float64(i)
		}
		return 0
	default:
		return 0
	}
}

// toInt 安全地将 interface{} 转换为 int
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	case string:
		// 尝试解析字符串
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}
// handleDynamicRoutes 处理动态路由
func (s *Server) handleDynamicRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// GET /api/events/{id} - 获取事件详情
	if r.Method == http.MethodGet && len(path) > len("/api/events/") {
		idStr := path[len("/api/events/"):]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleEventDetail(w, r, id)
			return
		}
	}

	// DELETE /api/events/{id} - 删除单个事件
	if r.Method == http.MethodDelete && len(path) > len("/api/events/") {
		idStr := path[len("/api/events/"):]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleDeleteEvent(w, r, id)
			return
		}
	}

	// PUT /api/events/{id}/status - 更新事件状态
	if r.Method == http.MethodPut && len(path) > len("/api/events/") && path[len(path)-len("/status"):] == "/status" {
		idStr := path[len("/api/events/") : len(path)-len("/status")]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleUpdateEventStatus(w, r, id)
			return
		}
	}

	// PUT /api/events/{id}/quality-checks/batch - 批量更新质量检查状态
	if r.Method == http.MethodPut && len(path) > len("/api/events/") && path[len(path)-len("/quality-checks/batch"):] == "/quality-checks/batch" {
		idStr := path[len("/api/events/") : len(path)-len("/quality-checks/batch")]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleBatchUpdateQualityChecks(w, r, id)
			return
		}
	}

	// GET /api/events/{eventID}/quality-checks - 获取质量检查列表
	if r.Method == http.MethodGet && len(path) > len("/api/events/") && path[len(path)-len("/quality-checks"):] == "/quality-checks" {
		eventIDStr := path[len("/api/events/") : len(path)-len("/quality-checks")]
		if eventIDStr != "" {
			s.handleQualityChecks(w, r, eventIDStr)
			return
		}
	}

	// PUT /api/quality-checks/{id} - 更新质量检查
	if r.Method == http.MethodPut && len(path) > len("/api/quality-checks/") {
		idStr := path[len("/api/quality-checks/"):]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleQualityCheckUpdate(w, r, id)
			return
		}
	}

	http.NotFound(w, r)
}

// handleDeleteEvent 处理删除单个事件
func (s *Server) handleDeleteEvent(w http.ResponseWriter, r *http.Request, id int) {
	if err := s.storage.DeleteEvent(id); err != nil {
		http.Error(w, "failed to delete event", http.StatusInternalServerError)
		logger.Infof("ERROR: Failed to delete event %d: %v", id, err)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "事件删除成功",
		"id":      id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteAllEvents 处理删除所有事件
func (s *Server) handleDeleteAllEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.DeleteAllEvents(); err != nil {
		http.Error(w, "failed to delete all events", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "数据库清空成功",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleEventDetail 处理事件详情请求
func (s *Server) handleEventDetail(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	event, err := s.storage.GetEvent(id)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    event,
	})
}

// handleRepositories 处理仓库列表请求
func (s *Server) handleRepositories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：返回空列表
	response := map[string]interface{}{
		"success": true,
		"data":    []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleQualityChecks 处理质量检查列表请求
func (s *Server) handleQualityChecks(w http.ResponseWriter, r *http.Request, eventID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	checks, err := s.storage.ListQualityChecksByEventID(eventID)
	if err != nil {
		checks = []models.PRQualityCheck{}
	}

	response := map[string]interface{}{
		"success": true,
		"data":    checks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleQualityCheckUpdate 处理质量检查更新请求
func (s *Server) handleQualityCheckUpdate(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	check, err := s.storage.GetQualityCheck(id)
	if err != nil {
		http.Error(w, "quality check not found", http.StatusNotFound)
		return
	}

	// 解析请求体
	var updateData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// 更新质量检查状态
	if statusStr, ok := updateData["status"].(string); ok {
		if status, err := models.ParseQualityCheckStatus(statusStr); err == nil {
			check.CheckStatus = status
		}
	}

	// 更新错误信息
	if errorMsg, ok := updateData["error_message"].(string); ok {
		check.ErrorMessage = &errorMsg
	}

	// 更新输出
	if output, ok := updateData["output"].(string); ok {
		check.Output = &output
	}

	// 更新完成时间
	now := models.Now()
	check.CompletedAt = &now

	// 保存更新
	if err := s.storage.UpdateQualityCheck(check); err != nil {
		http.Error(w, "failed to update quality check", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    check,
	})
}

// handleMockEvents 处理Mock事件列表请求
func (s *Server) handleMockEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从JSON文件读取预定义的Mock事件模板
	mockDataPath := filepath.Join(s.qualityDir, "github_webhook_payload_mock.json")
	mockData, err := os.ReadFile(mockDataPath)
	if err != nil {
		logger.Infof("DEBUG: Failed to read mock data file: %v", err)
		// 如果文件不存在，返回空数组
		response := map[string]interface{}{
			"success": true,
			"data":    []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	var mockEvents []map[string]interface{}
	if err := json.Unmarshal(mockData, &mockEvents); err != nil {
		logger.Infof("ERROR: Failed to parse mock data: %v", err)
		http.Error(w, "failed to parse mock data", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    mockEvents,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMockSimulate 处理模拟事件请求
func (s *Server) handleMockSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 提取事件类型
	path := r.URL.Path
	eventTypeStr := path[len("/api/mock/simulate/"):]
	if eventTypeStr == "" {
		http.Error(w, "missing event type", http.StatusBadRequest)
		return
	}

	// 从JSON文件读取mock数据
	mockDataPath := filepath.Join(s.qualityDir, "github_webhook_payload_mock.json")
	logger.Infof("DEBUG: Reading mock data from: %s", mockDataPath)
	mockData, err := os.ReadFile(mockDataPath)
	if err != nil {
		logger.Infof("DEBUG: Failed to read mock data file: %v", err)
		http.Error(w, "failed to read mock data", http.StatusInternalServerError)
		return
	}
	logger.Infof("DEBUG: Successfully read mock data file: %d bytes", len(mockData))

	var mockEvents []map[string]interface{}
	if err := json.Unmarshal(mockData, &mockEvents); err != nil {
		logger.Infof("ERROR: Failed to parse mock data: %v", err)
		http.Error(w, "failed to parse mock data", http.StatusInternalServerError)
		return
	}
	logger.Infof("DEBUG: Successfully parsed mock data: %d events", len(mockEvents))

	// 查找匹配的mock数据
	var selectedMockData map[string]interface{}
	simpleEventType := eventTypeStr
	var action string
	if strings.HasPrefix(eventTypeStr, "pull_request.") {
		simpleEventType = "pull_request"
		action = strings.TrimPrefix(eventTypeStr, "pull_request.")
	}
	logger.Infof("DEBUG: Looking for event type: %s, simple type: %s, action: %s", eventTypeStr, simpleEventType, action)

	for i, mockEvent := range mockEvents {
		if mockEventType, ok := mockEvent["event_type"].(string); ok {
			logger.Infof("DEBUG: Checking event %d: type=%s", i, mockEventType)
			if mockEventType == simpleEventType {
				// 对于PR事件，检查action是否匹配
				if simpleEventType == "pull_request" && action != "" {
					if mockAction, ok := mockEvent["pr_action"].(string); ok {
						logger.Infof("DEBUG: Checking PR action: %s vs %s", mockAction, action)
						if mockAction == action {
							selectedMockData = mockEvent
							logger.Infof("DEBUG: Found matching PR event with action: %s", action)
							break
						}
					}
				} else {
					selectedMockData = mockEvent
					logger.Infof("DEBUG: Found matching event: %s", mockEventType)
					break
				}
			}
		}
	}

	// 如果没有找到匹配的mock数据，使用空数据
	if selectedMockData == nil {
		selectedMockData = make(map[string]interface{})
		selectedMockData["event_type"] = eventTypeStr
		logger.Debugf("No matching mock data found, using empty data")
	} else {
		logger.Infof("DEBUG: Selected mock data: %+v", selectedMockData)
	}

	// 异步处理事件
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Infof("ERROR: Panic in mock event processing: %v", r)
			}
		}()

		// 根据事件类型处理
		if simpleEventType == "pull_request" {
			s.prHandler.Handle(selectedMockData)
		} else if simpleEventType == "push" {
			s.pushHandler.Handle(selectedMockData)
		} else {
			logger.Infof("WARN: Unknown mock event type: %s", eventTypeStr)
		}
	}()

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"message":    "Mock event received and being processed",
		"event_type": eventTypeStr,
	})
}

// handleLogin 处理登录请求
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：固定返回登录成功
	response := map[string]interface{}{
		"success":  true,
		"message":  "登录成功",
		"username": "admin",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLogout 处理登出请求
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：固定返回登出成功
	response := map[string]interface{}{
		"success": true,
		"message": "登出成功",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCheckLogin 处理登录状态检查请求
func (s *Server) handleCheckLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：默认返回未登录
	response := map[string]interface{}{
		"is_logged_in": false,
		"username":     "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus 处理系统状态请求
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 计算运行时间
	uptime := time.Since(s.startTime)
	uptimeStr := formatUptime(uptime)

	// 获取事件统计（使用优化的统计查询）
	totalEvents, pendingEvents, err := s.storage.GetEventStats()
	if err != nil {
		totalEvents = 0
		pendingEvents = 0
	}

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"service_status":  "healthy",
			"database_status": "connected",
			"total_events":    totalEvents,
			"pending_events":  pendingEvents,
			"db_type":         "MySQL",
			"db_host":         "quality-mysql",
			"db_name":         "github_hub",
			"version":         "1.0.0",
			"uptime":          uptimeStr,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpdateEventStatus 处理更新事件状态请求
func (s *Server) handleUpdateEventStatus(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查事件是否存在
	event, err := s.storage.GetEvent(id)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	// 解析请求体
	var updateData struct {
		EventStatus string `json:"event_status"`
		ProcessedAt string `json:"processed_at"` // 可选，ISO 8601 格式
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// 如果提供了 event_status，则更新
	if updateData.EventStatus != "" {
		newStatus, err := models.ParseEventStatus(updateData.EventStatus)
		if err != nil {
			http.Error(w, "invalid event_status value", http.StatusBadRequest)
			return
		}

		var processedAt *models.LocalTime
		if updateData.ProcessedAt != "" {
			t, err := time.Parse(time.RFC3339, updateData.ProcessedAt)
			if err != nil {
				http.Error(w, "invalid processed_at format, use ISO 8601", http.StatusBadRequest)
				return
			}
			lt := models.FromTime(t)
			processedAt = &lt
		} else if newStatus == models.EventStatusCompleted || newStatus == models.EventStatusFailed {
			// 自动设置处理时间
			now := models.Now()
			processedAt = &now
		}

		if err := s.storage.UpdateEventStatus(id, newStatus, processedAt); err != nil {
			http.Error(w, "failed to update event status", http.StatusInternalServerError)
			return
		}
		event.EventStatus = newStatus
		event.ProcessedAt = processedAt
	}

	// 返回更新后的事件
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "事件状态更新成功",
		"data":    event,
	})
}

// handleBatchUpdateQualityChecks 处理批量更新质量检查请求
func (s *Server) handleBatchUpdateQualityChecks(w http.ResponseWriter, r *http.Request, eventID int) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查事件是否存在
	event, err := s.storage.GetEvent(eventID)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	// 解析请求体
	var updateData struct {
		QualityChecks []struct {
			ID           int     `json:"id"`
			CheckStatus  *string `json:"check_status"`   // 使用指针以区分零值和未设置
			ErrorMessage *string `json:"error_message"`
			Output       *string `json:"output"`
			StartedAt    *string `json:"started_at"`    // ISO 8601 格式
			CompletedAt  *string `json:"completed_at"`  // ISO 8601 格式
			Duration     *float64 `json:"duration_seconds"`
		} `json:"quality_checks"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if len(updateData.QualityChecks) == 0 {
		http.Error(w, "quality_checks array is required", http.StatusBadRequest)
		return
	}

	// 获取现有质量检查
	existingChecks := event.QualityChecks
	existingCheckMap := make(map[int]*models.PRQualityCheck)
	for i := range existingChecks {
		existingCheckMap[existingChecks[i].ID] = &existingChecks[i]
	}

	// 准备更新的检查项
	var checksToUpdate []models.PRQualityCheck
	now := models.Now()

	for _, update := range updateData.QualityChecks {
		existing, exists := existingCheckMap[update.ID]
		if !exists {
			http.Error(w, fmt.Sprintf("quality check with id %d not found", update.ID), http.StatusNotFound)
			return
		}

		check := *existing

		// 只更新提供的字段
		if update.CheckStatus != nil {
			status, err := models.ParseQualityCheckStatus(*update.CheckStatus)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid check_status for check %d", update.ID), http.StatusBadRequest)
				return
			}
			check.CheckStatus = status
		}

		if update.ErrorMessage != nil {
			check.ErrorMessage = update.ErrorMessage
		}

		if update.Output != nil {
			check.Output = update.Output
		}

		if update.StartedAt != nil {
			t, err := time.Parse(time.RFC3339, *update.StartedAt)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid started_at format for check %d", update.ID), http.StatusBadRequest)
				return
			}
			lt := models.FromTime(t)
			check.StartedAt = &lt
		}

		if update.CompletedAt != nil {
			t, err := time.Parse(time.RFC3339, *update.CompletedAt)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid completed_at format for check %d", update.ID), http.StatusBadRequest)
				return
			}
			lt := models.FromTime(t)
			check.CompletedAt = &lt

			// 如果设置了完成时间但没有持续时间，则自动计算
			if update.Duration == nil && check.StartedAt != nil {
				duration := check.CompletedAt.ToTime().Sub(check.StartedAt.ToTime()).Seconds()
				check.DurationSeconds = &duration
			}
		}

		if update.Duration != nil {
			check.DurationSeconds = update.Duration
		}

		check.UpdatedAt = now
		checksToUpdate = append(checksToUpdate, check)
	}

	// 批量更新
	if err := s.storage.BatchUpdateQualityChecks(checksToUpdate); err != nil {
		http.Error(w, "failed to update quality checks", http.StatusInternalServerError)
		return
	}

	// 返回更新后的质量检查列表
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("成功更新 %d 个质量检查项", len(checksToUpdate)),
		"data":    checksToUpdate,
	})
}

// formatUptime 格式化运行时间
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%d分%d秒", minutes, seconds)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%d小时%d分", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%d天%d小时", days, hours)
	}
}
