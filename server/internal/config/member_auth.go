package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type AITaskReadConfig struct {
	Enabled bool
	Auth    memberauth.Config
}

func (AITaskReadConfig) String() string     { return "AI 任务只读配置（密钥已隐藏）" }
func (c AITaskReadConfig) GoString() string { return c.String() }

// LoadAITaskRead 独立于外部 Agent、L0 与付费内容开关；关闭时不要求会员密钥。
func LoadAITaskRead() (AITaskReadConfig, error) {
	var c AITaskReadConfig
	if raw := os.Getenv("TM_AI_TASK_READ_ENABLED"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return c, memberauth.ErrInvalidConfig
		}
		c.Enabled = enabled
	}
	if !c.Enabled {
		return c, nil
	}
	c.Auth = memberauth.Config{SigningKey: os.Getenv("TM_JWT_SIGNING_KEY"), PreviousKey: os.Getenv("TM_JWT_SIGNING_KEY_PREVIOUS"), Issuer: os.Getenv("TM_JWT_ISSUER"), Audience: os.Getenv("TM_JWT_AUDIENCE")}
	if err := c.Auth.Validate(); err != nil {
		return AITaskReadConfig{}, err
	}
	return c, nil
}
