//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 配置文件、原数据库预算/调用审计和结算均实际执行；资料、模型与CN协议为合成回环夹具。
// 本组HTTP用测试签发JWT，原密码登录/独立进程另行验收。
func configureDossierLocalFixture(t *testing.T, f *localPublicationFixture, mode string, models ...provider.Config) *service.AIDossierSupportRegistryService {
	t.Helper()
	if len(models) > 1 {
		t.Fatal("每个资料样板只允许一个实际模型配置")
	}
	comparison := strings.HasPrefix(mode, "comparison_")
	mode = strings.TrimPrefix(mode, "comparison_")
	e := f.env
	reportDDL(t, e.pointsTestEnv, "20261030_create_tm_ai_dossiers.sql")
	reportDDL(t, e.pointsTestEnv, "20261030_register_tm_ai_dossier_support_plans.sql")
	reportDDL(t, e.pointsTestEnv, "20261102_allow_dossier_report_model_step.sql")
	selection := workflowSelectionJSON(t, e.snap.ID)
	if f.spec.AnalysisType == 2 {
		var response finalreport.SelectionResponse
		if json.Unmarshal([]byte(selection), &response) != nil {
			t.Fatal("合成选择器JSON错误")
		}
		response.Selections = append(response.Selections, finalreport.Selection{Section: "segment", SnapshotID: e.snap.ID, SentenceIDs: []int{5}}, finalreport.Selection{Section: "companies", SnapshotID: e.snap.ID, SentenceIDs: []int{6}})
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		selection = string(raw)
	}
	verdict, reason := "supported", "direct_support"
	switch mode {
	case "partial":
		verdict, reason = "partial", "partial_support"
	case "unsupported":
		verdict, reason = "unsupported", "not_supported"
	case "uncertain":
		verdict, reason = "uncertain", "insufficient_context"
	}
	support := supportSQLReply(verdict, reason)
	if mode == "malformed" {
		support = providertest.Success(`{"results":[]}`, provider.Usage{Prompt: 2, Completion: 1, Total: 3})
	}
	if mode == "transport" {
		support = providertest.Abort()
	}
	ent, sem := checkModelReply("pass", "supported"), checkModelReply("pass", "factual")
	if mode == "entailment" {
		ent = checkModelReply("block", "unsupported")
	}
	if mode == "semantic" {
		sem = checkModelReply("block", "prohibited")
	}
	if comparison {
		rows := []map[string]any{}
		for i := 1; i <= 6; i++ {
			row := map[string]any{"id": i, "verdict": verdict, "reason": reason}
			if mode == "partial_consistent" && i == 2 {
				row["verdict"], row["reason"] = "partial", "partial_support"
			}
			rows = append(rows, row)
		}
		raw, err := json.Marshal(map[string]any{"results": rows})
		if err != nil {
			t.Fatal(err)
		}
		support = providertest.Success(string(raw), provider.Usage{Prompt: 2, Completion: 2, Total: 4})
		axes := []map[string]any{}
		for i := 1; i <= 7; i++ {
			left, right, relation := "unknown", "unknown", "not_applicable"
			if i == 1 || i == 4 {
				left, right, relation = "supported", "supported", "aligned"
			}
			axes = append(axes, map[string]any{"id": i, "left": left, "right": right, "relation": relation})
		}
		if mode == "mismatch" || mode == "partial_consistent" {
			axes[0]["left"], axes[0]["relation"] = "partial", "uncertain"
		}
		compared, err := json.Marshal(map[string]any{"axes": axes, "wording": "same_meaning"})
		if err != nil {
			t.Fatal(err)
		}
		compareReply := providertest.Success(string(compared), provider.Usage{Prompt: 2, Completion: 2, Total: 4})
		if mode == "comparison_transport" {
			compareReply = providertest.Abort()
		}
		if mode == "comparison_malformed" {
			compareReply = providertest.Success(`{"axes":[]}`, provider.Usage{Prompt: 2, Completion: 2, Total: 4})
		}
		e.server = providertest.NewServer(t, providertest.Success(selection, provider.Usage{Prompt: 2, Completion: 1, Total: 3}), ent, support, compareReply, sem)
	} else {
		e.server = providertest.NewServer(t, providertest.Success(selection, provider.Usage{Prompt: 2, Completion: 1, Total: 3}), ent, support, sem)
	}
	modelConfig := workflowModelConfig(e.server.URL())
	outputTokens := 128
	if len(models) == 1 {
		modelConfig = models[0]
		outputTokens = 1024 // 六项字段回执不能沿用单卡合成夹具的128令牌上限。
	}
	model, err := provider.NewDirect(modelConfig, true)
	if err != nil {
		t.Fatal(err)
	}
	e.model = model
	f.spec.ModelProfileDigest = model.DirectProfileDigest()
	sc := dossier.SupportConfig{Model: f.spec.ModelName, Version: f.spec.ModelVersion, FilingNo: f.spec.FilingNo, ApprovalRef: f.spec.ApprovalRef, QualityRef: f.spec.QualityRef, HardBudgetRef: f.spec.HardBudgetRef, ProfileDigest: f.spec.ModelProfileDigest, MaxInputTokens: f.spec.MaxInputTokens, MaxOutputTokens: outputTokens, MaxPromptBytes: 16000, ReviewStep: 4}
	u := dossier.Field{State: "unknown"}
	sentence := e.snap.Sentences[0]
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 1, Version: 1, CreatedAt: e.snap.FetchedAt.UnixMilli(), Cards: []dossier.FactCard{{Key: "test_card", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: e.snap.ID, Hash: e.snap.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}
	if comparison {
		for _, item := range []struct {
			field *dossier.Field
			text  string
		}{{&d.Cards[0].Subject, f.spec.SubjectName}, {&d.Cards[0].Scope, "本测试资料"}} {
			start := strings.Index(sentence.Text, item.text)
			if start < 0 {
				t.Fatal("合成比较字段缺失")
			}
			ref := d.Cards[0].Statement
			ref.StartByte = sentence.StartByte + start
			ref.EndByte = ref.StartByte + len(item.text)
			*item.field = dossier.Field{State: "known", Ref: ref}
		}
		second := d.Cards[0]
		second.Key = "test_card_b"
		d.Cards = append(d.Cards, second)
	}
	dossiers, err := service.NewAIDossierService(repository.NewAIDossierRepo(e.db, sql.LevelReadCommitted), dossierTestAuth{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = dossiers.StageInternal(e.ctx, d); err != nil {
		t.Fatal(err)
	}
	e.mustExec(`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:publication',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions;
INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:member',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions`)
	bound, err := dossier.BindDraft(d, []sources.Snapshot{e.snap})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"test_card"}
	if comparison {
		keys = append(keys, "test_card_b")
	}
	prepared, err := dossier.PrepareSupport(e.ctx, sc, f.spec.SubjectName, bound, []sources.Snapshot{e.snap}, keys)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepared.Manifest(f.spec.AnalysisType)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(e.db, sql.LevelReadCommitted), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterInternal(e.ctx, m); err != nil {
		t.Fatal(err)
	}
	if _, err = e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_uses(component_digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(?,'test-fixture:ai-use',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.ComponentDigest()); err != nil {
		t.Fatal(err)
	}
	f.spec.ReportProfile = task.DossierReportProfile
	f.spec.DossierSupportDigest = m.ComponentDigest()
	if comparison {
		reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
		reportDDL(t, e.pointsTestEnv, "20261102_enable_dossier_comparison_report_audit.sql")
		mc := sc
		mc.ReviewStep = 0
		mc.MaxOutputTokens = 1024
		cp, err := dossier.PrepareCardComparison(e.ctx, dossier.CardComparisonConfig{Model: mc, Focus: "definition", ReviewStep: 5}, f.spec.SubjectName, bound, []sources.Snapshot{e.snap}, keys)
		if err != nil {
			t.Fatal(err)
		}
		manifest, err := cp.Manifest()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = registry.RegisterComparisonInternal(e.ctx, manifest); err != nil {
			t.Fatal(err)
		}
		if _, err = e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_uses(component_digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(?,'test-fixture:comparison-ai-use',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, manifest.ComponentDigest()); err != nil {
			t.Fatal(err)
		}
		f.spec.ReportProfile = task.DossierComparisonReportProfile
		f.spec.DossierComparisonDigest = manifest.ComponentDigest()
		if f.spec.Budget.MaxCalls < 7 {
			f.spec.Budget.MaxCalls = 7
		}
	}
	f.file.Plans = []config.AILocalPlan{f.spec}
	f.raw, err = json.Marshal(f.file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.path, f.raw, 0600); err != nil {
		t.Fatal(err)
	}
	return registry
}

type dossierWorkflowFixture struct {
	*localPublicationFixture
	web    *reportHTTPEnv
	worker *service.AIL1ReportWorker
	id     int64
}

func newDossierWorkflowFixture(t *testing.T, level sql.IsolationLevel, kind int, mode string) *dossierWorkflowFixture {
	t.Helper()
	baseMode := "local"
	if kind == 2 {
		baseMode = "local_deep"
	}
	f := newLocalPublicationFixture(t, level, kind, baseMode)
	e := f.env
	registry := configureDossierLocalFixture(t, f, mode)
	web := reportHTTPFixtureWithModel(t, e.reportEnv, "", true, f.spec.ModelName, f.spec.FilingNo)
	source, err := config.NewAILocalPlanSource(f.path, f.runtime)
	if err != nil {
		t.Fatal(err)
	}
	f.admission, err = service.NewAILocalAdmissionControl(source, repository.NewAILocalInstanceVerifier(e.db), e.repo, registry)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.admission.Ready(e.ctx); err != nil {
		t.Fatal("新完整本地准入失败", err)
	}
	f.creates, err = service.NewAITaskCreateService(e.tasks, f.admission, true)
	if err != nil {
		t.Fatal(err)
	}
	limiter, err := service.NewSQLDeepLimiter(e.db)
	if err != nil {
		t.Fatal(err)
	}
	f.creates.SetDeepLimiter(limiter)
	verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: web.key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(e.db))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		t.Fatal(err)
	}
	handler.NewAITaskCreateHandler(f.creates).Register(web.server.Config.Handler.(*http.ServeMux), auth, true)
	analysis := "light"
	if kind == 2 {
		analysis = "deep"
	}
	body := `{"analysis_type":"` + analysis + `","subject_type":"theme","subject_id":1,"price_version":"local-v1"}`
	request, _ := http.NewRequestWithContext(e.ctx, "POST", web.server.URL+"/api/v1/ai/tasks", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+web.token(t, 1, "client"))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "test_dossier_full_report")
	response, err := web.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var created struct {
		Code int
		Data dto.AITaskCreation
	}
	if json.NewDecoder(response.Body).Decode(&created) != nil || response.StatusCode != 200 || created.Code != 0 || created.Data.TaskID <= 0 {
		t.Fatal("HTTP完整准入创建失败", response.StatusCode)
	}
	e.v, err = e.tasks.Inspect(e.ctx, 1, created.Data.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	planner, err := service.NewAILocalReportPlanner(f.admission)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := service.NewAIReportCallAuditService(e.repo)
	if err != nil {
		t.Fatal(err)
	}
	selection, checks, model, err := service.NewAILocalReportModelControls(f.admission, e.ledger, e.payloads, workflowModelConfig(e.server.URL()), audit)
	if err != nil {
		t.Fatal(err)
	}
	supportControl, err := service.NewAIDossierSupportExecutionControl(registry, e.repo, f.admission, audit)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := service.NewAIDossierReportWorker(e.tasks, e.repo, e.ledger, e.agent, e.payloads, model, planner, selection, checks, supportControl, f.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if f.spec.ReportProfile == task.DossierComparisonReportProfile {
		comparison, err := service.NewAIDossierComparisonExecutionControl(registry, e.repo, f.admission, audit)
		if err != nil {
			t.Fatal(err)
		}
		worker, err = service.NewAIDossierComparisonReportWorker(e.tasks, e.repo, e.ledger, e.agent, e.payloads, model, planner, selection, checks, supportControl, comparison, f.owner, agenttask.Options{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 30 * time.Second}, PollInterval: 50 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
	}
	return &dossierWorkflowFixture{localPublicationFixture: f, web: web, worker: worker, id: created.Data.TaskID}
}

func TestIntegrationAIReportDossierWorkflow(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					for _, mode := range []string{"supported", "partial", "unsupported", "uncertain", "entailment", "semantic", "malformed", "transport"} {
						t.Run(mode, func(t *testing.T) {
							f := newDossierWorkflowFixture(t, level, kind, mode)
							e := f.env
							got, err := f.worker.Run(e.ctx, 1, f.id)
							unknown := mode == "malformed" || mode == "transport"
							if unknown {
								if mode == "transport" && err == nil || mode == "malformed" && (err != nil || got.Task.Outcome != "response_unknown") {
									t.Fatal("坏回执或未知传输被当可交付结果", err)
								}
								v, readErr := e.tasks.Inspect(e.ctx, 1, f.id)
								if readErr != nil || v.Status != task.Running || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != int(f.spec.PricePoints) {
									t.Fatal("未知调用擅自释放", readErr)
								}
								calls := len(e.server.Requests())
								again, rerunErr := f.worker.Run(e.ctx, 1, f.id)
								if len(e.server.Requests()) != calls || rerunErr != nil || again.Task.Outcome != "response_unknown" {
									t.Fatal("未知恢复重发或改变拒绝类别", rerunErr)
								}
								captured := 0
								if mode == "malformed" {
									captured = 1
								}
								if calls != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_step_results WHERE step_no=4`) != captured || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_steps WHERE step_no=4`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 {
									t.Fatal("未知调用现场与原捕获边界不符")
								}
								requirePointAudit(t, e.pointsTestEnv, e.ledger)
								return
							}
							if err != nil {
								t.Fatal("六步完整编排失败", err)
							}
							want := task.Succeeded
							if mode == "uncertain" || mode == "entailment" || mode == "semantic" {
								want = task.Failed
							}
							v, err := e.tasks.Inspect(e.ctx, 1, f.id)
							if err != nil || v.Status != want || got.Commit.Status != want || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 {
								t.Fatal("任务/提交/账务终态不一致", err)
							}
							calls := 4
							audits := 8
							steps := 6
							if mode == "uncertain" {
								calls = 3
								audits = 6
								steps = 5
							}
							if mode == "entailment" {
								calls = 2
								audits = 4
								steps = 4
							}
							if len(e.server.Requests()) != calls || e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != audits || v.CurrentStep != steps {
								t.Fatal("六步原调用存证或提前失败步数不符")
							}
							if _, err = f.worker.Run(e.ctx, 1, f.id); err != nil || len(e.server.Requests()) != calls || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind IN ('settle','release')`) != 2 {
								t.Fatal("终态重放重复结算", err)
							}
							if want == task.Succeeded {
								ctl, err := service.NewAILocalReportReadControl(f.admission)
								if err != nil {
									t.Fatal(err)
								}
								resolved, err := ctl.Resolve(e.ctx, 1, f.id)
								if err != nil || resolved.Policy == nil {
									t.Fatal("成功报告不能重建原捕获策略", err)
								}
								base, err := service.NewAIReportService(e.repo, e.ledger, nil, false)
								if err != nil {
									t.Fatal(err)
								}
								reads, err := service.NewAIReportReadServiceWithPolicySource(base, ctl, ctl, true)
								if err != nil {
									t.Fatal(err)
								}
								if report, err := reads.Read(e.ctx, 1, f.id); err != nil || report.TaskID != f.id {
									t.Fatal("当前成功报告无法读取", err)
								}
								e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=0`)
								if report, err := reads.Read(e.ctx, 1, f.id); err == nil || report.TaskID != 0 {
									t.Fatal("撤回AI用途仍交付成功报告")
								}
								e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
								if report, err := reads.Read(e.ctx, 1, f.id); err != nil || report.TaskID != f.id || len(e.server.Requests()) != calls {
									t.Fatal("恢复后读取触发模型或丢报告", err)
								}
							}
							requirePointAudit(t, e.pointsTestEnv, e.ledger)
						})
					}
				})
			}
		})
	}
}
