package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"os"
	"strconv"
)

// LoadAIConversations 只开私有元数据/历史，绝不隐式启用问答或收费生成。
func LoadAIConversations(read AITaskReadConfig) (bool, error) {
	raw := os.Getenv("TM_AI_CONVERSATIONS_ENABLED")
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil || enabled && (!read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return enabled, nil
}
