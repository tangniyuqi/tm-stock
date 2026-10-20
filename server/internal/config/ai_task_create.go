package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// LoadAITaskCreate 不因开发付费、报告或Agent开关而启用创建。
func LoadAITaskCreate(read AITaskReadConfig) (bool, error) {
	raw := os.Getenv("TM_AI_TASK_CREATE_ENABLED")
	if raw == "" {
		return false, nil
	}
	on, err := strconv.ParseBool(raw)
	if err != nil || on && (!read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return on, nil
}
