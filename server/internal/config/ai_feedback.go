package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"os"
)

func feedbackFlag(name string) (bool, error) {
	switch os.Getenv(name) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, memberauth.ErrInvalidConfig
	}
}
func LoadAIFeedback(read AITaskReadConfig) (bool, error) {
	on, err := feedbackFlag("TM_AI_FEEDBACK_ENABLED")
	if err != nil || on && (!read.Enabled || read.Auth.Validate() != nil) {
		return false, memberauth.ErrInvalidConfig
	}
	return on, nil
}
func LoadAIFeedbackAdmin() (bool, error) { return feedbackFlag("TM_AI_FEEDBACK_ADMIN_ENABLED") }
