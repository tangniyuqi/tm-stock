package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// LoadAITaskCancel 不因只读或付费开发放行而开启写入；开启须能查询当前进度。
func LoadAITaskCancel(read AITaskReadConfig) (bool, error) {
	raw := os.Getenv("TM_AI_TASK_CANCEL_ENABLED")
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, memberauth.ErrInvalidConfig
	}
	if enabled && (!read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return enabled, nil
}
