package storage

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github-hub/internal/quality/models"
)

// TestMockStorage_CreateEvent 测试创建事件
func TestMockStorage_CreateEvent(t *testing.T) {
	storage := NewMockStorage()

	event := &models.GitHubEvent{
		EventID:     "test-event-1",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{"test": "data"}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	err := storage.CreateEvent(event)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	if event.ID != 1 {
		t.Errorf("expected ID 1, got %d", event.ID)
	}

	// 验证事件可以被检索
	retrieved, err := storage.GetEvent(event.ID)
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}

	if retrieved.EventID != event.EventID {
		t.Errorf("expected event_id '%s', got '%s'", event.EventID, retrieved.EventID)
	}
}

// TestMockStorage_CreateEventWithChecks 测试创建带质量检查的事件
func TestMockStorage_CreateEventWithChecks(t *testing.T) {
	storage := NewMockStorage()

	checks := models.CreateChecksForEvent("test-event-2")

	event := &models.GitHubEvent{
		EventID:       "test-event-2",
		EventType:     models.EventTypePullRequest,
		EventStatus:   models.EventStatusPending,
		Repository:    "test/repo",
		Branch:        "feature",
		TargetBranch:  stringPtr("main"),
		QualityChecks: checks,
		Payload:       []byte(`{}`),
		CreatedAt:     models.Now(),
		UpdatedAt:     models.Now(),
	}

	err := storage.CreateEvent(event)
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	// 验证质量检查被保存
	retrievedChecks, err := storage.ListQualityChecksByEventID(event.EventID)
	if err != nil {
		t.Fatalf("ListQualityChecksByEventID failed: %v", err)
	}

	expectedCount := 9
	if len(retrievedChecks) != expectedCount {
		t.Errorf("expected %d quality checks, got %d", expectedCount, len(retrievedChecks))
	}
}

// TestMockStorage_GetEvent 测试获取事件
func TestMockStorage_GetEvent(t *testing.T) {
	storage := NewMockStorage()

	// 测试获取不存在的事件
	_, err := storage.GetEvent(999)
	if err == nil {
		t.Error("expected error when getting non-existent event")
	}

	// 创建一个事件然后获取它
	event := &models.GitHubEvent{
		EventID:     "test-event-3",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	storage.CreateEvent(event)

	retrieved, err := storage.GetEvent(event.ID)
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}

	if retrieved.Repository != event.Repository {
		t.Errorf("expected repository '%s', got '%s'", event.Repository, retrieved.Repository)
	}
}

// TestMockStorage_GetEventByEventID 测试通过 event_id 获取事件
func TestMockStorage_GetEventByEventID(t *testing.T) {
	storage := NewMockStorage()

	event := &models.GitHubEvent{
		EventID:     "test-event-4",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	storage.CreateEvent(event)

	retrieved, err := storage.GetEventByEventID(event.EventID)
	if err != nil {
		t.Fatalf("GetEventByEventID failed: %v", err)
	}

	if retrieved.ID != event.ID {
		t.Errorf("expected ID %d, got %d", event.ID, retrieved.ID)
	}
}

// TestMockStorage_ListEvents 测试列出所有事件
func TestMockStorage_ListEvents(t *testing.T) {
	storage := NewMockStorage()

	// 创建多个事件
	for i := 1; i <= 3; i++ {
		event := &models.GitHubEvent{
			EventID:     "test-event-list",
			EventType:   models.EventTypePush,
			EventStatus: models.EventStatusPending,
			Repository:  "test/repo",
			Branch:      "main",
			Payload:     []byte(`{}`),
			CreatedAt:   models.Now(),
			UpdatedAt:   models.Now(),
		}
		storage.CreateEvent(event)
	}

	events, err := storage.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

// TestMockStorage_UpdateEvent 测试更新事件
func TestMockStorage_UpdateEvent(t *testing.T) {
	storage := NewMockStorage()

	event := &models.GitHubEvent{
		EventID:     "test-event-5",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	storage.CreateEvent(event)

	// 更新事件状态
	event.EventStatus = models.EventStatusCompleted
	err := storage.UpdateEvent(event)
	if err != nil {
		t.Fatalf("UpdateEvent failed: %v", err)
	}

	// 验证更新
	retrieved, _ := storage.GetEvent(event.ID)
	if retrieved.EventStatus != models.EventStatusCompleted {
		t.Errorf("expected status '%s', got '%s'", models.EventStatusCompleted, retrieved.EventStatus)
	}
}

// TestMockStorage_UpdateNonExistentEvent 测试更新不存在的事件
func TestMockStorage_UpdateNonExistentEvent(t *testing.T) {
	storage := NewMockStorage()

	event := &models.GitHubEvent{
		ID:          999,
		EventID:     "non-existent",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	err := storage.UpdateEvent(event)
	if err == nil {
		t.Error("expected error when updating non-existent event")
	}
}

// TestMockStorage_DeleteEvent 测试删除事件
func TestMockStorage_DeleteEvent(t *testing.T) {
	storage := NewMockStorage()

	event := &models.GitHubEvent{
		EventID:     "test-event-6",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	storage.CreateEvent(event)

	err := storage.DeleteEvent(event.ID)
	if err != nil {
		t.Fatalf("DeleteEvent failed: %v", err)
	}

	// 验证事件已删除
	_, err = storage.GetEvent(event.ID)
	if err == nil {
		t.Error("expected error when getting deleted event")
	}
}

// TestMockStorage_DeleteNonExistentEvent 测试删除不存在的事件
func TestMockStorage_DeleteNonExistentEvent(t *testing.T) {
	storage := NewMockStorage()

	err := storage.DeleteEvent(999)
	if err == nil {
		t.Error("expected error when deleting non-existent event")
	}
}

// TestMockStorage_DeleteAllEvents 测试删除所有事件
func TestMockStorage_DeleteAllEvents(t *testing.T) {
	storage := NewMockStorage()

	// 创建多个事件
	for i := 1; i <= 3; i++ {
		event := &models.GitHubEvent{
			EventID:     "test-event-delete-all",
			EventType:   models.EventTypePush,
			EventStatus: models.EventStatusPending,
			Repository:  "test/repo",
			Branch:      "main",
			Payload:     []byte(`{}`),
			CreatedAt:   models.Now(),
			UpdatedAt:   models.Now(),
		}
		storage.CreateEvent(event)
	}

	err := storage.DeleteAllEvents()
	if err != nil {
		t.Fatalf("DeleteAllEvents failed: %v", err)
	}

	// 验证所有事件已删除
	events, _ := storage.ListEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// TestMockStorage_QualityCheckOperations 测试质量检查操作
func TestMockStorage_QualityCheckOperations(t *testing.T) {
	storage := NewMockStorage()

	// 创建质量检查
	check := &models.PRQualityCheck{
		GitHubEventID:  "test-event-7",
		CheckType:      models.QualityCheckTypeCompilation,
		CheckStatus:    models.QualityCheckStatusPending,
		Stage:          models.StageTypeBasicCI,
		StageOrder:     1,
		CheckOrder:     1,
		RetryCount:     0,
		CreatedAt:      models.Now(),
		UpdatedAt:      models.Now(),
	}

	err := storage.CreateQualityCheck(check)
	if err != nil {
		t.Fatalf("CreateQualityCheck failed: %v", err)
	}

	// 获取质量检查
	retrieved, err := storage.GetQualityCheck(check.ID)
	if err != nil {
		t.Fatalf("GetQualityCheck failed: %v", err)
	}

	if retrieved.CheckType != models.QualityCheckTypeCompilation {
		t.Errorf("expected check type '%s', got '%s'", models.QualityCheckTypeCompilation, retrieved.CheckType)
	}

	// 更新质量检查
	check.CheckStatus = models.QualityCheckStatusPassed
	err = storage.UpdateQualityCheck(check)
	if err != nil {
		t.Fatalf("UpdateQualityCheck failed: %v", err)
	}

	// 验证更新
	retrieved, _ = storage.GetQualityCheck(check.ID)
	if retrieved.CheckStatus != models.QualityCheckStatusPassed {
		t.Errorf("expected status '%s', got '%s'", models.QualityCheckStatusPassed, retrieved.CheckStatus)
	}
}

// TestMockStorage_CleanupExpired 测试清理过期数据
func TestMockStorage_CleanupExpired(t *testing.T) {
	storage := NewMockStorage()

	// 创建旧事件
	oldEvent := &models.GitHubEvent{
		EventID:     "old-event",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.FromTime(time.Now().Add(-2 * time.Hour)),
		UpdatedAt:   models.FromTime(time.Now().Add(-2 * time.Hour)),
	}
	storage.CreateEvent(oldEvent)

	// 创建新事件
	newEvent := &models.GitHubEvent{
		EventID:     "new-event",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}
	storage.CreateEvent(newEvent)

	// 清理1小时前的数据
	err := storage.CleanupExpired(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupExpired failed: %v", err)
	}

	// 验证旧事件被删除，新事件保留
	events, _ := storage.ListEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event after cleanup, got %d", len(events))
	}

	if len(events) > 0 && events[0].EventID != "new-event" {
		t.Errorf("expected new-event to remain, got %s", events[0].EventID)
	}
}

// TestMockStorage_ErrorHandling 测试错误处理
func TestMockStorage_ErrorHandling(t *testing.T) {
	storage := NewMockStorage()

	testError := errors.New("test error")
	storage.SetCreateError(testError)

	event := &models.GitHubEvent{
		EventID:     "test-event-error",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	err := storage.CreateEvent(event)
	if err != testError {
		t.Errorf("expected test error, got %v", err)
	}

	// 重置错误
	storage.SetCreateError(nil)
	storage.SetGetError(testError)

	storage.CreateEvent(event)
	_, err = storage.GetEvent(event.ID)
	if err != testError {
		t.Errorf("expected test error from GetEvent, got %v", err)
	}
}

// stringPtr 返回字符串指针的辅助函数
func stringPtr(s string) *string {
	return &s
}

// TestMockStorage_ListEventsPaginated 测试分页查询事件
func TestMockStorage_ListEventsPaginated(t *testing.T) {
	storage := NewMockStorage()

	// 创建25个事件用于测试分页
	for i := 1; i <= 25; i++ {
		event := &models.GitHubEvent{
			EventID:     fmt.Sprintf("test-event-paginated-%d", i),
			EventType:   models.EventTypePush,
			EventStatus: models.EventStatusPending,
			Repository:  "test/repo",
			Branch:      "main",
			Payload:     []byte(`{}`),
			CreatedAt:   models.Now(),
			UpdatedAt:   models.Now(),
		}
		storage.CreateEvent(event)
	}

	// 测试第一页（20条）
	events, total, err := storage.ListEventsPaginated(0, 20)
	if err != nil {
		t.Fatalf("ListEventsPaginated failed: %v", err)
	}

	if len(events) != 20 {
		t.Errorf("expected 20 events on first page, got %d", len(events))
	}

	if total != 25 {
		t.Errorf("expected total 25 events, got %d", total)
	}

	// 测试第二页（5条）
	events, total, err = storage.ListEventsPaginated(20, 20)
	if err != nil {
		t.Fatalf("ListEventsPaginated (page 2) failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events on second page, got %d", len(events))
	}

	// 验证顺序（ID应该降序）
	for i := 0; i < len(events)-1; i++ {
		if events[i].ID < events[i+1].ID {
			t.Errorf("events should be in descending order by ID")
		}
	}
}

// TestMockStorage_UpdateEventStatus 测试更新事件状态
func TestMockStorage_UpdateEventStatus(t *testing.T) {
	storage := NewMockStorage()

	event := &models.GitHubEvent{
		EventID:     "test-event-status",
		EventType:   models.EventTypePush,
		EventStatus: models.EventStatusPending,
		Repository:  "test/repo",
		Branch:      "main",
		Payload:     []byte(`{}`),
		CreatedAt:   models.Now(),
		UpdatedAt:   models.Now(),
	}

	storage.CreateEvent(event)

	// 测试只更新状态（不设置processed_at）
	processedAt := models.FromTime(time.Now())
	err := storage.UpdateEventStatus(event.ID, models.EventStatusCompleted, &processedAt)
	if err != nil {
		t.Fatalf("UpdateEventStatus failed: %v", err)
	}

	// 验证更新
	retrieved, _ := storage.GetEvent(event.ID)
	if retrieved.EventStatus != models.EventStatusCompleted {
		t.Errorf("expected status '%s', got '%s'", models.EventStatusCompleted, retrieved.EventStatus)
	}

	if retrieved.ProcessedAt == nil {
		t.Error("expected ProcessedAt to be set")
	}

	// 测试不设置processed_at
	err = storage.UpdateEventStatus(event.ID, models.EventStatusFailed, nil)
	if err != nil {
		t.Fatalf("UpdateEventStatus (without processed_at) failed: %v", err)
	}

	retrieved, _ = storage.GetEvent(event.ID)
	if retrieved.EventStatus != models.EventStatusFailed {
		t.Errorf("expected status '%s', got '%s'", models.EventStatusFailed, retrieved.EventStatus)
	}
}

// TestMockStorage_BatchUpdateQualityChecks 测试批量更新质量检查
func TestMockStorage_BatchUpdateQualityChecks(t *testing.T) {
	storage := NewMockStorage()

	// 创建事件和多个质量检查
	event := &models.GitHubEvent{
		EventID:       "test-event-batch",
		EventType:     models.EventTypePush,
		EventStatus:   models.EventStatusPending,
		Repository:    "test/repo",
		Branch:        "main",
		QualityChecks: models.CreateChecksForEvent("test-event-batch"),
		Payload:       []byte(`{}`),
		CreatedAt:     models.Now(),
		UpdatedAt:     models.Now(),
	}

	storage.CreateEvent(event)

	// 准备更新的质量检查
	checksToUpdate := []models.PRQualityCheck{}
	for i, check := range event.QualityChecks {
		check.CheckStatus = models.QualityCheckStatusPassed
		if i == 0 {
			msg := "Test output"
			check.Output = &msg
		}
		if i == 1 {
			msg := "Test error"
			check.ErrorMessage = &msg
		}
		checksToUpdate = append(checksToUpdate, check)
	}

	// 批量更新
	err := storage.BatchUpdateQualityChecks(checksToUpdate)
	if err != nil {
		t.Fatalf("BatchUpdateQualityChecks failed: %v", err)
	}

	// 验证所有检查都已更新
	for i, checkID := range event.QualityChecks {
		retrieved, err := storage.GetQualityCheck(checkID.ID)
		if err != nil {
			t.Fatalf("GetQualityCheck failed for ID %d: %v", checkID.ID, err)
		}

		if retrieved.CheckStatus != models.QualityCheckStatusPassed {
			t.Errorf("check %d: expected status 'passed', got '%s'", checkID.ID, retrieved.CheckStatus)
		}

		// 验证 output 字段
		if i == 0 {
			if retrieved.Output == nil || *retrieved.Output != "Test output" {
				t.Errorf("check %d: expected output 'Test output', got '%v'", checkID.ID, retrieved.Output)
			}
		}

		// 验证 error_message 字段
		if i == 1 {
			if retrieved.ErrorMessage == nil || *retrieved.ErrorMessage != "Test error" {
				t.Errorf("check %d: expected error_message 'Test error', got '%v'", checkID.ID, retrieved.ErrorMessage)
			}
		}
	}
}

// TestMockStorage_BatchUpdateQualityChecksEmpty 测试批量更新空数组
func TestMockStorage_BatchUpdateQualityChecksEmpty(t *testing.T) {
	storage := NewMockStorage()

	// 测试空数组
	err := storage.BatchUpdateQualityChecks([]models.PRQualityCheck{})
	if err != nil {
		t.Errorf("BatchUpdateQualityChecks with empty array should not return error, got %v", err)
	}
}

// TestMockStorage_GetEventStats 测试获取事件统计
func TestMockStorage_GetEventStats(t *testing.T) {
	storage := NewMockStorage()

	// 创建一些事件（pending 和 completed）
	for i := 1; i <= 10; i++ {
		status := models.EventStatusPending
		if i > 7 {
			status = models.EventStatusCompleted
		}

		event := &models.GitHubEvent{
			EventID:     fmt.Sprintf("test-event-stats-%d", i),
			EventType:   models.EventTypePush,
			EventStatus: status,
			Repository:  "test/repo",
			Branch:      "main",
			Payload:     []byte(`{}`),
			CreatedAt:   models.Now(),
			UpdatedAt:   models.Now(),
		}
		storage.CreateEvent(event)
	}

	// 获取统计
	total, pending, err := storage.GetEventStats()
	if err != nil {
		t.Fatalf("GetEventStats failed: %v", err)
	}

	if total != 10 {
		t.Errorf("expected total 10 events, got %d", total)
	}

	// i=1 到 7 是 pending（7个），i=8,9,10 是 completed（3个）
	if pending != 7 {
		t.Errorf("expected 7 pending events, got %d", pending)
	}
}
