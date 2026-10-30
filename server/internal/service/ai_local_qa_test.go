package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func localQAControlFixture(t *testing.T) (*AILocalQAControl, qa.Proof, *localIdentityFixture, string, config.AILocalQA) {
	t.Helper()
	id := strings.Repeat("a", 32)
	runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: "tm_ai_local_" + id, Listen: "127.0.0.1:18081", PollIntervalMS: 1000, BatchSize: 1}
	m := config.AILocalQAModel{Endpoint: config.AILocalExecutionModel{Name: "本地问答协议", BaseURL: "http://127.0.0.1:19001/v1", Model: "qa-local-test", TimeoutMS: 20000}, FilingNo: "local-eval:filing", ApprovalRef: "local-eval:model", QualityRef: "local-eval:quality", BudgetRef: "local-eval:budget"}
	model, err := provider.NewDirect(m.ProviderConfig(), true)
	if err != nil {
		t.Fatal(err)
	}
	m.ProfileDigest = model.DirectProfileDigest()
	now := time.Now().UTC().Truncate(time.Millisecond)
	file := config.AILocalQA{Version: 1, InstanceID: id, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: now.UnixMilli() - 1000, ExpiresAtMS: now.Add(time.Hour).UnixMilli(), Members: []int64{1}, Subjects: []int64{1}, Models: []config.AILocalQAModel{m, m, m}, AllowedDomains: []string{"qa.example.test"}, Limits: config.AILocalQALimits{MaxSources: 4, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, TimeoutMS: 20000}}
	path := filepath.Join(t.TempDir(), "qa.json")
	raw, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := config.NewAILocalQASource(path, runtime)
	if err != nil {
		t.Fatal(err)
	}
	identity := &localIdentityFixture{}
	control, err := NewAILocalQAControl(source, identity, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := file.QAConfig()
	if err != nil {
		t.Fatal(err)
	}
	p, err := qa.NewPlan(qa.Identity{UserID: 1, ConversationID: 2, MessageSeq: 1}, conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "示例目录是什么？"}, qa.Scope{SubjectType: "theme", SubjectID: 1, SubjectName: "示例题材", ApprovalRef: "local-eval:scope", ValidUntil: now.Add(10 * time.Minute)}, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	return control, p.Proof(), identity, path, file
}
func TestAILocalQAControlIdentityAndExecution(t *testing.T) {
	for _, mode := range []string{"valid", "readonly", "identity", "member", "subject", "type", "models", "metadata", "calls", "tokens", "expired", "beyond_file", "cancelled", "changed_file"} {
		t.Run(mode, func(t *testing.T) {
			c, p, identity, path, file := localQAControlFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "readonly":
				c.submit = false
			case "identity":
				identity.err = qa.ErrUnavailable
			case "member":
				p.Identity.UserID = 2
			case "subject":
				p.SubjectID = 2
			case "type":
				p.SubjectType = "topic"
			case "models":
				p.Models = p.Models[:2]
			case "metadata":
				p.Models[0].ApprovalRef = "local-eval:other"
			case "calls":
				p.MaxCalls = 4
			case "tokens":
				p.MaxTokens++
			case "expired":
				p.ValidUntil = time.Now().Add(-time.Second)
			case "beyond_file":
				p.ValidUntil = time.Now().Add(2 * time.Hour)
			case "cancelled":
				cancel()
			case "changed_file":
				file.Revision = "v2"
				raw, err := json.Marshal(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			models, err := c.ForPlan(ctx, p)
			if (err == nil) != (mode == "valid") {
				t.Fatal("问答调用边界未生效")
			}
			if err == nil {
				for n, model := range models {
					if !model.MatchesDirect(p.Models[n].ProfileDigest, p.Models[n].Name) {
						t.Fatal("实际模型身份不符")
					}
				}
			}
		})
	}
}
func TestAILocalQAControlReadAndRevocation(t *testing.T) {
	c, p, identity, _, file := localQAControlFixture(t)
	ctx := context.Background()
	if err := c.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if id, err := c.InstanceID(ctx); err != nil || id != file.InstanceID {
		t.Fatal("实例绑定不符")
	}
	if _, err := c.ForPolicy(ctx, p.PolicyDigest); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ForPolicy(ctx, strings.Repeat("f", 64)); err == nil {
		t.Fatal("读取了其它策略")
	}
	for _, evidence := range []qa.ReportEvidence{{TaskID: 1, SubjectID: 1, SubjectType: "theme"}, {TaskID: 0, SubjectID: 1, SubjectType: "theme"}, {TaskID: 1, SubjectID: 2, SubjectType: "theme"}, {TaskID: 1, SubjectID: 1, SubjectType: "topic"}} {
		_, err := c.ForReport(ctx, evidence)
		want := evidence.TaskID == 1 && evidence.SubjectID == 1 && evidence.SubjectType == "theme"
		if (err == nil) != want {
			t.Fatal("报告对象边界不符")
		}
	}
	identity.err = qa.ErrUnavailable
	if _, err := c.ForPolicy(ctx, p.PolicyDigest); err == nil {
		t.Fatal("撤回实例后仍读策略")
	}
	if _, err := c.ForReport(ctx, qa.ReportEvidence{TaskID: 1, SubjectID: 1, SubjectType: "theme"}); err == nil {
		t.Fatal("撤回实例后仍读报告配置")
	}
	if _, err := NewAILocalQAControl(nil, identity, true); err == nil {
		t.Fatal("缺来源仍装配")
	}
	if _, err := NewAILocalQAControl(c.source, nil, true); err == nil {
		t.Fatal("缺身份仍装配")
	}
}
