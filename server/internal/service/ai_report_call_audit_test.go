package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type callAuditFixture struct {
	*localPublicationStoreFixture
	audits               map[string]finalreport.CallAudit
	insertErr, commitErr error
}

func (s *callAuditFixture) WithinCallAuditTx(ctx context.Context, fn func(finalreport.CallAuditTx) error) error {
	if err := s.WithinStageTx(ctx, func(task.StageTx) error { return fn(s) }); err != nil {
		return err
	}
	// 模拟提交成功但确认丢失；下一次必须根据已保存start拒绝重发。
	return s.commitErr
}
func (s *callAuditFixture) ReadCallAudit(ctx context.Context, _, _ int64, _ int, phase string) (finalreport.CallAudit, error) {
	if err := ctx.Err(); err != nil {
		return finalreport.CallAudit{}, err
	}
	a, found := s.audits[phase]
	if !found {
		return a, points.ErrNotFound
	}
	return a, nil
}
func (s *callAuditFixture) InsertCallAudit(ctx context.Context, a finalreport.CallAudit) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	if err := a.Validate(); err != nil {
		return err
	}
	s.audits[a.Phase] = a
	return ctx.Err()
}
func newCallAuditFixture(t *testing.T) (*AIReportCallAuditService, *callAuditFixture, task.StageRequest, provider.Request) {
	t.Helper()
	_, base, _, _ := localPublicationFixture(t)
	st := base.reportScript
	st.p.now = base.record.CreatedAt.Add(time.Millisecond)
	st.v.Status, st.v.CurrentStep, st.v.LeaseOwner = task.Running, 1, "local_audit_owner_0001"
	until := st.p.now.Add(time.Minute)
	st.v.LeaseUntil = &until
	key := task.StageDigest([]byte("audit-operation"))
	input := task.StageDigest([]byte("audit-input"))
	r := l1Stage(st.v, st.v.LeaseOwner, "extract", 2, key, input)
	st.rows = []task.Reservation{
		{TaskID: st.v.ID, UserID: st.v.UserID, Seq: 1, Owner: r.Owner, Fence: r.Fence, Key: task.StageDigest([]byte("agent")), InputDigest: input, Units: task.Units{Calls: 1, Tokens: 100}, CreatedAt: st.p.now},
		{TaskID: st.v.ID, UserID: st.v.UserID, Seq: 2, Owner: r.Owner, Fence: r.Fence, Key: key, InputDigest: input, Units: task.Units{Calls: 1, Tokens: 2080}, CreatedAt: st.p.now},
	}
	st.state = task.BudgetState{TaskID: st.v.ID, UserID: st.v.UserID, Limits: st.v.Budget.Limits(), Reserved: task.Units{Calls: 2, Tokens: 2180}, Count: 2}
	st.exists = true
	st.results = map[int]task.StageResult{}
	s := &callAuditFixture{localPublicationStoreFixture: base, audits: map[string]finalreport.CallAudit{}}
	svc, err := NewAIReportCallAuditService(s)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0.0
	req := provider.Request{TaskID: st.v.No, Step: 2, Attempt: 1, Model: base.record.Plan.ModelName, System: "仅测试:固定提示", Prompt: "仅测试:资料", MaxTokens: 2048, Format: provider.FormatJSONSchema, Strict: true, Schema: []byte(`{"type":"object"}`), Temperature: &zero}
	return svc, s, r, req
}

func TestReportCallAuditRequiresOriginalReservation(t *testing.T) {
	if _, err := NewAIReportCallAuditService(nil); err == nil {
		t.Fatal("空审计仓储仍可构造")
	}
	for _, mode := range []string{"正常", "无准入", "无预留", "围栏", "原键", "单位", "取消", "租约过期", "模型", "请求追溯", "已捕获", "保存错误", "提交未知"} {
		t.Run(mode, func(t *testing.T) {
			svc, st, r, req := newCallAuditFixture(t)
			switch mode {
			case "无准入":
				st.found = false
			case "无预留":
				st.rows = st.rows[:1]
				st.state.Count = 1
				st.state.Reserved = st.rows[0].Units
			case "围栏":
				r.Fence++
			case "原键":
				r.OperationKey = task.StageDigest([]byte("other"))
			case "单位":
				req.MaxTokens = 5000
			case "取消":
				st.v.CancelRequested = true
			case "租约过期":
				st.v.LeaseUntil = &st.p.now
			case "模型":
				req.Model = "仅测试:其它模型"
			case "请求追溯":
				req.TaskID = "other"
			case "已捕获":
				st.results[2] = task.StageResult{}
			case "保存错误":
				st.insertErr = errPointsInjected
			case "提交未知":
				st.commitErr = errPointsInjected
			}
			before, count := st.v, len(st.rows)
			err := svc.Start(context.Background(), r, req)
			if (err == nil) != (mode == "正常") {
				t.Fatal("调用前存证边界不符", err)
			}
			if st.v != before || len(st.rows) != count {
				t.Fatal("审计改动原任务或预留")
			}
			if mode == "正常" || mode == "提交未知" {
				st.commitErr = nil
				if err := svc.Start(context.Background(), r, req); !errors.Is(err, task.ErrUnconfirmed) || len(st.audits) != 1 {
					t.Fatal("已有start仍允许重发", err)
				}
			} else if len(st.audits) != 0 {
				t.Fatal("拒绝后留下审计")
			}
		})
	}
}

func TestReportCallAuditErrorResultAndCancelledContext(t *testing.T) {
	svc, st, r, req := newCallAuditFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.Start(ctx, r, req); err == nil || len(st.audits) != 0 {
		t.Fatal("取消后仍登记调用", err)
	}
	if err := svc.Start(nil, r, req); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("空上下文未拒绝", err)
	}
	if err := svc.Start(context.Background(), r, req); err != nil {
		t.Fatal(err)
	}
	if err := svc.Finish(context.Background(), r, provider.Response{}, errPointsInjected); err != nil {
		t.Fatal(err)
	}
	if st.audits["finish"].Outcome != "errored" {
		t.Fatal("错误响应被记为已返回成功")
	}
	if err := svc.Finish(nil, r, provider.Response{}, nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("空结果上下文未拒绝", err)
	}
}

func TestReportCallAuditFinishBindingAndReplay(t *testing.T) {
	svc, st, r, req := newCallAuditFixture(t)
	out := provider.Response{Text: "仅测试:结果", Model: req.Model}
	if err := svc.Finish(context.Background(), r, out, nil); !errors.Is(err, points.ErrNotFound) {
		t.Fatal("无start仍记录结果", err)
	}
	if err := svc.Start(context.Background(), r, req); err != nil {
		t.Fatal(err)
	}
	// 旧租约失效/取消及证据读取关闭均不丢掉原调用结果。
	st.v.CancelRequested = true
	st.v.LeaseUntil = &st.p.now
	st.evidenceError = errPointsInjected
	for i := 0; i < 2; i++ {
		if err := svc.Finish(context.Background(), r, out, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(st.audits) != 2 {
		t.Fatal("结果重放新增审计")
	}
	out.Text = "仅测试:不同结果"
	if err := svc.Finish(context.Background(), r, out, nil); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("变更结果没有冲突", err)
	}
	if err := svc.Finish(context.Background(), r, provider.Response{Text: "仅测试:结果", Model: req.Model}, errPointsInjected); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("成功/错误状态可混用", err)
	}
	r.Fence++
	if err := svc.Finish(context.Background(), r, out, nil); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("其它围栏可覆盖原结果", err)
	}
}
