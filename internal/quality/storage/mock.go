package storage

import (
	"errors"
	"time"

	"github-hub/internal/quality/models"
)

// MockStorage 模拟存储实现，用于测试
type MockStorage struct {
	events        map[int]*models.GitHubEvent
	eventsByID    map[string]*models.GitHubEvent
	qualityChecks map[int]*models.PRQualityCheck
	nextEventID   int
	nextCheckID   int
	createError   error
	getError      error
}

// NewMockStorage 创建新的模拟存储
func NewMockStorage() *MockStorage {
	return &MockStorage{
		events:        make(map[int]*models.GitHubEvent),
		eventsByID:    make(map[string]*models.GitHubEvent),
		qualityChecks: make(map[int]*models.PRQualityCheck),
		nextEventID:   1,
		nextCheckID:   1,
	}
}

// CreateEvent 创建事件
func (m *MockStorage) CreateEvent(event *models.GitHubEvent) error {
	if m.createError != nil {
		return m.createError
	}

	event.ID = m.nextEventID
	m.nextEventID++

	m.events[event.ID] = event
	m.eventsByID[event.EventID] = event

	// 创建质量检查项
	for i := range event.QualityChecks {
		check := &event.QualityChecks[i]
		check.ID = m.nextCheckID
		m.nextCheckID++
		m.qualityChecks[check.ID] = check
	}

	return nil
}

// GetEvent 获取事件
func (m *MockStorage) GetEvent(id int) (*models.GitHubEvent, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	event, ok := m.events[id]
	if !ok {
		return nil, errors.New("event not found")
	}
	return event, nil
}

// GetEventByEventID 通过 event_id 获取事件
func (m *MockStorage) GetEventByEventID(eventID string) (*models.GitHubEvent, error) {
	for _, event := range m.events {
		if event.EventID == eventID {
			return event, nil
		}
	}
	return nil, errors.New("event not found")
}

// ListEvents 列出所有事件
func (m *MockStorage) ListEvents() ([]*models.GitHubEvent, error) {
	events := make([]*models.GitHubEvent, 0, len(m.events))
	for _, event := range m.events {
		events = append(events, event)
	}
	return events, nil
}

// UpdateEvent 更新事件
func (m *MockStorage) UpdateEvent(event *models.GitHubEvent) error {
	if _, ok := m.events[event.ID]; !ok {
		return errors.New("event not found")
	}
	m.events[event.ID] = event
	m.eventsByID[event.EventID] = event
	return nil
}

// DeleteEvent 删除事件
func (m *MockStorage) DeleteEvent(id int) error {
	event, ok := m.events[id]
	if !ok {
		return errors.New("event not found")
	}

	delete(m.events, id)
	delete(m.eventsByID, event.EventID)
	return nil
}

// DeleteAllEvents 删除所有事件
func (m *MockStorage) DeleteAllEvents() error {
	m.events = make(map[int]*models.GitHubEvent)
	m.eventsByID = make(map[string]*models.GitHubEvent)
	m.qualityChecks = make(map[int]*models.PRQualityCheck)
	return nil
}

// CreateQualityCheck 创建质量检查
func (m *MockStorage) CreateQualityCheck(check *models.PRQualityCheck) error {
	check.ID = m.nextCheckID
	m.nextCheckID++
	m.qualityChecks[check.ID] = check
	return nil
}

// GetQualityCheck 获取质量检查
func (m *MockStorage) GetQualityCheck(id int) (*models.PRQualityCheck, error) {
	check, ok := m.qualityChecks[id]
	if !ok {
		return nil, errors.New("quality check not found")
	}
	return check, nil
}

// ListQualityChecksByEventID 列出事件的所有质量检查
func (m *MockStorage) ListQualityChecksByEventID(eventID string) ([]models.PRQualityCheck, error) {
	var checks []models.PRQualityCheck
	for _, check := range m.qualityChecks {
		if check.GitHubEventID == eventID {
			checks = append(checks, *check)
		}
	}
	return checks, nil
}

// UpdateQualityCheck 更新质量检查
func (m *MockStorage) UpdateQualityCheck(check *models.PRQualityCheck) error {
	if _, ok := m.qualityChecks[check.ID]; !ok {
		return errors.New("quality check not found")
	}
	m.qualityChecks[check.ID] = check
	return nil
}

// CleanupExpired 清理过期数据
func (m *MockStorage) CleanupExpired(ttl time.Duration) error {
	now := time.Now()
	for id, event := range m.events {
		if now.Sub(event.UpdatedAt.ToTime()) > ttl {
			delete(m.events, id)
			delete(m.eventsByID, event.EventID)
		}
	}
	return nil
}

// SetCreateError 设置创建错误（用于测试错误处理）
func (m *MockStorage) SetCreateError(err error) {
	m.createError = err
}

// SetGetError 设置获取错误（用于测试错误处理）
func (m *MockStorage) SetGetError(err error) {
	m.getError = err
}

// ListEventsPaginated 分页查询事件
func (m *MockStorage) ListEventsPaginated(offset, limit int) ([]*models.GitHubEvent, int, error) {
	events := make([]*models.GitHubEvent, 0, len(m.events))
	for _, event := range m.events {
		events = append(events, event)
	}

	// 按 ID 降序排序（使用冒泡排序）
	n := len(events)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if events[j].ID < events[j+1].ID {
				events[j], events[j+1] = events[j+1], events[j]
			}
		}
	}

	total := len(events)

	// 应用分页
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	if start >= end {
		return []*models.GitHubEvent{}, total, nil
	}

	return events[start:end], total, nil
}

// UpdateEventStatus 更新事件状态
func (m *MockStorage) UpdateEventStatus(id int, status models.EventStatus, processedAt *models.LocalTime) error {
	event, ok := m.events[id]
	if !ok {
		return errors.New("event not found")
	}

	event.EventStatus = status
	event.ProcessedAt = processedAt
	event.UpdatedAt = models.Now()

	// 更新 maps 中的引用
	m.eventsByID[event.EventID] = event

	return nil
}

// BatchUpdateQualityChecks 批量更新质量检查
func (m *MockStorage) BatchUpdateQualityChecks(checks []models.PRQualityCheck) error {
	for _, check := range checks {
		if _, ok := m.qualityChecks[check.ID]; !ok {
			return errors.New("quality check not found")
		}
		// 更新副本
		updatedCheck := check
		updatedCheck.UpdatedAt = models.Now()
		m.qualityChecks[check.ID] = &updatedCheck

		// 更新所属事件的 quality_checks
		if event, ok := m.eventsByID[check.GitHubEventID]; ok {
			for i, qc := range event.QualityChecks {
				if qc.ID == check.ID {
					event.QualityChecks[i] = updatedCheck
					break
				}
			}
		}
	}
	return nil
}

// GetEventStats 获取事件统计
func (m *MockStorage) GetEventStats() (total int, pending int, err error) {
	total = len(m.events)
	pending = 0

	for _, event := range m.events {
		if event.EventStatus == models.EventStatusPending {
			pending++
		}
	}

	return total, pending, nil
}
