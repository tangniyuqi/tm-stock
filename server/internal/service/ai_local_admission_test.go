package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type localIdentityFixture struct{ err error }

func (v *localIdentityFixture) Verify(ctx context.Context, _ string) error {
	if v.err != nil {
		return v.err
	}
	return ctx.Err()
}

func localAdmissionFixture(t *testing.T) (*AILocalAdmissionControl, *reportScript, *localIdentityFixture, string, config.AILocalPlans) {
	t.Helper()
	_, st, _ := reportFixture(t)
	st.theme = "仅测试:合成题材"
	st.snaps[0].LicenseRef = "local-eval:license"
	snap, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: "evidence.example.test", Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "仅测试:合成资料", LicenseRef: st.snaps[0].LicenseRef}, st.snaps[0].URL, sources.MediaPlain, []byte(st.snaps[0].Body+"合成资料记载第五项事实。合成资料记载第六项事实。"), st.snaps[0].FetchedAt, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	snap.ID = st.snaps[0].ID
	st.snaps[0] = snap
	r := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: strings.Repeat("a", 32), Database: "tm_ai_local_" + strings.Repeat("a", 32), Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
	spec := config.AILocalPlan{AnalysisType: 1, SubjectID: st.v.SubjectID, SubjectName: st.theme, ActorID: 7, Symbol: "000001", AgentProfileDigest: strings.Repeat("b", 64), AgentCalls: 1, AgentTokens: 100, AgentPages: 0, ModelName: "仅测试:模型", ModelProfileDigest: strings.Repeat("c", 64), FilingNo: "仅测试:公示编号", ModelVersion: "local-eval:v1", MaxInputTokens: 32, SelectionMaxTokens: 2048, CheckMaxTokens: 64, PolicyRef: "local-eval:policy", AllowedDomains: []string{"evidence.example.test"}, ApprovalRef: "local-eval:approval", QualityRef: "local-eval:quality", HardBudgetRef: "local-eval:hard-budget", PriceRef: "local-eval:price", SourceRef: "local-eval:source", PriceVersion: "local-v1", PricePoints: 7, Budget: task.Budget{MaxAttempts: 1, MaxCalls: 6, MaxTokens: 5000, MaxPages: 10, MaxDurationMS: 60000}, Materials: []config.AILocalMaterial{{SnapshotID: snap.ID, Digest: snap.Hash, LicenseRef: snap.LicenseRef}}}
	deep := spec
	deep.AnalysisType = 2
	deep.MinQuotes = 6
	deep.PricePoints = 30
	now := time.Now().UnixMilli()
	file := config.AILocalPlans{Version: 1, InstanceID: r.InstanceID, Revision: "v1", Scope: "synthetic-local-only", IssuedAtMS: now - 1000, ExpiresAtMS: now + 60000, Members: []int64{1, 2}, Plans: []config.AILocalPlan{spec, deep}}
	path := filepath.Join(t.TempDir(), "plans.json")
	writeLocalAdmissionFile(t, path, file)
	source, err := config.NewAILocalPlanSource(path, r)
	if err != nil {
		t.Fatal(err)
	}
	id := &localIdentityFixture{}
	c, err := NewAILocalAdmissionControl(source, id, st)
	if err != nil {
		t.Fatal(err)
	}
	return c, st, id, path, file
}
func writeLocalAdmissionFile(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAILocalAdmissionBindsBothPlansAndScopes(t *testing.T) {
	c, st, _, _, _ := localAdmissionFixture(t)
	ctx := context.Background()
	if err := c.Ready(ctx); err != nil {
		t.Fatal("本地轻深计划未通过", err)
	}
	if err := c.ReadyDeep(ctx); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []int{1, 2} {
		scope := task.AdmissionScope{UserID: 1, AnalysisType: kind, SubjectType: task.Theme, SubjectID: st.v.SubjectID}
		p, err := c.Plan(ctx, scope)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Recheck(ctx, p); err != nil {
			t.Fatal(err)
		}
		v := st.v
		v.AnalysisType = kind
		v.Budget = p.Budget
		v.PricePoints = p.Price.Points
		v.PriceVersion = p.Price.Version
		plan, err := c.ExecutionPlan(ctx, v)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := plan.AdmissionDigest()
		if err != nil || digest != p.ExecutionDigest || plan.Agent.TaskID != v.ID || plan.Agent.UserID != v.UserID || plan.Agent.TaskNo != v.No {
			t.Fatal("准入与执行未共用同一身份", err)
		}
		p.Materials[0].Digest = strings.Repeat("f", 64)
		if err := c.Recheck(ctx, p); !errors.Is(err, points.ErrIdempotency) {
			t.Fatal("变更快照身份未拒绝", err)
		}
	}
	for _, scope := range []task.AdmissionScope{{UserID: 3, AnalysisType: 1, SubjectType: task.Theme, SubjectID: st.v.SubjectID}, {UserID: 1, AnalysisType: 1, SubjectType: task.Theme, SubjectID: 999}, {UserID: 1, AnalysisType: 1, SubjectType: task.Topic, SubjectID: st.v.SubjectID}} {
		if _, err := c.Plan(ctx, scope); !errors.Is(err, task.ErrAdmissionUnavailable) {
			t.Fatal("未批准主体未拒绝", err)
		}
	}
}

func TestAILocalAdmissionRechecksWithdrawalAndReplacement(t *testing.T) {
	for _, mode := range []string{"标记", "授权撤回", "摘要", "许可引用", "题材名", "文件价格", "文件撤销", "预算不足", "深度价格", "模型身份", "取消"} {
		t.Run(mode, func(t *testing.T) {
			c, st, id, path, file := localAdmissionFixture(t)
			ctx := context.Background()
			if err := c.Ready(ctx); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "标记":
				id.err = points.ErrInvariant
			case "授权撤回":
				st.evidenceError = sources.ErrUnauthorized
			case "摘要":
				st.snaps[0].Hash = strings.Repeat("f", 64)
			case "许可引用":
				st.snaps[0].LicenseRef = "local-eval:other-license"
			case "题材名":
				st.theme = "仅测试:其它题材"
			case "文件价格":
				file.Plans[0].PricePoints++
				writeLocalAdmissionFile(t, path, file)
			case "文件撤销":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "预算不足":
				file.Plans[0].Budget.MaxTokens = 10
				writeLocalAdmissionFile(t, path, file)
				source, err := config.NewAILocalPlanSource(path, &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: file.InstanceID, Database: "tm_ai_local_" + file.InstanceID, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10})
				if err != nil {
					t.Fatal(err)
				}
				c.source = source
			case "深度价格":
				file.Plans[1].PricePoints = 7
				writeLocalAdmissionFile(t, path, file)
				source, err := config.NewAILocalPlanSource(path, &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: file.InstanceID, Database: "tm_ai_local_" + file.InstanceID, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10})
				if err != nil {
					t.Fatal(err)
				}
				c.source = source
			case "模型身份":
				file.Plans[0].ModelProfileDigest = "invalid"
				writeLocalAdmissionFile(t, path, file)
				source, err := config.NewAILocalPlanSource(path, &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: file.InstanceID, Database: "tm_ai_local_" + file.InstanceID, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10})
				if err != nil {
					t.Fatal(err)
				}
				c.source = source
			case "取消":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			if err := c.Ready(ctx); err == nil {
				t.Fatal("撤回/变更边界未拒绝")
			}
		})
	}
}
