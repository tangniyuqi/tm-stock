package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type reportPoints struct {
	*scriptedPointsTx
	terminal points.Transaction
}

func (p *reportPoints) SetHold(ctx context.Context, old, next points.Hold) error {
	if err := p.scriptedPointsTx.SetHold(ctx, old, next); err != nil {
		return err
	}
	p.hold = next
	return nil
}
func (p *reportPoints) InsertTransaction(ctx context.Context, v points.Transaction) (int64, error) {
	id, err := p.scriptedPointsTx.InsertTransaction(ctx, v)
	if err == nil {
		v.ID = id
		p.terminal = v
	}
	return id, err
}
func (p *reportPoints) FindTransactionByID(ctx context.Context, _, _ int64) (points.Transaction, error) {
	return p.terminal, p.step(ctx)
}

type reportScript struct {
	*stageScript
	pt                   *reportPoints
	commit               *finalreport.Commit
	snaps                []sources.Snapshot
	theme                string
	evidenceError        error
	entailment, semantic finalreport.Verdict
	reviewCalls          int
	admissionError       error
	reviewHook           func()
}

func (s *reportScript) Points() points.Tx { return s.pt }

// 既有内部夹具无会员准入标记；新会员绑定由独立真库/单测覆盖，不能当默认生产批准。
func (s *reportScript) WithinAdmissionTx(ctx context.Context, fn func(task.AdmissionReadTx) error) error {
	return s.WithinStageTx(ctx, func(task.StageTx) error { return fn(s) })
}
func (s *reportScript) ReadAdmission(ctx context.Context, _ task.Task) (task.AdmissionRecord, bool, error) {
	if s.admissionError != nil {
		return task.AdmissionRecord{}, false, s.admissionError
	}
	return task.AdmissionRecord{}, false, ctx.Err()
}
func (s *reportScript) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	return s.WithinStageTx(ctx, func(task.StageTx) error { return fn(s) })
}
func (s *reportScript) ReportCommit(ctx context.Context, _, _ int64) (finalreport.Commit, error) {
	if err := s.p.step(ctx); err != nil {
		return finalreport.Commit{}, err
	}
	if s.commit == nil {
		return finalreport.Commit{}, points.ErrNotFound
	}
	return *s.commit, nil
}
func (s *reportScript) InsertReportCommit(ctx context.Context, c finalreport.Commit) error {
	if err := s.p.step(ctx); err != nil {
		return err
	}
	s.commit = &c
	return nil
}
func (s *reportScript) ReportEvidence(ctx context.Context, _ []int64, _ int64) ([]sources.Snapshot, string, error) {
	if err := s.p.step(ctx); err != nil {
		return nil, "", err
	}
	return s.snaps, s.theme, s.evidenceError
}
func (s *reportScript) InsertEvent(ctx context.Context, e task.Event) error {
	if err := s.scriptedTaskStore.InsertEvent(ctx, e); err != nil {
		return err
	}
	s.stageEvents[e.Seq] = e
	return nil
}

func reportFixture(t *testing.T) (*AIReportService, *reportScript, finalreport.Request) {
	t.Helper()
	budget, b, reserve := budgetFixture(t)
	if _, err := budget.Reserve(context.Background(), reserve); err != nil {
		t.Fatal(err)
	}
	st := &reportScript{stageScript: &stageScript{budgetScript: b, results: map[int]task.StageResult{}, steps: map[int]task.StageCheckpoint{}, stageEvents: map[int64]task.Event{}}, pt: &reportPoints{scriptedPointsTx: b.p}, theme: "示例题材", entailment: finalreport.Pass, semantic: finalreport.Pass}
	st.p.hold.BizID = "task:" + st.v.No
	cfg := finalreport.Config{Ref: "test-only-policy", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{"evidence.example.test"}}
	policy, err := finalreport.NewPolicy(cfg, func(_ context.Context, in finalreport.EntailmentInput) (finalreport.Verdict, error) {
		if st.inTx {
			t.Fatal("蕴含校验持有数据库事务")
		}
		st.reviewCalls++
		return st.entailment, nil
	}, func(_ context.Context, text string) (finalreport.Verdict, error) {
		if st.inTx {
			t.Fatal("语义校验持有数据库事务")
		}
		st.reviewCalls++
		if st.reviewHook != nil {
			st.reviewHook()
		}
		return st.semantic, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: cfg.AllowedDomains[0], Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "test-only", LicenseRef: "test-only-private-license"}, "https://evidence.example.test/example", sources.MediaPlain, []byte("示例题材是本测试资料的主题。示例题材的事项发生于资料记载日期。示例题材包含资料所述环节。示例题材的公司信息见本段。"), st.p.now, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	snap.ID = 1
	st.snaps = []sources.Snapshot{snap}
	proposal := finalreport.Proposal{TaskNo: st.v.No, ThemeID: st.v.SubjectID, ThemeName: st.theme, Report: render.Report{SchemaVer: 1, Disclosure: render.Disclosure{AIGenerated: true, ModelName: cfg.ModelName, FilingNo: cfg.FilingNo}}}
	for i, key := range []render.SectionKey{render.SecDefinition, render.SecTimeline, render.SecSegment, render.SecCompanies} {
		q := snap.Sentences[i]
		node, err := render.QuoteNode(render.Quote{SnapshotID: 1, Start: q.StartByte, End: q.EndByte, Text: q.Text, SourceType: "年报", URL: snap.URL, FetchedAt: snap.FetchedAt.UnixMilli(), ContentHash: snap.Hash}, render.ProvAIExtractedQuote)
		if err != nil {
			t.Fatal(err)
		}
		title, _ := render.SectionTitle(key)
		proposal.Report.Sections = append(proposal.Report.Sections, render.Section{Key: key, Title: title, AILabel: true, Nodes: []render.Node{node}})
	}
	r := finalreport.Request{TaskNo: st.v.No, Stage: task.StageRequest{UserID: st.v.UserID, TaskID: st.v.ID, Owner: st.v.LeaseOwner, Fence: st.v.Fence, OperationKey: reserve.Key, InputDigest: reserve.InputDigest, StepNo: 1, Stage: "judge"}}
	stage, err := NewAITaskStageService(st, map[string]task.StagePolicy{"judge": policy.StagePolicy()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Capture(context.Background(), r.Stage, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Complete(context.Background(), r.Stage); err != nil {
		t.Fatal(err)
	}
	ledger, err := NewPointsService(&scriptedPointsStore{tx: st.p})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewAIReportService(st, ledger, policy, true)
	if err != nil {
		t.Fatal(err)
	}
	st.p.calls = 0
	st.txCount = 0
	return s, st, r
}

func TestAIReportAtomicReceiptAndReplay(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(fmt.Sprint(rejected), func(t *testing.T) {
			s, st, r := reportFixture(t)
			if rejected {
				st.semantic = finalreport.Uncertain
			}
			got, err := s.Finalize(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			want := task.Succeeded
			if rejected {
				want = task.Failed
			}
			if got.Status != want || st.v.Status != want || st.commit == nil || st.reviewCalls != 2 || st.p.lastAccount.Frozen != 0 || got.TerminalTxnID <= 0 {
				t.Fatal("终检提交不完整", got)
			}
			if rejected && (len(st.commit.Payload) != 0 || got.ReportDigest != "" || st.p.lastAccount.Avail != 20) {
				t.Fatal("拒绝仍保存正文或扣积分")
			}
			if !rejected && (len(st.commit.Payload) == 0 || got.ReportDigest == "" || st.p.lastAccount.Avail != 13) {
				t.Fatal("成功未按锁价结算")
			}
			calls := st.reviewCalls
			again, err := s.Finalize(context.Background(), r)
			if err != nil || again != got || st.reviewCalls != calls {
				t.Fatal("重放重复终检或改变收据", again, err)
			}
		})
	}
}

func TestAIReportBoundariesFailClosed(t *testing.T) {
	s, st, _ := reportFixture(t)
	for _, build := range []func() error{func() error { _, err := NewAIReportService(nil, s.core.ledger, s.policy, true); return err }, func() error { _, err := NewAIReportService(st, nil, s.policy, true); return err }, func() error { _, err := NewAIReportService(st, s.core.ledger, nil, true); return err }} {
		if build() == nil {
			t.Fatal("缺依赖/策略仍启用报告")
		}
	}
	var absent *AIReportService
	if _, err := absent.Finalize(context.Background(), finalreport.Request{}); err == nil {
		t.Fatal("空服务放行")
	}
	off, err := NewAIReportService(st, s.core.ledger, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := off.Finalize(context.Background(), finalreport.Request{}); !errors.Is(err, finalreport.ErrDisabled) {
		t.Fatal("默认关闭失效", err)
	}
	for name, change := range map[string]func(*AIReportService, *reportScript, *finalreport.Request){
		"关闭":   func(s *AIReportService, _ *reportScript, _ *finalreport.Request) { s.enabled = false; s.policy = nil },
		"旧围栏":  func(_ *AIReportService, _ *reportScript, r *finalreport.Request) { r.Stage.Fence++ },
		"错会员":  func(_ *AIReportService, _ *reportScript, r *finalreport.Request) { r.Stage.UserID++ },
		"错任务号": func(_ *AIReportService, _ *reportScript, r *finalreport.Request) { r.TaskNo = strings.Repeat("f", 32) },
		"错输入": func(_ *AIReportService, _ *reportScript, r *finalreport.Request) {
			r.Stage.InputDigest = strings.Repeat("f", 64)
		},
		"非judge": func(_ *AIReportService, _ *reportScript, r *finalreport.Request) { r.Stage.Stage = "agent_call" },
		"专题":     func(_ *AIReportService, st *reportScript, _ *finalreport.Request) { st.v.SubjectType = task.Topic },
		"L2":     func(_ *AIReportService, st *reportScript, _ *finalreport.Request) { st.v.AnalysisType = 2 },
		"损坏前缀":   func(_ *AIReportService, st *reportScript, _ *finalreport.Request) { delete(st.steps, 1) },
		"错锁价":    func(_ *AIReportService, st *reportScript, _ *finalreport.Request) { st.p.hold.Amount++ },
	} {
		t.Run(name, func(t *testing.T) {
			s, st, r := reportFixture(t)
			change(s, st, &r)
			out, err := s.Finalize(context.Background(), r)
			if err == nil || out != (finalreport.Receipt{}) || st.commit != nil || st.reviewCalls != 0 {
				t.Fatal("边界未拒绝", out, err)
			}
			if name == "关闭" && st.p.calls != 0 {
				t.Fatal("关闭仍读取数据库")
			}
		})
	}
	for _, reason := range []string{"cancel", "evidence", "theme", "entailment"} {
		t.Run(reason, func(t *testing.T) {
			s, st, r := reportFixture(t)
			switch reason {
			case "cancel":
				st.v.CancelRequested = true
			case "evidence":
				st.evidenceError = sources.ErrUnauthorized
			case "theme":
				st.theme = "已改名题材"
			case "entailment":
				st.entailment = finalreport.Block
			}
			got, err := s.Finalize(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			want := task.Failed
			if reason == "cancel" {
				want = task.Cancelled
			}
			if got.Status != want || st.p.lastAccount.Avail != 20 || st.p.lastAccount.Frozen != 0 {
				t.Fatal("拒绝/取消未释放", got)
			}
		})
	}
}

func TestAIReportStorageAndUnknownCommitReturnNoCandidate(t *testing.T) {
	s, baseline, r := reportFixture(t)
	if _, err := s.Finalize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	for fail := 1; fail <= baseline.p.calls; fail++ {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			s, st, r := reportFixture(t)
			st.p.failAt = fail
			out, err := s.Finalize(context.Background(), r)
			if !errors.Is(err, errPointsInjected) || out != (finalreport.Receipt{}) {
				t.Fatal("存储错误被吞或泄露候选收据", out, err)
			}
		})
	}
	for phase := 1; phase <= 3; phase++ {
		s, st, r := reportFixture(t)
		st.failCommitAt = phase
		out, err := s.Finalize(context.Background(), r)
		if !errors.Is(err, errPointsInjected) || out != (finalreport.Receipt{}) {
			t.Fatal("提交未知返回候选收据", phase, out, err)
		}
	}
}
