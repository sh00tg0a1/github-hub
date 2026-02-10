package handlers

import (
	"testing"

	"github-hub/internal/quality/storage"
)

// TestPushHandler_Handle_SimplifiedFormat 测试简化格式的 Push 处理
func TestPushHandler_Handle_SimplifiedFormat(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":  "push",
		"repository":  "test/repo",
		"branch":      "main",
		"commit_sha":  "abc123def",
		"pusher":      "testuser",
		"changed_files": "file1.py,file2.js,file3.go",
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["repository"] != "test/repo" {
		t.Errorf("expected repository 'test/repo', got '%v'", result["repository"])
	}

	if result["branch"] != "main" {
		t.Errorf("expected branch 'main', got '%v'", result["branch"])
	}

	if result["commit_sha"] != "abc123def" {
		t.Errorf("expected commit_sha 'abc123def', got '%v'", result["commit_sha"])
	}

	if result["pusher"] != "testuser" {
		t.Errorf("expected pusher 'testuser', got '%v'", result["pusher"])
	}

	expectedFilesCount := 3
	if result["changed_files"] != expectedFilesCount {
		t.Errorf("expected changed_files count %d, got %v", expectedFilesCount, result["changed_files"])
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
	if event.Repository != "test/repo" {
		t.Errorf("expected repository 'test/repo', got '%s'", event.Repository)
	}

	if event.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", event.Branch)
	}

	if event.Pusher == nil || *event.Pusher != "testuser" {
		t.Errorf("expected pusher 'testuser', got '%v'", event.Pusher)
	}

	// 验证质量检查被创建
	checks, err := mockStorage.ListQualityChecksByEventID(event.EventID)
	if err != nil {
		t.Fatalf("ListQualityChecksByEventID failed: %v", err)
	}

	expectedCheckCount := 9
	if len(checks) != expectedCheckCount {
		t.Errorf("expected %d quality checks, got %d", expectedCheckCount, len(checks))
	}
}

// TestPushHandler_Handle_WebhookFormat 测试 GitHub webhook 格式的 Push 处理
func TestPushHandler_Handle_WebhookFormat(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "webhook/repo",
		},
		"ref": "refs/heads/main",
		"head_commit": map[string]interface{}{
			"id": "sha789",
			"author": map[string]interface{}{
				"name": "commit author",
			},
			"message": "Test commit",
		},
		"pusher": map[string]interface{}{
			"name": "webhookuser",
		},
		"commits": []interface{}{
			map[string]interface{}{
				"added":    []interface{}{"newfile.py"},
				"modified": []interface{}{"changed.go", "updated.js"},
				"removed":  []interface{}{"deleted.txt"},
			},
		},
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["repository"] != "webhook/repo" {
		t.Errorf("expected repository 'webhook/repo', got '%v'", result["repository"])
	}

	if result["branch"] != "main" {
		t.Errorf("expected branch 'main', got '%v'", result["branch"])
	}

	if result["commit_sha"] != "sha789" {
		t.Errorf("expected commit_sha 'sha789', got '%v'", result["commit_sha"])
	}

	if result["pusher"] != "webhookuser" {
		t.Errorf("expected pusher 'webhookuser', got '%v'", result["pusher"])
	}

	// 验证变更文件数量：1 added + 2 modified + 1 removed = 4
	expectedFilesCount := 4
	if result["changed_files"] != expectedFilesCount {
		t.Errorf("expected changed_files count %d, got %v", expectedFilesCount, result["changed_files"])
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

	if event.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", event.Branch)
	}

	if event.CommitSHA == nil || *event.CommitSHA != "sha789" {
		t.Errorf("expected commit_sha 'sha789', got '%v'", event.CommitSHA)
	}
}

// TestPushHandler_Handle_RefsWithoutPrefix 测试不带 refs/heads/ 前缀的分支
func TestPushHandler_Handle_RefsWithoutPrefix(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "test/repo",
		},
		"ref": "main", // 没有 refs/heads/ 前缀
		"head_commit": map[string]interface{}{
			"id": "sha123",
		},
		"pusher": map[string]interface{}{
			"name": "user",
		},
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["branch"] != "main" {
		t.Errorf("expected branch 'main', got '%v'", result["branch"])
	}
}

// TestPushHandler_Handle_MissingRequiredFields 测试缺少必填字段
func TestPushHandler_Handle_MissingRequiredFields(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	// 缺少 branch
	eventData := map[string]interface{}{
		"event_type": "push",
		"repository": "test/repo",
	}

	result := handler.Handle(eventData)

	if result["status"] != "error" {
		t.Errorf("expected status 'error', got '%v'", result["status"])
	}

	if result["error"] == nil {
		t.Error("expected error message")
	}
}

// TestPushHandler_Handle_EmptyChangedFiles 测试空变更文件列表
func TestPushHandler_Handle_EmptyChangedFiles(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":    "push",
		"repository":    "test/repo",
		"branch":        "main",
		"commit_sha":    "abc",
		"pusher":        "user",
		"changed_files": "",
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["changed_files"] != 0 {
		t.Errorf("expected changed_files count 0, got %v", result["changed_files"])
	}
}

// TestPushHandler_Handle_SingleChangedFile 测试单个变更文件
func TestPushHandler_Handle_SingleChangedFile(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":    "push",
		"repository":    "test/repo",
		"branch":        "main",
		"commit_sha":    "abc",
		"pusher":        "user",
		"changed_files": "single.py",
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["changed_files"] != 1 {
		t.Errorf("expected changed_files count 1, got %v", result["changed_files"])
	}
}

// TestNewPushHandler 测试创建 Push 处理器
func TestNewPushHandler(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	if handler == nil {
		t.Error("expected non-nil handler")
	}

	if handler.storage == nil {
		t.Error("expected storage to be set")
	}
}

// TestPushHandler_Handle_WebhookWithMultipleCommits 测试多次提交的 webhook
func TestPushHandler_Handle_WebhookWithMultipleCommits(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "test/repo",
		},
		"ref": "refs/heads/main",
		"head_commit": map[string]interface{}{
			"id": "sha123",
		},
		"pusher": map[string]interface{}{
			"name": "user",
		},
		"commits": []interface{}{
			map[string]interface{}{
				"added":    []interface{}{"file1.py"},
				"modified": []interface{}{"file2.py"},
				"removed":  []interface{}{"file3.py"},
			},
			map[string]interface{}{
				"added":    []interface{}{"file4.js"},
				"modified": []interface{}{"file5.js"},
			},
		},
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	// 验证变更文件数量：
	// 第一个提交: 1 added + 1 modified + 1 removed = 3
	// 第二个提交: 1 added + 1 modified = 2
	// 总计: 5
	expectedFilesCount := 5
	if result["changed_files"] != expectedFilesCount {
		t.Errorf("expected changed_files count %d, got %v", expectedFilesCount, result["changed_files"])
	}
}

// TestPushHandler_Handle_WebhookWithoutCommits 测试没有 commits 字段的 webhook
func TestPushHandler_Handle_WebhookWithoutCommits(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "test/repo",
		},
		"ref": "refs/heads/main",
		"head_commit": map[string]interface{}{
			"id": "sha123",
		},
		"pusher": map[string]interface{}{
			"name": "user",
		},
		// 没有 commits 字段
	}

	result := handler.Handle(eventData)

	if result["status"] != "processed" {
		t.Errorf("expected status 'processed', got '%v'", result["status"])
	}

	if result["changed_files"] != 0 {
		t.Errorf("expected changed_files count 0, got %v", result["changed_files"])
	}
}

// TestPushHandler_Handle_PreserveEventID 测试事件 ID 保留
func TestPushHandler_Handle_PreserveEventID(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":  "push",
		"repository":  "test/repo",
		"branch":      "main",
		"commit_sha":  "abc",
		"pusher":      "user",
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

// TestPushHandler_Handle_ActionDefaultValue 测试 Action 默认值
func TestPushHandler_Handle_ActionDefaultValue(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":  "push",
		"repository":  "test/repo",
		"branch":      "main",
		"commit_sha":  "abc",
		"pusher":      "user",
	}

	handler.Handle(eventData)

	events, _ := mockStorage.ListEvents()
	if len(events) == 0 {
		t.Fatal("expected 1 event")
	}

	// Push 事件的 action 应该是 "push"
	if events[0].Action == nil || *events[0].Action != "push" {
		t.Errorf("expected action 'push', got '%v'", events[0].Action)
	}
}

// TestPushHandler_Handle_QualityChecksCreated 测试质量检查项创建
func TestPushHandler_Handle_QualityChecksCreated(t *testing.T) {
	mockStorage := storage.NewMockStorage()
	handler := NewPushHandler(mockStorage)

	eventData := map[string]interface{}{
		"event_type":  "push",
		"repository":  "test/repo",
		"branch":      "main",
		"commit_sha":  "abc",
		"pusher":      "user",
	}

	handler.Handle(eventData)

	events, _ := mockStorage.ListEvents()
	if len(events) == 0 {
		t.Fatal("expected 1 event")
	}

	checks, err := mockStorage.ListQualityChecksByEventID(events[0].EventID)
	if err != nil {
		t.Fatalf("ListQualityChecksByEventID failed: %v", err)
	}

	expectedCheckCount := 9
	if len(checks) != expectedCheckCount {
		t.Errorf("expected %d quality checks, got %d", expectedCheckCount, len(checks))
	}

	// 验证所有检查的状态都是 pending
	for i, check := range checks {
		if check.CheckStatus != "pending" {
			t.Errorf("check %d: expected status 'pending', got '%s'", i, check.CheckStatus)
		}
	}
}
