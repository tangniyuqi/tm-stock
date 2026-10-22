package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// 关闭原任务创建后仍可核查和恢复关联，但不能隐式打开创建权限。
func LoadAIConversationTasks(read AITaskReadConfig, conversations bool) (bool, error) {
	raw := os.Getenv("TM_AI_CONVERSATION_TASKS_ENABLED")
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil || enabled && (!conversations || !read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return enabled, nil
}
