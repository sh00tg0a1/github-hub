package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestNewGitHubEvent_SimplifiedFormat_Push 测试简化格式的 Push 事件创建
func TestNewGitHubEvent_SimplifiedFormat_Push(t *testing.T) {
	eventData := map[string]interface{}{
		"event_type":  "push",
		"repository":  "test/repo",
		"branch":      "main",
		"commit_sha":  "abc123",
		"pusher":      "testuser",
	}

	event, err := NewGitHubEvent(eventData, EventTypePush)
	if err != nil {
		t.Fatalf("NewGitHubEvent failed: %v", err)
	}

	if event.Repository != "test/repo" {
		t.Errorf("expected repository 'test/repo', got '%s'", event.Repository)
	}
	if event.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", event.Branch)
	}
	if event.CommitSHA == nil || *event.CommitSHA != "abc123" {
		t.Errorf("expected commit_sha 'abc123', got '%v'", event.CommitSHA)
	}
	if event.Pusher == nil || *event.Pusher != "testuser" {
		t.Errorf("expected pusher 'testuser', got '%v'", event.Pusher)
	}
	if event.EventType != EventTypePush {
		t.Errorf("expected event type '%s', got '%s'", EventTypePush, event.EventType)
	}
	if event.EventStatus != EventStatusPending {
		t.Errorf("expected status '%s', got '%s'", EventStatusPending, event.EventStatus)
	}
	if event.EventID == "" {
		t.Error("expected non-empty event ID")
	}
}

// TestNewGitHubEvent_SimplifiedFormat_PR 测试简化格式的 PR 事件创建
func TestNewGitHubEvent_SimplifiedFormat_PR(t *testing.T) {
	prNumber := 42
	eventData := map[string]interface{}{
		"event_type":     "pull_request",
		"repository":     "test/repo",
		"source_branch":  "feature",
		"target_branch":  "main",
		"commit_sha":     "def456",
		"pr_number":      float64(42),
		"pr_action":      "opened",
		"pr_author":      "contributor",
	}

	event, err := NewGitHubEvent(eventData, EventTypePullRequest)
	if err != nil {
		t.Fatalf("NewGitHubEvent failed: %v", err)
	}

	if event.Repository != "test/repo" {
		t.Errorf("expected repository 'test/repo', got '%s'", event.Repository)
	}
	if event.Branch != "feature" {
		t.Errorf("expected branch 'feature', got '%s'", event.Branch)
	}
	if event.TargetBranch == nil || *event.TargetBranch != "main" {
		t.Errorf("expected target_branch 'main', got '%v'", event.TargetBranch)
	}
	if event.PRNumber == nil || *event.PRNumber != prNumber {
		t.Errorf("expected pr_number %d, got '%v'", prNumber, event.PRNumber)
	}
	if event.Author == nil || *event.Author != "contributor" {
		t.Errorf("expected author 'contributor', got '%v'", event.Author)
	}
	if event.Action == nil || *event.Action != "opened" {
		t.Errorf("expected action 'opened', got '%v'", event.Action)
	}
}

// TestNewGitHubEvent_WebhookFormat_Push 测试 GitHub webhook 格式的 Push 事件
func TestNewGitHubEvent_WebhookFormat_Push(t *testing.T) {
	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "webhook/repo",
		},
		"ref":       "refs/heads/main",
		"head_commit": map[string]interface{}{
			"id": "sha789",
		},
		"pusher": map[string]interface{}{
			"name": "webhookuser",
		},
	}

	event, err := NewGitHubEvent(eventData, EventTypePush)
	if err != nil {
		t.Fatalf("NewGitHubEvent failed: %v", err)
	}

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

// TestNewGitHubEvent_WebhookFormat_PR 测试 GitHub webhook 格式的 PR 事件
func TestNewGitHubEvent_WebhookFormat_PR(t *testing.T) {
	prNumber := 123
	eventData := map[string]interface{}{
		"repository": map[string]interface{}{
			"full_name": "webhook/repo",
		},
		"action": "synchronized",
		"pull_request": map[string]interface{}{
			"number": float64(123),
			"head": map[string]interface{}{
				"ref": "develop",
				"sha": "prsha123",
			},
			"base": map[string]interface{}{
				"ref": "main",
			},
			"user": map[string]interface{}{
				"login": "prauthor",
			},
		},
	}

	event, err := NewGitHubEvent(eventData, EventTypePullRequest)
	if err != nil {
		t.Fatalf("NewGitHubEvent failed: %v", err)
	}

	if event.Repository != "webhook/repo" {
		t.Errorf("expected repository 'webhook/repo', got '%s'", event.Repository)
	}
	if event.Branch != "develop" {
		t.Errorf("expected branch 'develop', got '%s'", event.Branch)
	}
	if event.TargetBranch == nil || *event.TargetBranch != "main" {
		t.Errorf("expected target_branch 'main', got '%v'", event.TargetBranch)
	}
	if event.PRNumber == nil || *event.PRNumber != prNumber {
		t.Errorf("expected pr_number %d, got '%v'", prNumber, event.PRNumber)
	}
	if event.Author == nil || *event.Author != "prauthor" {
		t.Errorf("expected author 'prauthor', got '%v'", event.Author)
	}
	if event.Action == nil || *event.Action != "synchronized" {
		t.Errorf("expected action 'synchronized', got '%v'", event.Action)
	}
}

// TestNewGitHubEvent_InvalidFormat 测试无效格式
func TestNewGitHubEvent_InvalidFormat(t *testing.T) {
	tests := []struct {
		name      string
		eventData interface{}
		eventType EventType
		wantErr   bool
	}{
		{
			name:      "invalid format - not a map",
			eventData: "invalid string",
			eventType: EventTypePush,
			wantErr:   true,
		},
		{
			name:      "missing repository",
			eventData: map[string]interface{}{
				"branch": "main",
			},
			eventType: EventTypePush,
			wantErr:   true,
		},
		{
			name:      "missing branch",
			eventData: map[string]interface{}{
				"repository": "test/repo",
			},
			eventType: EventTypePush,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitHubEvent(tt.eventData, tt.eventType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGitHubEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewGitHubEvent_PayloadSerialization 测试 payload 序列化
func TestNewGitHubEvent_PayloadSerialization(t *testing.T) {
	eventData := map[string]interface{}{
		"event_type": "push",
		"repository": "test/repo",
		"branch":     "main",
		"custom_field": "test_value",
	}

	event, err := NewGitHubEvent(eventData, EventTypePush)
	if err != nil {
		t.Fatalf("NewGitHubEvent failed: %v", err)
	}

	// 解析 payload 验证序列化正确
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if payload["custom_field"] != "test_value" {
		t.Errorf("expected custom_field 'test_value', got '%v'", payload["custom_field"])
	}
}

// TestCreateChecksForEvent 测试质量检查项创建
func TestCreateChecksForEvent(t *testing.T) {
	eventID := "test-event-123"
	checks := CreateChecksForEvent(eventID)

	// 验证创建了9个检查项（4个基础CI + 1个部署 + 4个专项测试）
	expectedCount := 9
	if len(checks) != expectedCount {
		t.Errorf("expected %d checks, got %d", expectedCount, len(checks))
	}

	// 验证每个检查项的基本属性
	for i, check := range checks {
		if check.GitHubEventID != eventID {
			t.Errorf("check %d: expected event_id '%s', got '%s'", i, eventID, check.GitHubEventID)
		}
		if check.CheckStatus != QualityCheckStatusPending {
			t.Errorf("check %d: expected status '%s', got '%s'", i, QualityCheckStatusPending, check.CheckStatus)
		}
		if check.CreatedAt.IsZero() {
			t.Errorf("check %d: expected non-zero created_at", i)
		}
		if check.UpdatedAt.IsZero() {
			t.Errorf("check %d: expected non-zero updated_at", i)
		}
	}

	// 验证基础CI阶段检查 (4个)
	basicCIChecks := 0
	for _, check := range checks {
		if check.Stage == StageTypeBasicCI {
			basicCIChecks++
			if check.StageOrder != 1 {
				t.Errorf("Basic CI check: expected stage_order 1, got %d", check.StageOrder)
			}
		}
	}
	if basicCIChecks != 4 {
		t.Errorf("expected 4 basic CI checks, got %d", basicCIChecks)
	}

	// 验证部署阶段检查 (1个)
	deploymentChecks := 0
	for _, check := range checks {
		if check.Stage == StageTypeDeployment {
			deploymentChecks++
			if check.StageOrder != 2 {
				t.Errorf("Deployment check: expected stage_order 2, got %d", check.StageOrder)
			}
			if check.CheckType != QualityCheckTypeDeployment {
				t.Errorf("expected check type '%s', got '%s'", QualityCheckTypeDeployment, check.CheckType)
			}
		}
	}
	if deploymentChecks != 1 {
		t.Errorf("expected 1 deployment check, got %d", deploymentChecks)
	}

	// 验证专项测试阶段检查 (4个)
	specializedChecks := 0
	for _, check := range checks {
		if check.Stage == StageTypeSpecializedTests {
			specializedChecks++
			if check.StageOrder != 3 {
				t.Errorf("Specialized test: expected stage_order 3, got %d", check.StageOrder)
			}
		}
	}
	if specializedChecks != 4 {
		t.Errorf("expected 4 specialized test checks, got %d", specializedChecks)
	}

	// 验证检查类型唯一性
	checkTypes := make(map[QualityCheckType]bool)
	for _, check := range checks {
		if checkTypes[check.CheckType] {
			t.Errorf("duplicate check type '%s'", check.CheckType)
		}
		checkTypes[check.CheckType] = true
	}
}

// TestShouldProcessPushEvent 测试 Push 事件过滤
func TestShouldProcessPushEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventData map[string]interface{}
		want      bool
	}{
		{
			name: "main branch push with refs/heads/ prefix",
			eventData: map[string]interface{}{
				"ref": "refs/heads/main",
			},
			want: true,
		},
		{
			name: "main branch push without prefix",
			eventData: map[string]interface{}{
				"ref": "main",
			},
			want: true,
		},
		{
			name: "feature branch push",
			eventData: map[string]interface{}{
				"ref": "refs/heads/feature",
			},
			want: false,
		},
		{
			name: "develop branch push",
			eventData: map[string]interface{}{
				"ref": "refs/heads/develop",
			},
			want: false,
		},
		{
			name:      "missing ref field",
			eventData: map[string]interface{}{},
			want:      false,
		},
		{
			name: "ref is not a string",
			eventData: map[string]interface{}{
				"ref": 123,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldProcessPushEvent(tt.eventData)
			if got != tt.want {
				t.Errorf("ShouldProcessPushEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestShouldProcessPREvent 测试 PR 事件过滤
func TestShouldProcessPREvent(t *testing.T) {
	tests := []struct {
		name      string
		eventData map[string]interface{}
		want      bool
	}{
		{
			name: "feature to main PR",
			eventData: map[string]interface{}{
				"pull_request": map[string]interface{}{
					"head": map[string]interface{}{
						"ref": "feature",
					},
					"base": map[string]interface{}{
						"ref": "main",
					},
				},
			},
			want: true,
		},
		{
			name: "develop to main PR",
			eventData: map[string]interface{}{
				"pull_request": map[string]interface{}{
					"head": map[string]interface{}{
						"ref": "develop",
					},
					"base": map[string]interface{}{
						"ref": "main",
					},
				},
			},
			want: true,
		},
		{
			name: "main to main PR (should skip)",
			eventData: map[string]interface{}{
				"pull_request": map[string]interface{}{
					"head": map[string]interface{}{
						"ref": "main",
					},
					"base": map[string]interface{}{
						"ref": "main",
					},
				},
			},
			want: false,
		},
		{
			name: "feature to develop PR (should skip)",
			eventData: map[string]interface{}{
				"pull_request": map[string]interface{}{
					"head": map[string]interface{}{
						"ref": "feature",
					},
					"base": map[string]interface{}{
						"ref": "develop",
					},
				},
			},
			want: false,
		},
		{
			name: "simplified format - feature to main",
			eventData: map[string]interface{}{
				"head": map[string]interface{}{
					"ref": "feature",
				},
				"base": map[string]interface{}{
					"ref": "main",
				},
			},
			want: true,
		},
		{
			name:      "missing branch info",
			eventData: map[string]interface{}{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldProcessPREvent(tt.eventData)
			if got != tt.want {
				t.Errorf("ShouldProcessPREvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestQualityCheckStatusValues 测试质量检查状态值
func TestQualityCheckStatusValues(t *testing.T) {
	tests := []struct {
		status QualityCheckStatus
		valid  bool
	}{
		{QualityCheckStatusPending, true},
		{QualityCheckStatusRunning, true},
		{QualityCheckStatusPassed, true},
		{QualityCheckStatusFailed, true},
		{QualityCheckStatusSkipped, true},
		{QualityCheckStatusCancelled, true},
		{QualityCheckStatus("invalid"), false},
	}

	for _, tt := range tests {
		_, err := ParseQualityCheckStatus(string(tt.status))
		if tt.valid && err != nil {
			t.Errorf("ParseQualityCheckStatus(%q) unexpectedly returned error: %v", tt.status, err)
		}
	}
}

// TestEventStatusValues 测试事件状态值
func TestEventStatusValues(t *testing.T) {
	validStatuses := []EventStatus{
		EventStatusPending,
		EventStatusProcessing,
		EventStatusCompleted,
		EventStatusFailed,
		EventStatusSkipped,
	}

	for _, status := range validStatuses {
		if string(status) == "" {
			t.Errorf("EventStatus %v has empty string value", status)
		}
	}
}

// TestQualityCheckTypeValues 测试质量检查类型值
func TestQualityCheckTypeValues(t *testing.T) {
	validTypes := []QualityCheckType{
		QualityCheckTypeCompilation,
		QualityCheckTypeCodeLint,
		QualityCheckTypeSecurityScan,
		QualityCheckTypeUnitTest,
		QualityCheckTypeDeployment,
		QualityCheckTypeApiTest,
		QualityCheckTypeModuleE2E,
		QualityCheckTypeAgentE2E,
		QualityCheckTypeAiE2E,
	}

	for _, checkType := range validTypes {
		if string(checkType) == "" {
			t.Errorf("QualityCheckType %v has empty string value", checkType)
		}
	}
}

// TestGitHubEvent_Timestamps 测试时间戳
func TestGitHubEvent_Timestamps(t *testing.T) {
	eventData := map[string]interface{}{
		"event_type": "push",
		"repository": "test/repo",
		"branch":     "main",
	}

	event, err := NewGitHubEvent(eventData, EventTypePush)
	if err != nil {
		t.Fatalf("NewGitHubEvent failed: %v", err)
	}

	now := time.Now()
	if event.CreatedAt.After(now) {
		t.Error("CreatedAt is in the future")
	}
	if event.UpdatedAt.After(now) {
		t.Error("UpdatedAt is in the future")
	}
	if event.ProcessedAt != nil {
		t.Error("ProcessedAt should be nil for new event")
	}
}

// TestPRQualityCheck_DefaultValues 测试 PRQualityCheck 默认值
func TestPRQualityCheck_DefaultValues(t *testing.T) {
	checks := CreateChecksForEvent("test-id")

	for _, check := range checks {
		if check.StartedAt != nil {
			t.Error("StartedAt should be nil for new check")
		}
		if check.CompletedAt != nil {
			t.Error("CompletedAt should be nil for new check")
		}
		if check.DurationSeconds != nil {
			t.Error("DurationSeconds should be nil for new check")
		}
		if check.ErrorMessage != nil {
			t.Error("ErrorMessage should be nil for new check")
		}
		if check.Output != nil {
			t.Error("Output should be nil for new check")
		}
		if check.RetryCount != 0 {
			t.Errorf("RetryCount should be 0, got %d", check.RetryCount)
		}
	}
}
