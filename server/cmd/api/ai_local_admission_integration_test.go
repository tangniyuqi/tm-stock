//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 仅受控父程序本轮新库运行；配置/DSN经stdin，不读取产品配置或宿主模型密钥。
func TestAILocalPlanBootstrap(t *testing.T) {
	if os.Getenv("TM_TEST_AI_LOCAL_PLAN_CHILD") != "local-fixture-only" {
		t.Skip("仅原生本地计划专项子进程")
	}
	var input struct {
		DSN     string `json:"dsn"`
		Runtime string `json:"runtime"`
		Plans   string `json:"plans"`
		Fund    bool   `json:"fund"`
	}
	if json.NewDecoder(io.LimitReader(os.Stdin, 16384)).Decode(&input) != nil {
		t.Fatal("隔离计划测试配置无效")
	}
	runtime, err := config.ReadAILocalRuntime(input.Runtime)
	if err != nil || runtime.CheckDSN(input.DSN) != nil || !filepath.IsAbs(input.Plans) || filepath.Dir(input.Plans) != filepath.Dir(input.Runtime) {
		t.Fatal("隔离计划路径/库身份无效")
	}
	if _, err := os.Lstat(input.Plans); !os.IsNotExist(err) {
		t.Fatal("计划文件已存在，拒绝覆盖")
	}
	db, err := sql.Open("mysql", input.DSN)
	if err != nil {
		t.Fatal("隔离库连接失败")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := repository.NewAILocalInstanceVerifier(db).Verify(ctx, runtime.InstanceID); err != nil {
		t.Fatal("隔离标记未通过")
	}
	var count int
	if db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM tm_ai_sources)+(SELECT COUNT(*) FROM addon_quant_theme)+(SELECT COUNT(*) FROM tm_ai_tasks)+(SELECT COUNT(*) FROM tm_points_accounts)`).Scan(&count) != nil || count != 0 {
		t.Fatal("不是本轮空专属库")
	}
	name := "仅测试:本地计划合成题材"
	if _, err := db.ExecContext(ctx, `INSERT INTO addon_quant_theme(id,name,status) VALUES(1,?,1)`, name); err != nil {
		t.Fatal("测试题材初始化失败")
	}
	repo := repository.NewAISourceRepo(db)
	source, err := repo.RegisterSource(ctx, sources.Source{Domain: "evidence.example.test", Category: sources.CategoryAnnualReport, LicenseTag: "仅测试:合成资料", LicenseRef: "local-eval:license"})
	if err != nil {
		t.Fatal("合成来源登记失败")
	}
	if err := repo.SetSourceEnabled(ctx, source.ID, true); err != nil {
		t.Fatal("合成来源启用失败")
	}
	source.Enabled = true
	now := time.Now().UTC().Truncate(time.Millisecond).Add(-time.Second)
	snap, err := sources.BuildSnapshot(source, "https://evidence.example.test/local-plan", sources.MediaPlain, []byte("仅测试合成题材用于本次本地验收。合成资料记录测试事件日期。合成资料记录测试环节分工。合成资料记录测试公司事实。合成资料记录测试出处类别。合成资料记录测试文本版本。"), now, 1<<20)
	if err != nil {
		t.Fatal("合成快照构造失败")
	}
	snap, err = repo.SaveSnapshot(ctx, snap)
	if err != nil {
		t.Fatal("合成快照保存失败")
	}
	spec := config.AILocalPlan{AnalysisType: 1, SubjectID: 1, SubjectName: name, ActorID: 7, Symbol: "000001", AgentProfileDigest: strings.Repeat("b", 64), AgentCalls: 1, AgentTokens: 100, AgentPages: 0, ModelName: "仅测试:模型", ModelProfileDigest: strings.Repeat("c", 64), FilingNo: "仅测试:公示编号", ModelVersion: "local-eval:v1", MaxInputTokens: 32, SelectionMaxTokens: 2048, CheckMaxTokens: 64, PolicyRef: "local-eval:policy", AllowedDomains: []string{source.Domain}, ApprovalRef: "local-eval:approval", QualityRef: "local-eval:quality", HardBudgetRef: "local-eval:hard-budget", PriceRef: "local-eval:price", SourceRef: "local-eval:source", PriceVersion: "local-v1", PricePoints: 7, Budget: task.Budget{MaxAttempts: 1, MaxCalls: 6, MaxTokens: 5000, MaxPages: 10, MaxDurationMS: 60000}, Materials: []config.AILocalMaterial{{SnapshotID: snap.ID, Digest: snap.Hash, LicenseRef: snap.LicenseRef}}}
	deep := spec
	deep.AnalysisType = 2
	deep.MinQuotes = 6
	deep.PricePoints = 30
	p := config.AILocalPlans{Version: 1, InstanceID: runtime.InstanceID, Revision: "v1", Scope: "synthetic-local-only", IssuedAtMS: now.UnixMilli(), ExpiresAtMS: now.Add(time.Hour).UnixMilli(), Members: []int64{1, 2}, Plans: []config.AILocalPlan{spec, deep}}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal("计划编码失败")
	}
	f, err := os.OpenFile(input.Plans, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal("计划文件建立失败")
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("计划文件写入失败")
	}
	t.Setenv("TM_AI_LOCAL_PLANS", input.Plans)
	control, err := prepareAILocalAdmission(db, runtime)
	if err != nil {
		t.Fatal("实际API本地控制器装配失败")
	}
	// 用正式启动函数执行全部原准入/深度报告结构探针；这里不启动HTTP或真实模型。
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("x", 40), Issuer: "GVA", Audience: "GVA"}}
	creates, err := prepareAITaskCreatesWithDeep(ctx, db, read, true, true, control)
	if err != nil {
		t.Fatal("实际创建服务装配未通过", err)
	}
	for _, kind := range []string{"light", "deep"} {
		q, err := creates.Quote(ctx, 1, dto.AITaskQuoteInput{AnalysisType: kind, SubjectType: task.Theme, SubjectID: 1})
		want := int64(7)
		if kind == "deep" {
			want = 30
		}
		if err != nil || q.PricePoints != want || q.AvailablePoints != 0 || len(q.MaterialScope) != 1 {
			t.Fatal("合成计划报价/来源范围不符", err)
		}
		_, err = creates.Create(ctx, 1, "local_plan_test_"+kind, dto.AITaskCreateInput{AnalysisType: kind, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "local-v1"})
		if !errors.Is(err, points.ErrInsufficient) {
			t.Fatal("无积分创建未明确拒绝", err)
		}
	}
	if err := repo.SetSourceEnabled(ctx, source.ID, false); err != nil {
		t.Fatal("来源撤回反例建立失败")
	}
	if err := control.Ready(ctx); err == nil {
		t.Fatal("来源撤回仍放行")
	}
	if err := repo.SetSourceEnabled(ctx, source.ID, true); err != nil {
		t.Fatal("来源恢复失败")
	}
	p.Plans[0].PricePoints++
	changed, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if os.WriteFile(input.Plans, changed, 0600) != nil {
		t.Fatal("价格变更反例建立失败")
	}
	if err := control.Ready(ctx); err == nil {
		t.Fatal("文件变价未拒绝")
	}
	if os.WriteFile(input.Plans, raw, 0600) != nil {
		t.Fatal("原计划文件恢复失败")
	}
	if err := control.Ready(ctx); err != nil {
		t.Fatal("原文件/来源恢复后未通过", err)
	}
	if db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM tm_ai_tasks)+(SELECT COUNT(*) FROM tm_points_accounts)+(SELECT COUNT(*) FROM tm_points_txns)`).Scan(&count) != nil || count != 0 {
		t.Fatal("启动探针或拒绝路径留下任务/积分")
	}
	if input.Fund {
		if err := repository.NewAIConversationTaskSchemaVerifier(db).Verify(ctx); err != nil {
			t.Fatal("原非本地会话任务核验路径回归", err)
		}
		ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
		if err != nil {
			t.Fatal(err)
		}
		for _, uid := range []int64{1, 2} {
			key := "local_task_grant_one"
			if uid == 2 {
				key = "local_task_grant_two"
			}
			if _, err := ledger.Grant(ctx, points.GrantRequest{UserID: uid, Key: key, BizID: key, Amount: 50, Source: points.Gift, EffectiveAt: now}); err != nil {
				t.Fatal("仅本轮测试积分初始化失败")
			}
		}
		t.Log("LOCAL_TASK_FUNDS_VERIFIED：两个隔离测试会员各50测试积分；无生产积分或原会员旧积分迁移")
	}
	t.Log("LOCAL_PLAN_BOOTSTRAP_VERIFIED：原创建服务装配、轻深报价、来源撤回/恢复、文件变价拒绝及零积分创建拒绝；无任务/账务或模型调用")
}
