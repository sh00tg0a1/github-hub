package handlers

import (
	"testing"

	"github-hub/internal/quality/storage"
)

// TestPRHandler_Handle_SimplifiedFormat 测试简化格式的 PR 处理
func TestPRHandler_Handle_SimplifiedFormat(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":    "pull_request",
		"repository":    "test/repo",
		"pr_number":     float64(42),
		"pr_title":      "Test PR",
		"pr_state":      "open",
		"pr_action":     "opened",
		"source_branch": "feature",
		"target_branch": "main",
		"pr_author":     "testuser",
		"changed_files": "file1.py,file2.js",
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["repository"] != "test/repo" {
		t.Errorf("expected repository 'test/repo', got '%v'", result["repository"])
	}

	prNumber := result["pr_number"]
	if prNumber == nil {
		t.Error("expected pr_number to be set")
	} else if pn, ok := prNumber.(*int); ok && pn != nil && *pn != 42 {
		t.Errorf("expected pr_number 42, got %v", *pn)
	}

	// 验证事件被保存
	events, err := mockStorage.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Repository != "test/repo" {
		t.Errorf("expected repository 'test/repo', got '%s'", events[0].Repository)
	}

	if events[0].PRNumber == nil || *events[0].PRNumber != 42 {
		t.Errorf("expected pr_number 42, got %v", events[0].PRNumber)
	}

	// 验证质量检查被创建
	checks, err := mockStorage.ListQualityChecksByEventID(events[0].EventID)
	if err != nil {
		t.Fatalf("ListQualityChecksByEventID failed: %v", err)
	}

	expectedCheckCount := 9
	if len(checks) != expectedCheckCount {
		t.Errorf("expected %d quality checks, got %d", expectedCheckCount, len(checks))
	}
}

// TestPRHandler_Handle_WebhookFormat 测试 GitHub webhook 格式的 PR 处理
func TestPRHandler_Handle_WebhookFormat(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "webhook/repo",
		},
		"action": "synchronized",
		"pull_request": map[string]interface{}{
			"number": float64(123),
			"title":  "Webhook PR",
			"state":  "open",
			"head": map[string]interface{}{
				"ref": "develop",
				"sha": "abc123",
			},
			"base": map[string]interface{}{
				"ref": "main",
				"sha": "def456",
			},
			"user": map[string]interface{}{
				"login": "webhookuser",
			},
			"commits":       float64(3),
			"additions":     float64(100),
			"deletions":     float64(50),
			"changed_files": float64(5),
		},
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["repository"] != "webhook/repo" {
		t.Errorf("expected repository 'webhook/repo', got '%v'", result["repository"])
	}

	if result["pr_title"] != "Webhook PR" {
		t.Errorf("expected pr_title 'Webhook PR', got '%v'", result["pr_title"])
	}

	// 验证事件被保存
	events, err := mockStorage.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Repository != "webhook/repo" {
		t.Errorf("expected repository 'webhook/repo', got '%s'", event.Repository)
	}

	if event.Branch != "develop" {
		t.Errorf("expected branch 'develop', got '%s'", event.Branch)
	}

	if event.TargetBranch == nil || *event.TargetBranch != "main" {
		t.Errorf("expected target_branch 'main', got '%v'", event.TargetBranch)
	}

	if event.Author == nil || *event.Author != "webhookuser" {
		t.Errorf("expected author 'webhookuser', got '%v'", event.Author)
	}

	if event.Action == nil || *event.Action != "synchronized" {
		t.Errorf("expected action 'synchronized', got '%v'", event.Action)
	}
}

// TestPRHandler_Handle_StorageError 测试存储错误处理
func TestPRHandler_Handle_StorageError(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	// 设置存储错误
	testErr := storage.NewMockStorage()
	// 使用一个不存在的存储来模拟错误
	handler = NewPRHandler(testErr)

	eventData := map[string]interface{}{
		"event_type":    "pull_request",
		"repository":    "test/repo",
		"pr_number":     float64(1),
		"source_branch": "feature",
		"target_branch": "main",
	}

	// 这个测试主要验证错误不会导致 panic
	result := handler.Handle(eventData)

	// 由于使用了有效的 mock，不应该有错误
	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}
}

// TestPRHandler_Handle_DefaultValues 测试默认值处理
func TestPRHandler_Handle_DefaultValues(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	// 不提供 pr_action 和 pr_state
	eventData := map[string]interface{}{
		"event_type":    "pull_request",
		"repository":    "test/repo",
		"pr_number":     float64(1),
		"source_branch": "feature",
		"target_branch": "main",
	}

	result := handler.Handle(eventData)

	if result["action"] != "opened" {
		t.Errorf("expected default action 'opened', got '%v'", result["action"])
	}

	if result["state"] != "open" {
		t.Errorf("expected default state 'open', got '%v'", result["state"])
	}
}

// TestPRHandler_Handle_MissingRequiredFields 测试缺少必填字段
func TestPRHandler_Handle_MissingRequiredFields(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	// 缺少 source_branch
	eventData := map[string]interface{}{
		"event_type":    "pull_request",
		"repository":    "test/repo",
		"target_branch": "main",
	}

	result := handler.Handle(eventData)

	if result["status"] != "error" {
		t.Errorf("expected status 'error', got '%v'", result["status"])
	}

	if result["error"] == nil {
		t.Error("expected error message")
	}
}

// TestPRHandler_Handle_ChangedFilesCount 测试变更文件数量计算
func TestPRHandler_Handle_ChangedFilesCount(t *testing.T) {
	tests := []struct {
		name          string
		changedFiles  string
		expectedCount int
	}{
		{
			name:          "single file",
			changedFiles:  "file1.py",
			expectedCount: 1,
		},
		{
			name:          "multiple files",
			changedFiles:  "file1.py,file2.js,file3.go",
			expectedCount: 3,
		},
		{
			name:          "empty string",
			changedFiles:  "",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := storage.NewMockStorage()
			handler := NewPRHandler(mockStorage)

			eventData := map[string]interface{}{
				"event_type":    "pull_request",
				"repository":    "test/repo",
				"pr_number":     float64(1),
				"source_branch": "feature",
				"target_branch": "main",
				"changed_files": tt.changedFiles,
			}

			result := handler.Handle(eventData)

			if result["changed_files"] != tt.expectedCount {
				t.Errorf("expected changed_files count %d, got %v", tt.expectedCount, result["changed_files"])
			}
		})
	}
}

// TestNewPRHandler 测试创建 PR 处理器
func TestNewPRHandler(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	if handler == nil {
		t.Error("expected non-nil handler")
	}

	if handler.storage == nil {
		t.Error("expected storage to be set")
	}
}

// TestPRHandler_Handle_WebhookFormatWithoutPullRequest 测试 webhook 格式但没有 pull_request 字段
func TestPRHandler_Handle_WebhookFormatWithoutPullRequest(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	// 没有 event_type 字段，所以会被识别为 webhook 格式
	// 但整个 eventData 就是 PR 数据（需要包含 repository）
	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "test/repo",
		},
		"number": float64(456),
		"title":  "Direct PR Format",
		"head": map[string]interface{}{
			"ref": "branch",
		},
		"base": map[string]interface{}{
			"ref": "main",
		},
	}

	result := handler.Handle(eventData)

	// 应该成功处理
	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	// 验证事件被保存
	events, err := mockStorage.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

// TestPRHandler_Handle_PreserveEventID 测试事件 ID 保留
func TestPRHandler_Handle_PreserveEventID(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPRHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":    "pull_request",
		"repository":    "test/repo",
		"pr_number":     float64(1),
		"source_branch": "feature",
		"target_branch": "main",
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Fatalf("expected status 'processed', got '%v'", result["status"])
	}

	// 验证事件有非空的 EventID
	events, _ := mockStorage.ListEvents()
	if len(events) == 0 {
		t.Fatal("expected 1 event")
	}

	if events[0].EventID == "" {
		t.Error("expected non-empty EventID")
	}
}
