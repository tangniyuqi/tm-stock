package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

func TestAILocalPlansAgentInputRequirement(t *testing.T) {
	for _, mode := range []string{"legacy", "required", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			runtime, file := localPlansFixture()
			if mode == "required" {
				file.Plans[0].AgentInputAudit = agents.InputAuditVersion
			}
			if mode == "unknown" {
				file.Plans[0].AgentInputAudit = "unknown"
			}
			path := filepath.Join(t.TempDir(), "plans.json")
			if err := os.WriteFile(path, mustLocalJSON(t, file), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := NewAILocalPlanSource(path, runtime)
			if (err == nil) != (mode != "unknown") {
				t.Fatal("输入审计配置要求边界错误")
			}
		})
	}
}
