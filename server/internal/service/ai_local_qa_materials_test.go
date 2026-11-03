package service

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestAILocalQAMaterialProofBinding(t *testing.T) {
	for _, mode := range []string{"valid", "readonly", "member", "subject", "type", "scope", "empty", "extra", "snapshot", "hash", "expired", "beyond_file", "identity", "revoked", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			c, p, identity, path, file := localQAControlFixture(t)
			file.Version, file.Scope = 2, "official-materials-development-only"
			p.Evidence = []qa.EvidenceProof{{ID: 9, Hash: strings.Repeat("b", 64)}}
			file.MaterialUse = &config.AILocalQAMaterialUse{DatasetDigest: strings.Repeat("a", 64), ApprovalRef: "local-eval:official-development", Purpose: "bounded-material-qa", Bindings: []config.AILocalQAMaterialBinding{{SubjectID: p.SubjectID, ScopeApproval: p.ScopeApproval, Evidence: append([]qa.EvidenceProof(nil), p.Evidence...)}}}
			write := func() {
				raw, err := json.Marshal(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			write()
			runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: file.InstanceID, Database: "tm_ai_local_" + file.InstanceID, Listen: "127.0.0.1:18081", PollIntervalMS: 1000, BatchSize: 1}
			source, err := config.NewAILocalQASource(path, runtime)
			if err != nil {
				t.Fatal(err)
			}
			c.source = source
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "readonly":
				c.submit = false
			case "member":
				p.Identity.UserID = 2
			case "subject":
				p.SubjectID = 2
			case "type":
				p.SubjectType = "topic"
			case "scope":
				p.ScopeApproval = "local-eval:other"
			case "empty":
				p.Evidence = nil
			case "extra":
				p.Evidence = append(p.Evidence, qa.EvidenceProof{ID: 10, Hash: strings.Repeat("c", 64)})
			case "snapshot":
				p.Evidence[0].ID = 10
			case "hash":
				p.Evidence[0].Hash = strings.Repeat("c", 64)
			case "expired":
				p.ValidUntil = time.Now().Add(-time.Minute)
			case "beyond_file":
				p.ValidUntil = time.UnixMilli(file.ExpiresAtMS + 1)
			case "identity":
				identity.err = qa.ErrUnavailable
			case "revoked":
				file.MaterialUse.ApprovalRef = "local-eval:changed"
				write()
			case "cancelled":
				cancel()
			}
			if err := c.ValidateProof(ctx, p); (err == nil) != (mode == "valid" || mode == "readonly") {
				t.Fatal("正式资料读取未复核开发用途/原证明")
			}
			if mode != "valid" && mode != "readonly" {
				// 缺省Tx不得被访问：服务必须先拒绝失效的本地用途再读取任何资料范围。
				svc := &AIQAService{policies: c}
				if _, err := svc.restore(ctx, nil, qa.Record{Proof: p}, conversation.Message{}); err == nil {
					t.Fatal("答案恢复未先拒绝失效用途")
				}
			}
			if _, err := c.ForPlan(ctx, p); (err == nil) != (mode == "valid") {
				t.Fatal("正式资料执行未绑定原范围")
			}
			if _, err := c.ForReport(ctx, qa.ReportEvidence{TaskID: 1, SubjectType: "theme", SubjectID: 1}); err == nil {
				t.Fatal("借正式资料问答批准执行未配置的报告追问")
			}
		})
	}
}
