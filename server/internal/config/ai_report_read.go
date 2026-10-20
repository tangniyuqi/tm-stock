package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// LoadAIReportRead 只读取显式开关，不把开发付费放行或任务读取当作发布准入。
func LoadAIReportRead(read AITaskReadConfig) (bool, error) {
	raw := os.Getenv("TM_AI_REPORT_READ_ENABLED")
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
