package models

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// GitHubEvent GitHub事件模型
type GitHubEvent struct {
	ID           int            `json:"id"`
	EventID      string         `json:"event_id"`
	EventType    EventType      `json:"event_type"`
	EventStatus  EventStatus    `json:"event_status"`
	Repository   string         `json:"repository"`
	Branch       string         `json:"branch"`
	TargetBranch *string        `json:"target_branch,omitempty"`
	CommitSHA    *string        `json:"commit_sha,omitempty"`
	PRNumber     *int           `json:"pr_number,omitempty"`
	Action       *string        `json:"action,omitempty"`
	Pusher       *string        `json:"pusher,omitempty"`
	Author       *string        `json:"author,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	QualityChecks []PRQualityCheck `json:"quality_checks,omitempty"`
	CreatedAt    LocalTime      `json:"created_at"`
	UpdatedAt    LocalTime      `json:"updated_at"`
	ProcessedAt  *LocalTime     `json:"processed_at,omitempty"`
}

// PRQualityCheck PR质量检查模型
type PRQualityCheck struct {
	ID            int                `json:"id"`
	GitHubEventID string             `json:"github_event_id"`
	CheckType     QualityCheckType   `json:"check_type"`
	CheckStatus   QualityCheckStatus `json:"check_status"`
	Stage         StageType          `json:"stage"`
	StageOrder    int                `json:"stage_order"`
	CheckOrder    int                `json:"check_order"`
	StartedAt     *LocalTime         `json:"started_at,omitempty"`
	CompletedAt   *LocalTime         `json:"completed_at,omitempty"`
	DurationSeconds *float64         `json:"duration_seconds,omitempty"`
	ErrorMessage  *string            `json:"error_message,omitempty"`
	Output        *string            `json:"output,omitempty"`
	RetryCount    int                `json:"retry_count"`
	CreatedAt     LocalTime          `json:"created_at"`
	UpdatedAt     LocalTime          `json:"updated_at"`
}

// NewGitHubEvent 创建新的GitHub事件
func NewGitHubEvent(eventData interface{}, eventType EventType) (*GitHubEvent, error) {
	// 检测数据格式
	var isSimplifiedFormat bool
	var repository, branch string
	var targetBranch, commitSHA, action, pusher, author *string
	var prNumber *int

	// 尝试将eventData转换为map
	eventMap, ok := eventData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid event data format")
	}

	// 检查是否为简化格式
	if _, ok := eventMap["event_type"]; ok {
		isSimplifiedFormat = true
	}

	if isSimplifiedFormat {
		// 简化格式处理
		if repo, ok := eventMap["repository"].(string); ok {
			repository = repo
		}
		if p, ok := eventMap["pusher"].(string); ok {
			pusher = &p
		}
		if a, ok := eventMap["pr_author"].(string); ok {
			author = &a
		}

		if eventType == EventTypePush {
			if b, ok := eventMap["branch"].(string); ok {
				branch = b
			}
			if sha, ok := eventMap["commit_sha"].(string); ok {
				commitSHA = &sha
			}
			actionVal := "push"
			action = &actionVal
		} else if eventType == EventTypePullRequest {
			if b, ok := eventMap["source_branch"].(string); ok {
				branch = b
			}
			if tb, ok := eventMap["target_branch"].(string); ok {
				targetBranch = &tb
			}
			if sha, ok := eventMap["commit_sha"].(string); ok {
				commitSHA = &sha
			}
			if pn, ok := eventMap["pr_number"].(float64); ok {
				pnInt := int(pn)
				prNumber = &pnInt
			}
			if prAction, ok := eventMap["pr_action"].(string); ok {
				action = &prAction
			} else {
				actionVal := "opened"
				action = &actionVal
			}
			if a, ok := eventMap["pr_author"].(string); ok {
				author = &a
			}
		}
	} else {
		// GitHub webhook格式处理
		if eventType == EventTypePush {
			if repo, ok := eventMap["repository"].(map[string]interface{}); ok {
				if fullName, ok := repo["full_name"].(string); ok {
					repository = fullName
				}
			}
			if ref, ok := eventMap["ref"].(string); ok {
				// 移除 refs/heads/ 前缀
				if len(ref) > 11 && ref[:11] == "refs/heads/" {
					branch = ref[11:]
				} else {
					branch = ref
				}
			}
			if headCommit, ok := eventMap["head_commit"].(map[string]interface{}); ok {
				if sha, ok := headCommit["id"].(string); ok {
					commitSHA = &sha
				}
			}
			if p, ok := eventMap["pusher"].(map[string]interface{}); ok {
				if name, ok := p["name"].(string); ok {
					pusher = &name
				}
			}
			actionVal := "push"
			action = &actionVal
		} else if eventType == EventTypePullRequest {
			if repo, ok := eventMap["repository"].(map[string]interface{}); ok {
				if fullName, ok := repo["full_name"].(string); ok {
					repository = fullName
				}
			}
			
			var pr map[string]interface{}
			if p, ok := eventMap["pull_request"].(map[string]interface{}); ok {
				pr = p
			} else {
				pr = eventMap
			}

			if head, ok := pr["head"].(map[string]interface{}); ok {
				if ref, ok := head["ref"].(string); ok {
					branch = ref
				}
				if sha, ok := head["sha"].(string); ok {
					commitSHA = &sha
				}
			}
			if base, ok := pr["base"].(map[string]interface{}); ok {
				if ref, ok := base["ref"].(string); ok {
					targetBranch = &ref
				}
			}
			if pn, ok := pr["number"].(float64); ok {
				pnInt := int(pn)
				prNumber = &pnInt
			}
			if a, ok := eventMap["action"].(string); ok {
				action = &a
			} else {
				actionVal := "opened"
				action = &actionVal
			}
			if user, ok := pr["user"].(map[string]interface{}); ok {
				if login, ok := user["login"].(string); ok {
					author = &login
				}
			}
		}
	}

	if repository == "" || branch == "" {
		return nil, fmt.Errorf("missing required fields: repository or branch")
	}

	// 生成EventID
	eventID := uuid.New().String()[:16]

	// 序列化payload
	payloadBytes, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	now := Now()

	return &GitHubEvent{
		ID:           0, // 将由存储层分配
		EventID:      eventID,
		EventType:    eventType,
		EventStatus:  EventStatusPending,
		Repository:   repository,
		Branch:       branch,
		TargetBranch: targetBranch,
		CommitSHA:    commitSHA,
		PRNumber:     prNumber,
		Action:       action,
		Pusher:       pusher,
		Author:       author,
		Payload:      payloadBytes,
		QualityChecks: []PRQualityCheck{},
		CreatedAt:    now,
		UpdatedAt:    now,
		ProcessedAt:  nil,
	}, nil
}

// CreateChecksForEvent 为事件创建所有质量检查项
func CreateChecksForEvent(githubEventID string) []PRQualityCheck {
	checks := []PRQualityCheck{}
	now := Now()

	// 基础CI流水线阶段
	basicCIChecks := []struct {
		CheckType QualityCheckType
		Order     int
	}{
		{QualityCheckTypeCompilation, 1},
		{QualityCheckTypeCodeLint, 2},
		{QualityCheckTypeSecurityScan, 3},
		{QualityCheckTypeUnitTest, 4},
	}

	for _, check := range basicCIChecks {
		checks = append(checks, PRQualityCheck{
			ID:            0, // 将由存储层分配
			GitHubEventID: githubEventID,
			CheckType:     check.CheckType,
			CheckStatus:   QualityCheckStatusPending,
			Stage:         StageTypeBasicCI,
			StageOrder:    1,
			CheckOrder:    check.Order,
			StartedAt:     nil,
			CompletedAt:   nil,
			DurationSeconds: nil,
			ErrorMessage:  nil,
			Output:        nil,
			RetryCount:    0,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	// 部署阶段
	checks = append(checks, PRQualityCheck{
		ID:            0,
		GitHubEventID: githubEventID,
		CheckType:     QualityCheckTypeDeployment,
		CheckStatus:   QualityCheckStatusPending,
		Stage:         StageTypeDeployment,
		StageOrder:    2,
		CheckOrder:    1,
		StartedAt:     nil,
		CompletedAt:   nil,
		DurationSeconds: nil,
		ErrorMessage:  nil,
		Output:        nil,
		RetryCount:    0,
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	// 专项测试流水线阶段
	specializedChecks := []struct {
		CheckType QualityCheckType
		Order     int
	}{
		{QualityCheckTypeApiTest, 1},
		{QualityCheckTypeModuleE2E, 2},
		{QualityCheckTypeAgentE2E, 3},
		{QualityCheckTypeAiE2E, 4},
	}

	for _, check := range specializedChecks {
		checks = append(checks, PRQualityCheck{
			ID:            0,
			GitHubEventID: githubEventID,
			CheckType:     check.CheckType,
			CheckStatus:   QualityCheckStatusPending,
			Stage:         StageTypeSpecializedTests,
			StageOrder:    3,
			CheckOrder:    check.Order,
			StartedAt:     nil,
			CompletedAt:   nil,
			DurationSeconds: nil,
			ErrorMessage:  nil,
			Output:        nil,
			RetryCount:    0,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	return checks
}

// ShouldProcessPushEvent 判断是否应该处理push事件
// 支持GitHub webhook格式和简化格式
func ShouldProcessPushEvent(eventData map[string]interface{}) bool {
	var branch string

	// 尝试从简化格式获取分支 (GitHub Actions格式)
	if b, ok := eventData["branch"].(string); ok {
		branch = b
	} else if ref, ok := eventData["ref"].(string); ok {
		// GitHub webhook格式: "refs/heads/main"
		if len(ref) > 11 && ref[:11] == "refs/heads/" {
			branch = ref[11:]
		} else {
			branch = ref
		}
	}

	// 处理main分支的push事件
	return branch == "main"
}

// ShouldProcessPREvent 判断是否应该处理PR事件
// 支持GitHub webhook格式和简化格式
func ShouldProcessPREvent(eventData map[string]interface{}) bool {
	var headBranch, baseBranch string

	// 尝试从简化格式获取分支 (GitHub Actions格式)
	if sourceBranch, ok := eventData["source_branch"].(string); ok {
		headBranch = sourceBranch
	}
	if targetBranch, ok := eventData["target_branch"].(string); ok {
		baseBranch = targetBranch
	}

	// 如果简化格式没有找到，尝试GitHub webhook格式
	if headBranch == "" || baseBranch == "" {
		var pr map[string]interface{}
		if p, ok := eventData["pull_request"].(map[string]interface{}); ok {
			pr = p
		} else {
			pr = eventData
		}

		if headBranch == "" {
			if head, ok := pr["head"].(map[string]interface{}); ok {
				if ref, ok := head["ref"].(string); ok {
					headBranch = ref
				}
			}
		}

		if baseBranch == "" {
			if base, ok := pr["base"].(map[string]interface{}); ok {
				if ref, ok := base["ref"].(string); ok {
					baseBranch = ref
				}
			}
		}
	}

	// 只处理非main分支合入main分支的PR
	return headBranch != "main" && baseBranch == "main"
}
