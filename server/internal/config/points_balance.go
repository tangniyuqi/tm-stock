package config

import "github.com/tangniyuqi/tm-stock/server/internal/memberauth"

// LoadPointsBalance 独立开关，不因任务只读或开发付费放行自动开放。
func LoadPointsBalance(read AITaskReadConfig) (bool, error) {
	on, err := feedbackFlag("TM_AI_POINTS_BALANCE_ENABLED")
	if err != nil || on && (!read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return on, nil
}
