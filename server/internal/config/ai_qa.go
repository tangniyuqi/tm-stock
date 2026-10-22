package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"os"
	"strconv"
)

type AIQAConfig struct{ Read, Submit, ReportSubmit bool }

func LoadAIQA(read AITaskReadConfig, conversations bool) (AIQAConfig, error) {
	var c AIQAConfig
	for _, v := range []struct {
		name   string
		target *bool
	}{{"TM_AI_QA_READ_ENABLED", &c.Read}, {"TM_AI_QA_SUBMIT_ENABLED", &c.Submit}, {"TM_AI_REPORT_QA_SUBMIT_ENABLED", &c.ReportSubmit}} {
		raw := os.Getenv(v.name)
		if raw != "" {
			b, err := strconv.ParseBool(raw)
			if err != nil {
				return AIQAConfig{}, memberauth.ErrInvalidConfig
			}
			*v.target = b
		}
	}
	if c.Submit && !c.Read || c.ReportSubmit && !c.Submit || c.Read && (!conversations || !read.Enabled || read.Auth.Validate() != nil) {
		return AIQAConfig{}, memberauth.ErrInvalidConfig
	}
	return c, nil
}
