package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
)

// LoadAIAgents 与既有 L0 开关隔离；没有明确批准配置时不允许外联。
func LoadAIAgents() (tradingcn.Config, error) {
	c := tradingcn.Config{}
	switch os.Getenv("TM_AI_AGENTS_ENABLED") {
	case "", "false":
		return c, nil
	case "true":
		c.Enabled = true
	default:
		return c, fmt.Errorf("TM_AI_AGENTS_ENABLED 只接受 true 或 false")
	}
	switch os.Getenv("TM_AI_AGENTS_LOCAL_EVALUATION") {
	case "", "false":
	case "true":
		c.LocalEvaluation = true
	default:
		return tradingcn.Config{}, fmt.Errorf("TM_AI_AGENTS_LOCAL_EVALUATION 只接受 true 或 false")
	}
	c.BaseURL = os.Getenv("TM_AI_AGENTS_URL")
	c.ApprovedHost = os.Getenv("TM_AI_AGENTS_APPROVED_HOST")
	c.Bearer = os.Getenv("TM_AI_AGENTS_TOKEN")
	c.ApprovalRef = os.Getenv("TM_AI_AGENTS_APPROVAL_REF")
	c.LicenseRef = os.Getenv("TM_AI_AGENTS_LICENSE_REF")
	c.QuickModel = os.Getenv("TM_AI_AGENTS_QUICK_MODEL")
	c.DeepModel = os.Getenv("TM_AI_AGENTS_DEEP_MODEL")
	ms, err := strconv.Atoi(os.Getenv("TM_AI_AGENTS_TIMEOUT_MS"))
	if err != nil || ms < 1000 || ms > 30000 {
		return tradingcn.Config{}, fmt.Errorf("须明确设置 TM_AI_AGENTS_TIMEOUT_MS 为 1000 至 30000 毫秒")
	}
	c.Timeout = time.Duration(ms) * time.Millisecond
	c.MaxSubmissions, err = strconv.Atoi(os.Getenv("TM_AI_AGENTS_MAX_SUBMISSIONS"))
	if err != nil {
		return tradingcn.Config{}, fmt.Errorf("须明确设置 TM_AI_AGENTS_MAX_SUBMISSIONS 为 1 至 1000")
	}
	if err := c.Validate(); err != nil {
		return tradingcn.Config{}, err
	}
	return c, nil
}
