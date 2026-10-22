package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 仅测试发布批准替身，不能注册到产品。
type reportReadControl struct {
	st            *reportScript
	calls, denyAt int
	hook          func()
	err           error
}

func (c *reportReadControl) Recheck(ctx context.Context, p AIReportPublication) error {
	c.calls++
	if c.st.inTx {
		return points.ErrInvariant
	}
	if p.UserID != c.st.v.UserID || p.TaskID != c.st.v.ID || !finalreport.ValidDigest(p.ReportDigest) || !finalreport.ValidDigest(p.PolicyDigest) || p.ModelName != "测试模型" || p.FilingNo != "test-only-filing" {
		return points.ErrInvalid
	}
	if c.hook != nil {
		c.hook()
	}
	if c.calls == c.denyAt {
		return finalreport.ErrUnavailable
	}
	if c.err != nil {
		return c.err
	}
	return ctx.Err()
}
func reportReadFixture(t *testing.T) (*AIReportReadService, *reportScript, *reportReadControl) {
	t.Helper()
	base, st, r := reportFixture(t)
	if _, err := base.Finalize(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	ctl := &reportReadControl{st: st}
	s, err := NewAIReportReadService(base, ctl, true)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, ctl
}
func TestAIReportReadProjectionAndNoMutation(t *testing.T) {
	s, st, ctl := reportReadFixture(t)
	beforeCalls := st.reviewCalls
	hold := st.p.hold
	txn := st.pt.terminal
	v := st.v
	commit := *st.commit
	got, err := s.Read(context.Background(), st.v.UserID, st.v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskID != v.ID || got.SubjectID != v.SubjectID || got.SubjectName != st.theme || !got.Disclosure.AIGenerated || got.Disclosure.ModelName != "测试模型" || len(got.Sections) != 4 || got.GeneratedAt != st.commit.CreatedAt.UnixMilli() || ctl.calls != 2 {
		t.Fatal(got, ctl.calls)
	}
	for i, sec := range got.Sections {
		if !sec.AIGenerated || len(sec.Quotes) != 1 || sec.Quotes[0].Text != st.snaps[0].Sentences[i].Text || sec.Quotes[0].SourceURL != st.snaps[0].URL || sec.Quotes[0].CollectedAt != st.snaps[0].FetchedAt.UnixMilli() {
			t.Fatal("公开投影失真")
		}
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{st.v.No, st.snaps[0].LicenseRef, "snapshot_id", "start_byte", "input_digest", "ReportDigest", "PolicyDigest", "lease", "fence", "verdict", "budget"} {
		if strings.Contains(string(b), private) {
			t.Fatal("内部字段泄漏", private)
		}
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", got, got), st.theme) {
		t.Fatal("日志泄漏正文")
	}
	again, err := s.Read(context.Background(), v.UserID, v.ID)
	if err != nil || again.GeneratedAt != got.GeneratedAt {
		t.Fatal(err)
	}
	ok, err := s.Available(context.Background(), v.UserID, v.ID)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if st.reviewCalls != beforeCalls || st.p.hold != hold || st.pt.terminal != txn || st.v != v || st.commit.ContentDigest != commit.ContentDigest {
		t.Fatal("读取调用模型或改变结算")
	}
}

type reportReadUnknownStore struct {
	finalreport.Store
	calls, failAt int
}

func (s *reportReadUnknownStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	err := s.Store.WithinReportTx(ctx, fn)
	s.calls++
	if err == nil && s.calls == s.failAt {
		return errPointsInjected
	}
	return err
}
func TestAIReportReadClosedGatesAndCurrentSources(t *testing.T) {
	for _, name := range []string{"off", "pending", "failed", "missing_commit", "event", "settlement", "policy", "source", "theme", "revoked_between", "approval", "last_approval", "dependency", "first_commit", "last_commit", "context"} {
		t.Run(name, func(t *testing.T) {
			s, st, c := reportReadFixture(t)
			switch name {
			case "off":
				s.enabled = false
			case "pending":
				st.v.Status = task.Running
			case "failed":
				st.v.Status = task.Failed
			case "missing_commit":
				st.commit = nil
			case "event":
				delete(st.stageEvents, st.v.EventSeq)
			case "settlement":
				st.pt.terminal.Hash = strings.Repeat("f", 64)
			case "policy":
				cp := st.steps[1]
				cp.PolicyDigest = strings.Repeat("f", 64)
				st.steps[1] = cp
			case "source":
				st.evidenceError = sources.ErrUnauthorized
			case "theme":
				st.theme = "另一示例题材"
			case "revoked_between":
				c.hook = func() { st.evidenceError = sources.ErrUnauthorized }
			case "approval":
				c.denyAt = 1
			case "last_approval":
				c.denyAt = 2
			case "dependency":
				c.err = errPointsInjected
			case "first_commit":
				s.base.store = &reportReadUnknownStore{Store: st, failAt: 1}
			case "last_commit":
				s.base.store = &reportReadUnknownStore{Store: st, failAt: 2}
			}
			ctx := context.Background()
			if name == "context" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			out, err := s.Read(ctx, st.v.UserID, st.v.ID)
			if err == nil || out.TaskID != 0 || len(out.Sections) != 0 {
				t.Fatal("失效或不明仍出正文", out, err)
			}
		})
	}
	var missing *AIReportReadService
	if _, err := missing.Read(context.Background(), 1, 1); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := NewAIReportReadService(nil, nil, false); err == nil {
		t.Fatal("空基础核")
	}
	s, st, _ := reportReadFixture(t)
	if _, err := NewAIReportReadService(s.base, nil, true); err == nil {
		t.Fatal("无发布证明")
	}
	if _, err := s.Read(context.Background(), 0, st.v.ID); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	s.enabled = false
	if ok, err := s.Available(context.Background(), st.v.UserID, st.v.ID); ok || err != nil {
		t.Fatal(ok, err)
	}
	s.enabled = true
	s.control = &reportReadControl{st: st, err: errPointsInjected}
	if ok, err := s.Available(context.Background(), st.v.UserID, st.v.ID); ok || err == nil {
		t.Fatal("依赖故障伪装正常不可用")
	}
}

type reportAvailabilityStub struct {
	available bool
	err       error
	calls     int
}

func (s *reportAvailabilityStub) Available(context.Context, int64, int64) (bool, error) {
	s.calls++
	return s.available, s.err
}
func TestAITaskReadReportAvailabilityOnlyOnDetail(t *testing.T) {
	row := readTask(9)
	row.Status = task.Succeeded
	st := &taskReadStub{rows: []task.Task{row}}
	reports := &reportAvailabilityStub{available: true}
	s, err := NewAITaskReadServiceWithReports(st, reports)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Progress(context.Background(), 1, 9)
	if err != nil || !p.ReportAvailable || reports.calls != 1 {
		t.Fatal(p, err)
	}
	h, err := s.History(context.Background(), 1, 0, 20)
	if err != nil || h.Items[0].ReportAvailable || reports.calls != 1 {
		t.Fatal("列表联查或虚构可见性", err)
	}
	reports.err = errPointsInjected
	if out, err := s.Progress(context.Background(), 1, 9); err == nil || out.ID != 0 {
		t.Fatal("可用性故障仍返回旧状态")
	}
	if _, err := NewAITaskReadServiceWithReports(st, nil); err == nil {
		t.Fatal("缺报告依赖")
	}
	if _, err := NewAITaskReadServiceWithReports(nil, reports); err == nil {
		t.Fatal("缺仓储")
	}
}
func TestReportReadSetDeep(t *testing.T) {
	s, _, _ := reportReadFixture(t)
	if err := s.SetDeep(nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("nil deep accepted")
	}
	if err := s.SetDeep(s.base); err == nil {
		t.Fatal("L1 policy accepted as deep")
	}
	cfg := finalreport.Config{Ref: "test-only", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-ent", SemanticVersion: "test-sem", AllowedDomains: []string{"evidence.example.test"}, AnalysisType: task.DeepAnalysisType, MinQuotes: task.DeepMinQuotes}
	l2, err := finalreport.NewPolicy(cfg, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Pass, nil
	}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Pass, nil })
	if err != nil {
		t.Fatal(err)
	}
	deep := &AIReportService{store: s.base.store, core: s.base.core, policy: l2, enabled: true}
	if err := s.SetDeep(deep); err != nil || s.deep != deep {
		t.Fatal("valid deep not attached", err)
	}
}
