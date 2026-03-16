package handlers

import (
	"log"

	"github-hub/internal/quality/models"
	"github-hub/internal/quality/storage"
)

// PushHandler Push事件处理器
type PushHandler struct {
	storage storage.Storage
}

// NewPushHandler 创建新的Push处理器
func NewPushHandler(storage storage.Storage) *PushHandler {
	return &PushHandler{
		storage: storage,
	}
}

// Handle 处理Push事件
func (h *PushHandler) Handle(eventData map[string]interface{}) map[string]interface{} {
	log.Println("Processing Push event")

	// 检测数据格式
	isSimplifiedFormat := false
	if _, ok := eventData["event_type"]; ok {
		isSimplifiedFormat = true
	}

	var repository string
	var branch string
	var commitSHA string
	var pusher string
	var changedFilesCount int

	if isSimplifiedFormat {
		// 简化的mock数据格式
		if repo, ok := eventData["repository"].(string); ok {
			repository = repo
		}
		if b, ok := eventData["branch"].(string); ok {
			branch = b
		}
		if sha, ok := eventData["commit_sha"].(string); ok {
			commitSHA = sha
		}
		if p, ok := eventData["pusher"].(string); ok {
			pusher = p
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

		log.Printf("Repository: %s", repository)
		log.Printf("Branch: %s", branch)
		log.Printf("Commit SHA: %s", commitSHA)
		log.Printf("Pusher: %s", pusher)
		log.Printf("Changed files: %d", changedFilesCount)

	} else {
		// 完整的GitHub webhook格式
		if repo, ok := eventData["repository"].(map[string]interface{}); ok {
			if fullName, ok := repo["full_name"].(string); ok {
				repository = fullName
			}
		}

		if ref, ok := eventData["ref"].(string); ok {
			// 移除 refs/heads/ 前缀
			if len(ref) > 11 && ref[:11] == "refs/heads/" {
				branch = ref[11:]
			} else {
				branch = ref
			}
		}

		if headCommit, ok := eventData["head_commit"].(map[string]interface{}); ok {
			if sha, ok := headCommit["id"].(string); ok {
				commitSHA = sha
			}
			if author, ok := headCommit["author"].(map[string]interface{}); ok {
				if name, ok := author["name"].(string); ok {
					log.Printf("Commit author: %s", name)
				}
			}
			if message, ok := headCommit["message"].(string); ok {
				log.Printf("Commit message: %s", message)
			}
		}

		if p, ok := eventData["pusher"].(map[string]interface{}); ok {
			if name, ok := p["name"].(string); ok {
				pusher = name
			}
		}

		if commits, ok := eventData["commits"].([]interface{}); ok {
			log.Printf("Total commits: %d", len(commits))
			// 计算变更文件总数
			for _, commit := range commits {
				if c, ok := commit.(map[string]interface{}); ok {
					if added, ok := c["added"].([]interface{}); ok {
						changedFilesCount += len(added)
					}
					if modified, ok := c["modified"].([]interface{}); ok {
						changedFilesCount += len(modified)
					}
					if removed, ok := c["removed"].([]interface{}); ok {
						changedFilesCount += len(removed)
					}
				}
			}
		}

		log.Printf("Repository: %s", repository)
		log.Printf("Branch: %s", branch)
		log.Printf("Commit SHA: %s", commitSHA)
		log.Printf("Pusher: %s", pusher)
		log.Printf("Changed files: %d", changedFilesCount)
	}

	// 创建事件
	event, err := models.NewGitHubEvent(eventData, models.EventTypePush)
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

	log.Printf("Created event #%d with %d quality checks for Push event", event.ID, len(event.QualityChecks))

	// 这里可以添加自定义的处理逻辑
	// 例如：
	// 1. 检查提交消息格式
	// 2. 分析变更文件
	// 3. 触发CI/CD流程
	// 4. 发送通知

	return map[string]interface{}{
		"status":        "processed",
		"repository":    repository,
		"branch":        branch,
		"commit_sha":    commitSHA,
		"pusher":        pusher,
		"changed_files": changedFilesCount,
	}
}
