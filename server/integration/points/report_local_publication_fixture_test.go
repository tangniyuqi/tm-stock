//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type localPublicationFixture struct {
	env       *workflowEnv
	owner     string
	spec      config.AILocalPlan
	file      config.AILocalPlans
	path      string
	raw       []byte
	runtime   *config.AILocalRuntime
	admission *service.AILocalAdmissionControl
	creates   *service.AITaskCreateService
}

// 共用原会员创建与报告测试的准备路径，不创建新的执行/扣费体系。
func newLocalPublicationFixture(t *testing.T, level sql.IsolationLevel, kind int, mode string) *localPublicationFixture {
	t.Helper()
	return newLocalPublicationFixtureWithDuration(t, level, kind, mode, 0)
}

// 浏览器专项可预先声明交互任务预算；不延长任何已创建任务、原自动化夹具或调用超时。
func newLocalPublicationFixtureWithDuration(t *testing.T, level sql.IsolationLevel, kind int, mode string, duration time.Duration) *localPublicationFixture {
	t.Helper()
	e := newWorkflowEnv(t, level, mode)
	reportDDL(t, e.pointsTestEnv, "20261029_create_tm_ai_report_call_audits.sql")
	owner := e.v.LeaseOwner
	if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, owner, e.v.Fence, "internal_failure", 0); err != nil {
		t.Fatal(err)
	}
	grant := pointGrant(time.Now().UTC().Truncate(time.Millisecond), 40)
	grant.Key = "local_extra_grant_key"
	grant.BizID = "local-extra-grant"
	if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
		t.Fatal(err)
	}
	p := e.planner.p
	m := p.Model
	price := int64(7)
	if kind == 2 {
		price = 30
	}
	spec := config.AILocalPlan{AnalysisType: kind, SubjectID: 1, SubjectName: p.ThemeName, ActorID: p.Agent.ActorID, Symbol: p.Agent.Symbol, AgentProfileDigest: p.Agent.ProfileDigest, AgentCalls: p.Agent.Units.Calls, AgentTokens: p.Agent.Units.Tokens, AgentPages: p.Agent.Units.Pages, ModelName: m.Model, ModelProfileDigest: m.ProfileDigest, FilingNo: m.FilingNo, ModelVersion: m.Version, MaxInputTokens: m.MaxInputTokens, SelectionMaxTokens: m.SelectionTokens(), CheckMaxTokens: m.CheckTokens(), PolicyRef: p.Policy.Ref, MinQuotes: p.Policy.MinQuotes, AllowedDomains: p.Policy.AllowedDomains, ApprovalRef: m.ApprovalRef, QualityRef: m.QualityRef, HardBudgetRef: m.HardBudgetRef, PriceRef: "local-eval:price", SourceRef: "local-eval:source", PriceVersion: "local-v1", PricePoints: price, Budget: e.v.Budget}
	if duration > 0 {
		spec.Budget.MaxDurationMS = duration.Milliseconds()
	}
	for _, s := range p.Snapshots {
		spec.Materials = append(spec.Materials, config.AILocalMaterial{SnapshotID: s.ID, Digest: s.Hash, LicenseRef: s.LicenseRef})
	}
	id := strings.TrimPrefix(e.database, "tm_ai_local_")
	runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: e.database, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
	at := time.Now().UnixMilli()
	file := config.AILocalPlans{Version: 1, InstanceID: id, Revision: "v1", Scope: "synthetic-local-only", IssuedAtMS: at - 1000, ExpiresAtMS: at + 600000, Members: []int64{1, 2}, Plans: []config.AILocalPlan{spec}}
	path := filepath.Join(t.TempDir(), "plans.json")
	raw, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := config.NewAILocalPlanSource(path, runtime)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := service.NewAILocalAdmissionControl(source, repository.NewAILocalInstanceVerifier(e.db), e.repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := admission.Ready(e.ctx); err != nil {
		t.Fatal(err)
	}
	creates, err := service.NewAITaskCreateService(e.tasks, admission, true)
	if err != nil {
		t.Fatal(err)
	}
	limiter, err := service.NewSQLDeepLimiter(e.db)
	if err != nil {
		t.Fatal(err)
	}
	creates.SetDeepLimiter(limiter)

	return &localPublicationFixture{e, owner, spec, file, path, raw, runtime, admission, creates}
}
