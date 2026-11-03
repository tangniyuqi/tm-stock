package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func localQAMaterialsFixture(t *testing.T) (AILocalQA, *AILocalRuntime) {
	t.Helper()
	c, runtime := localQAFileFixture(t)
	c.Version, c.Scope = 2, "official-materials-development-only"
	c.MaterialUse = &AILocalQAMaterialUse{DatasetDigest: strings.Repeat("a", 64), ApprovalRef: "local-eval:official-development", Purpose: "bounded-material-qa", Bindings: []AILocalQAMaterialBinding{{SubjectID: 1, ScopeApproval: "local-eval:scope", Evidence: []qa.EvidenceProof{{ID: 9, Hash: strings.Repeat("b", 64)}}}}}
	for n := range c.Models {
		c.Models[n].JSONSchemaFallback, c.Models[n].ReasoningEffort = true, "none"
		client, err := provider.NewDirect(c.Models[n].ProviderConfig(), true)
		if err != nil {
			t.Fatal(err)
		}
		c.Models[n].ProfileDigest = client.DirectProfileDigest()
	}
	return c, runtime
}

func TestAILocalQAMaterialConfiguration(t *testing.T) {
	for _, mode := range []string{"valid", "old_version", "old_scope", "missing_use", "dataset", "approval", "purpose", "empty_binding", "subject", "scope_approval", "empty_evidence", "too_many", "snapshot_id", "large_id", "snapshot_hash", "duplicate", "unsorted", "reasoning", "profile"} {
		t.Run(mode, func(t *testing.T) {
			c, runtime := localQAMaterialsFixture(t)
			u := c.MaterialUse
			switch mode {
			case "old_version":
				c.Version = 1
			case "old_scope":
				c.Scope = "synthetic-loopback-only"
			case "missing_use":
				c.MaterialUse = nil
			case "dataset":
				u.DatasetDigest = "unknown"
			case "approval":
				u.ApprovalRef = "production-approved"
			case "purpose":
				u.Purpose = "report-generation"
			case "empty_binding":
				u.Bindings = nil
			case "subject":
				u.Bindings[0].SubjectID = 2
			case "scope_approval":
				u.Bindings[0].ScopeApproval = ""
			case "empty_evidence":
				u.Bindings[0].Evidence = nil
			case "too_many":
				u.Bindings[0].Evidence = make([]qa.EvidenceProof, 9)
			case "snapshot_id":
				u.Bindings[0].Evidence[0].ID = 0
			case "large_id":
				u.Bindings[0].Evidence[0].ID = 9007199254740992
			case "snapshot_hash":
				u.Bindings[0].Evidence[0].Hash = strings.Repeat("B", 64)
			case "duplicate":
				u.Bindings[0].Evidence = append(u.Bindings[0].Evidence, u.Bindings[0].Evidence[0])
			case "unsorted":
				u.Bindings[0].Evidence = append(u.Bindings[0].Evidence, qa.EvidenceProof{ID: 8, Hash: strings.Repeat("b", 64)})
			case "reasoning":
				c.Models[0].ReasoningEffort = "arbitrary"
			case "profile":
				c.Models[0].JSONSchemaFallback = false
			}
			path := filepath.Join(t.TempDir(), "qa.json")
			writeLocalQA(t, path, c)
			_, err := NewAILocalQASource(path, runtime)
			if (err == nil) != (mode == "valid") {
				t.Fatal("正式资料开发配置边界不符")
			}
		})
	}
}

func TestAILocalQAMaterialStrictShape(t *testing.T) {
	c, runtime := localQAMaterialsFixture(t)
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	useRaw, err := json.Marshal(c.MaterialUse)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"null", "unknown_use", "unknown_binding", "unknown_evidence", "alias", "missing", "duplicate", "v1_false_flag", "v1_empty_reasoning"} {
		t.Run(mode, func(t *testing.T) {
			bad := string(raw)
			switch mode {
			case "null":
				bad = strings.Replace(bad, string(useRaw), `null`, 1)
			case "unknown_use":
				bad = strings.Replace(bad, `"purpose":`, `"api_key":"forbidden_value","purpose":`, 1)
			case "unknown_binding":
				bad = strings.Replace(bad, `"subject_id":1`, `"subject_id":1,"unexpected":false`, 1)
			case "unknown_evidence":
				bad = strings.Replace(bad, `"ID":9`, `"ID":9,"unexpected":false`, 1)
			case "alias":
				bad = strings.Replace(bad, `"ID":9`, `"id":9`, 1)
			case "missing":
				bad = strings.Replace(bad, `"subject_id":1,`, ``, 1)
			case "duplicate":
				bad = strings.Replace(bad, `"subject_id":1`, `"subject_id":1,"subject_id":1`, 1)
			default:
				old, _ := localQAFileFixture(t)
				b, err := json.Marshal(old)
				if err != nil {
					t.Fatal(err)
				}
				field := `"json_schema_fallback":false,`
				if mode == "v1_empty_reasoning" {
					field = `"reasoning_effort":"",`
				}
				bad = strings.Replace(string(b), `"endpoint":`, field+`"endpoint":`, 1)
			}
			path := filepath.Join(t.TempDir(), "qa.json")
			if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewAILocalQASource(path, runtime); err == nil || strings.Contains(err.Error(), "forbidden_value") {
				t.Fatal("嵌套闭集配置未拒绝或泄漏内容")
			}
		})
	}
}
