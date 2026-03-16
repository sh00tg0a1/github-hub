package models

import "fmt"

// EventStatus 事件状态枚举
type EventStatus string

const (
	EventStatusPending    EventStatus = "pending"
	EventStatusProcessing EventStatus = "processing"
	EventStatusCompleted  EventStatus = "completed"
	EventStatusFailed     EventStatus = "failed"
	EventStatusSkipped    EventStatus = "skipped"
	EventStatusCancelled  EventStatus = "cancelled"
)

// EventType 事件类型枚举
type EventType string

const (
	EventTypePush        EventType = "push"
	EventTypePullRequest EventType = "pull_request"
)

// QualityCheckStatus 质量检查状态枚举
type QualityCheckStatus string

const (
	QualityCheckStatusPending   QualityCheckStatus = "pending"
	QualityCheckStatusRunning   QualityCheckStatus = "running"
	QualityCheckStatusPassed    QualityCheckStatus = "passed"
	QualityCheckStatusFailed    QualityCheckStatus = "failed"
	QualityCheckStatusSkipped   QualityCheckStatus = "skipped"
	QualityCheckStatusCancelled QualityCheckStatus = "cancelled"
)

// QualityCheckType 质量检查类型枚举
type QualityCheckType string

const (
	QualityCheckTypeCompilation  QualityCheckType = "compilation"
	QualityCheckTypeCodeLint     QualityCheckType = "code_lint"
	QualityCheckTypeSecurityScan QualityCheckType = "security_scan"
	QualityCheckTypeUnitTest     QualityCheckType = "unit_test"
	QualityCheckTypeDeployment   QualityCheckType = "deployment"
	QualityCheckTypeApiTest      QualityCheckType = "api_test"
	QualityCheckTypeModuleE2E    QualityCheckType = "module_e2e"
	QualityCheckTypeAgentE2E     QualityCheckType = "agent_e2e"
	QualityCheckTypeAiE2E        QualityCheckType = "ai_e2e"
)

// StageType 检查阶段类型
type StageType string

const (
	StageTypeBasicCI          StageType = "basic_ci"
	StageTypeDeployment       StageType = "deployment"
	StageTypeSpecializedTests StageType = "specialized_tests"
)

// ParseQualityCheckStatus 解析质量检查状态字符串
func ParseQualityCheckStatus(status string) (QualityCheckStatus, error) {
	switch QualityCheckStatus(status) {
	case QualityCheckStatusPending, QualityCheckStatusRunning, QualityCheckStatusPassed,
		QualityCheckStatusFailed, QualityCheckStatusSkipped, QualityCheckStatusCancelled:
		return QualityCheckStatus(status), nil
	default:
		return "", fmt.Errorf("invalid quality check status: %s", status)
	}
}

// ParseEventStatus 解析事件状态字符串
func ParseEventStatus(status string) (EventStatus, error) {
	switch EventStatus(status) {
	case EventStatusPending, EventStatusProcessing, EventStatusCompleted,
		EventStatusFailed, EventStatusSkipped:
		return EventStatus(status), nil
	default:
		return "", fmt.Errorf("invalid event status: %s", status)
	}
}
