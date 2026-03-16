package handlers

import (
	"log"

	"github-hub/internal/quality/models"
	"github-hub/internal/quality/storage"
)

// PRHandler PR事件处理器
type PRHandler struct {
	storage storage.Storage
}

// NewPRHandler 创建新的PR处理器
func NewPRHandler(storage storage.Storage) *PRHandler {
	return &PRHandler{
		storage: storage,
	}
}

// Handle 处理PR事件
func (h *PRHandler) Handle(eventData map[string]interface{}) map[string]interface{} {
	log.Println("Processing PR event")

	// 检测数据格式
	isSimplifiedFormat := false
	if _, ok := eventData["event_type"]; ok {
		isSimplifiedFormat = true
	}

	var repository string
	var prNumber *int
	var prTitle string
	var prState string
	var prAction string
	var sourceBranch, targetBranch string
	var author string
	var changedFilesCount int

	if isSimplifiedFormat {
		// 简化的mock数据格式
		if repo, ok := eventData["repository"].(string); ok {
			repository = repo
		}
		if pn, ok := eventData["pr_number"].(float64); ok {
			pnInt := int(pn)
			prNumber = &pnInt
		}
		if title, ok := eventData["pr_title"].(string); ok {
			prTitle = title
		}
		if state, ok := eventData["pr_state"].(string); ok {
			prState = state
		} else {
			prState = "open"
		}
		if action, ok := eventData["pr_action"].(string); ok {
			prAction = action
		} else {
			prAction = "opened"
		}
		if branch, ok := eventData["source_branch"].(string); ok {
			sourceBranch = branch
		}
		if branch, ok := eventData["target_branch"].(string); ok {
			targetBranch = branch
		}
		if a, ok := eventData["pr_author"].(string); ok {
			author = a
		}
		if changedFiles, ok := eventData["changed_files"].(string); ok {
			if changedFiles != "" {
				// 简单计算文件数量
				count := 1
				for i := 0; i < len(changedFiles); i++ {
					if changedFiles[i] == ',' {
						count++
					}
				}
				changedFilesCount = count
			}
		}

		log.Printf("PR #%v: %s", prNumber, prTitle)
		log.Printf("Repository: %s", repository)
		log.Printf("Action: %s", prAction)
		log.Printf("State: %s", prState)
		log.Printf("From: %s", sourceBranch)
		log.Printf("To: %s", targetBranch)
		log.Printf("Author: %s", author)
		log.Printf("Changed files: %d", changedFilesCount)

	} else {
		// 完整的GitHub webhook格式
		var pr map[string]interface{}
		if p, ok := eventData["pull_request"].(map[string]interface{}); ok {
			pr = p
		} else {
			pr = eventData
		}

		if pn, ok := pr["number"].(float64); ok {
			pnInt := int(pn)
			prNumber = &pnInt
		}
		if title, ok := pr["title"].(string); ok {
			prTitle = title
		}
		if state, ok := pr["state"].(string); ok {
			prState = state
		}
		if action, ok := eventData["action"].(string); ok {
			prAction = action
		} else {
			prAction = "opened"
		}

		// 提取仓库信息
		if repo, ok := eventData["repository"].(map[string]interface{}); ok {
			if fullName, ok := repo["full_name"].(string); ok {
				repository = fullName
			}
		}

		// 提取分支信息
		if head, ok := pr["head"].(map[string]interface{}); ok {
			if ref, ok := head["ref"].(string); ok {
				sourceBranch = ref
			}
			if sha, ok := head["sha"].(string); ok {
				if len(sha) > 7 {
					log.Printf("Head SHA: %s", sha[:7])
				} else {
					log.Printf("Head SHA: %s", sha)
				}
			}
		}
		if base, ok := pr["base"].(map[string]interface{}); ok {
			if ref, ok := base["ref"].(string); ok {
				targetBranch = ref
			}
			if sha, ok := base["sha"].(string); ok {
				if len(sha) > 7 {
					log.Printf("Base SHA: %s", sha[:7])
				} else {
					log.Printf("Base SHA: %s", sha)
				}
			}
		}

		// 提取作者信息
		if user, ok := pr["user"].(map[string]interface{}); ok {
			if login, ok := user["login"].(string); ok {
				author = login
			}
		}

		// 提取PR统计信息
		if commits, ok := pr["commits"].(float64); ok {
			log.Printf("Commits: %v", commits)
		}
		if additions, ok := pr["additions"].(float64); ok {
			log.Printf("Additions: %v", additions)
		}
		if deletions, ok := pr["deletions"].(float64); ok {
			log.Printf("Deletions: %v", deletions)
		}
		if changedFiles, ok := pr["changed_files"].(float64); ok {
			changedFilesCount = int(changedFiles)
		}

		log.Printf("PR #%v: %s", prNumber, prTitle)
		log.Printf("Repository: %s", repository)
		log.Printf("Action: %s", prAction)
		log.Printf("State: %s", prState)
		log.Printf("From: %s", sourceBranch)
		log.Printf("To: %s", targetBranch)
		log.Printf("Author: %s", author)
		log.Printf("Changed files: %d", changedFilesCount)
	}

	// 创建事件
	event, err := models.NewGitHubEvent(eventData, models.EventTypePullRequest)
	if err != nil {
		log.Printf("Error creating event: %v", err)
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	// 为事件创建质量检查项
	event.QualityChecks = models.CreateChecksForEvent(event.EventID)

	// 保存事件到存储
	if err := h.storage.CreateEvent(event); err != nil {
		log.Printf("Error saving event: %v", err)
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	log.Printf("Created event #%d with %d quality checks for PR #%v", event.ID, len(event.QualityChecks), prNumber)

	// 这里可以添加自定义的处理逻辑
	// 例如：
	// 1. 检查PR标题和描述
	// 2. 分析变更文件
	// 3. 触发CI/CD流程
	// 4. 发送通知

	return map[string]interface{}{
		"status":        "processed",
		"repository":    repository,
		"pr_number":     prNumber,
		"pr_title":      prTitle,
		"action":        prAction,
		"state":         prState,
		"head_branch":   sourceBranch,
		"base_branch":   targetBranch,
		"changed_files": changedFilesCount,
	}
}
