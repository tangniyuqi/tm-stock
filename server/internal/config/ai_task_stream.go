package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// LoadAITaskStream 独立默认关闭；只读或开发付费开关不隐式启用流连接。
func LoadAITaskStream(read AITaskReadConfig) (bool, error) {
	raw := os.Getenv("TM_AI_TASK_STREAM_ENABLED")
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil || enabled && (!read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return enabled, nil
}
