package storage

import (
	"time"

	"github-hub/internal/quality/models"
)

// Storage 存储接口定义
type Storage interface {
	// Event 操作
	CreateEvent(event *models.GitHubEvent) error
	GetEvent(id int) (*models.GitHubEvent, error)
	GetEventByEventID(eventID string) (*models.GitHubEvent, error)
	ListEvents() ([]*models.GitHubEvent, error)
	ListEventsPaginated(offset, limit int) ([]*models.GitHubEvent, int, error)
	UpdateEvent(event *models.GitHubEvent) error
	UpdateEventStatus(id int, status models.EventStatus, processedAt *models.LocalTime) error
	DeleteEvent(id int) error
	DeleteAllEvents() error

	// QualityCheck 操作
	CreateQualityCheck(check *models.PRQualityCheck) error
	GetQualityCheck(id int) (*models.PRQualityCheck, error)
	ListQualityChecksByEventID(eventID string) ([]models.PRQualityCheck, error)
	UpdateQualityCheck(check *models.PRQualityCheck) error
	BatchUpdateQualityChecks(checks []models.PRQualityCheck) error

	// 清理操作
	CleanupExpired(ttl time.Duration) error

	// 统计操作
	GetEventStats() (total int, pending int, err error)
}
