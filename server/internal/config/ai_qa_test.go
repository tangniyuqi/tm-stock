package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"strings"
	"testing"
)

func TestQAConfigClosedAndDependencies(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, c := range []struct {
		read, write string
		conv        bool
		cfg         AITaskReadConfig
		bad         bool
	}{{"", "", false, AITaskReadConfig{}, false}, {"false", "false", false, AITaskReadConfig{}, false}, {"true", "false", true, good, false}, {"true", "true", true, good, false}, {"false", "true", true, good, true}, {"true", "", false, good, true}, {"true", "", true, AITaskReadConfig{}, true}, {"yes", "", true, good, true}, {"", "yes", true, good, true}} {
		t.Setenv("TM_AI_QA_READ_ENABLED", c.read)
		t.Setenv("TM_AI_QA_SUBMIT_ENABLED", c.write)
		t.Setenv("TM_AI_REPORT_QA_SUBMIT_ENABLED", "")
		t.Setenv("TM_DEV_GRANT_PAID", "true")
		_, err := LoadAIQA(c.cfg, c.conv)
		if (err != nil) != c.bad {
			t.Fatal("问答开关依赖未拒绝", c.read, c.write, err)
		}
	}
}
func TestQAConfigReportSubmitRequiresBaseSubmit(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, c := range []struct {
		read, submit, report string
		bad                  bool
	}{{"true", "false", "true", true}, {"true", "true", "true", false}, {"true", "true", "false", false}, {"false", "false", "true", true}} {
		t.Setenv("TM_AI_QA_READ_ENABLED", c.read)
		t.Setenv("TM_AI_QA_SUBMIT_ENABLED", c.submit)
		t.Setenv("TM_AI_REPORT_QA_SUBMIT_ENABLED", c.report)
		t.Setenv("TM_DEV_GRANT_PAID", "true")
		_, err := LoadAIQA(good, true)
		if (err != nil) != c.bad {
			t.Fatal("报告追问开关依赖未拒绝", c.read, c.submit, c.report, err)
		}
	}
}
