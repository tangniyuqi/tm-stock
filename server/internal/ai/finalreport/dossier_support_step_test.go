package finalreport

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestDossierSupportSealedModelStep(t *testing.T) {
	s, e := sources.BuildSnapshot(sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "test-only", LicenseRef: "test-only"},
		"https://example.test/test-only", sources.MediaPlain, []byte("仅测试：样本描述只用于执行适配。"), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 4096)
	if e != nil {
		t.Fatal(e)
	}
	s.ID = 1
	u := dossier.Field{State: "unknown"}
	sent := s.Sentences[0]
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 1, Version: 1, CreatedAt: s.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{{Key: "test_card", Section: "definition",
		Statement: dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: sent.Number, StartByte: sent.StartByte, EndByte: sent.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
	b, e := dossier.BindDraft(d, []sources.Snapshot{s})
	if e != nil {
		t.Fatal(e)
	}
	c := dossier.SupportConfig{Model: "test-model", Version: "test-v1", FilingNo: "test-filing", ApprovalRef: "test-approval", QualityRef: "test-quality", HardBudgetRef: "test-budget", ProfileDigest: strings.Repeat("a", 64), MaxInputTokens: 1000, MaxOutputTokens: 128, MaxPromptBytes: 16000}
	for _, kind := range []int{1, 2} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			v := task.Task{ID: 1, UserID: 1, No: strings.Repeat("b", 32), SubjectID: 1, SubjectType: task.Theme, AnalysisType: kind}
			p, e := dossier.NewSupportPlan(context.Background(), c, v, "仅测试", b, []sources.Snapshot{s}, []string{"test_card"})
			if e != nil {
				t.Fatal(e)
			}
			step, e := NewDossierSupportStep(p)
			if e != nil || !step.Valid() || step.StageName() != "judge" || step.InputDigest() != p.InputDigest() || !step.MatchesTask(v) || step.OperationKey(v, 3) != p.OperationKey(v, 3) || step.Units() != p.Units() {
				t.Fatal("封闭适配失配", e)
			}
			cfg := step.Config()
			wantKind := 0
			if kind == 2 {
				wantKind = 2
			}
			if cfg.Model != c.Model || cfg.ProfileDigest != c.ProfileDigest || cfg.CheckTokens() != c.MaxOutputTokens || cfg.MaxInputTokens != c.MaxInputTokens || cfg.AnalysisType != wantKind {
				t.Fatal("配置转接失配")
			}
			r := step.Request()
			if r.Prompt != p.Request().Prompt {
				t.Fatal("适配改写提示")
			}
			r.Schema[0] = 'x'
			if step.Request().Schema[0] != '{' {
				t.Fatal("适配丢失副本边界")
			}
			response := provider.Response{Model: c.Model, Text: `{"results":[{"id":1,"verdict":"partial","reason":"partial_support"}]}`, FinishReason: "stop", RequestID: "test-only", Usage: provider.Usage{Prompt: 1, Completion: 1, Total: 2}}
			raw, e := step.CapturePayload(response)
			if e != nil {
				t.Fatal(e)
			}
			if e = step.StagePolicy().Check(context.Background(), task.StageValidationInput{Stage: "judge", InputDigest: p.InputDigest(), Payload: string(raw)}); e != nil {
				t.Fatal("部分支持未保留", e)
			}
			if strings.Contains(fmt.Sprintf("%+v %#v", step, step), "test_card") {
				t.Fatal("适配器格式化泄露")
			}
		})
	}
	if got, e := NewDossierSupportStep(nil); got != nil || e == nil {
		t.Fatal("空核验计划仍适配")
	}
	var empty *DossierSupportStep
	if empty.Valid() || empty.InputDigest() != "" || empty.Config() != (CheckConfig{}) || empty.MatchesTask(task.Task{}) || empty.OperationKey(task.Task{}, 3) != "" || empty.Units() != (task.Units{}) || empty.Request().Model != "" || empty.StagePolicy().Valid() {
		t.Fatal("空适配有能力")
	}
	if got, e := empty.CapturePayload(provider.Response{}); got != nil || e == nil {
		t.Fatal("空适配仍捕获")
	}
}
