package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 仅测试内存替身：不冒充SQL回滚、隔离、真实预算/来源或定价证明。
type admissionPoints struct {
	*scriptedPointsTx
	lots []points.Lot
}

func (p *admissionPoints) LockUsableLots(ctx context.Context, _ int64, _ time.Time) ([]points.Lot, error) {
	return p.lots, p.step(ctx)
}
func (p *admissionPoints) InsertTransaction(ctx context.Context, v points.Transaction) (int64, error) {
	id, err := p.scriptedPointsTx.InsertTransaction(ctx, v)
	if err == nil {
		v.ID = id
		p.original = v
	}
	return id, err
}
func (p *admissionPoints) InsertHold(ctx context.Context, h points.Hold) (int64, error) {
	id, err := p.scriptedPointsTx.InsertHold(ctx, h)
	if err == nil {
		h.ID = id
		p.hold = h
	}
	return id, err
}
func (p *admissionPoints) SetAccount(ctx context.Context, old, next points.Account) error {
	if err := p.scriptedPointsTx.SetAccount(ctx, old, next); err != nil {
		return err
	}
	p.account = next
	return nil
}
func (p *admissionPoints) SetLot(ctx context.Context, old, next points.Lot) error {
	if err := p.step(ctx); err != nil {
		return err
	}
	for i, v := range p.lots {
		if v.ID == old.ID {
			p.lots[i] = next
		}
	}
	return nil
}

type admissionStore struct {
	*scriptedTaskStore
	points            *admissionPoints
	calls, failCommit int
	inTx              bool
	admission         *task.AdmissionRecord
	marker            string
}

func (s *admissionStore) InsertAdmittedTask(ctx context.Context, v task.Task, digest string) (int64, error) {
	id, err := s.InsertTask(ctx, v)
	if err == nil {
		s.marker = digest
	}
	return id, err
}
func (s *admissionStore) InsertAdmission(ctx context.Context, r task.AdmissionRecord) error {
	if err := s.points.step(ctx); err != nil {
		return err
	}
	s.admission = &r
	return nil
}
func (s *admissionStore) ReadAdmission(ctx context.Context, v task.Task) (task.AdmissionRecord, bool, error) {
	if err := s.points.step(ctx); err != nil {
		return task.AdmissionRecord{}, false, err
	}
	if s.marker == "" && s.admission == nil {
		return task.AdmissionRecord{}, false, nil
	}
	if s.admission == nil || s.admission.Digest != s.marker {
		return task.AdmissionRecord{}, false, points.ErrInvariant
	}
	if err := s.admission.Bind(v); err != nil {
		return task.AdmissionRecord{}, false, err
	}
	return *s.admission, true, nil
}

func (s *admissionStore) Points() points.Tx { return s.points }
func (s *admissionStore) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d, ok := ctx.Deadline()
	if !ok || time.Until(d) > 10*time.Second {
		return points.ErrInvalid
	}
	s.calls++
	s.inTx = true
	defer func() { s.inTx = false }()
	if err := fn(s); err != nil {
		return err
	}
	if s.failCommit == s.calls {
		return errPointsInjected
	}
	return nil
}

type admissionControl struct {
	p                   task.AdmissionPlan
	store               *admissionStore
	plans, checks       int
	failPlan, failCheck error
	denyAt              int
	hook                func(task.AdmissionPlan)
}

func (c *admissionControl) Ready(ctx context.Context) error { return ctx.Err() }
func (c *admissionControl) Plan(ctx context.Context, _ task.AdmissionScope) (task.AdmissionPlan, error) {
	c.plans++
	if c.store.inTx {
		return task.AdmissionPlan{}, points.ErrInvariant
	}
	return c.p, c.failPlan
}
func (c *admissionControl) Recheck(ctx context.Context, p task.AdmissionPlan) error {
	c.checks++
	if c.store.inTx {
		return points.ErrInvariant
	}
	if c.hook != nil {
		c.hook(p)
	}
	if c.checks == c.denyAt {
		return task.ErrAdmissionUnavailable
	}
	return c.failCheck
}
func admissionFixture(t *testing.T) (*AITaskCreateService, *admissionStore, *admissionControl, dto.AITaskCreateInput, string) {
	t.Helper()
	_, st, request, price, budget := taskFixture(t)
	budget.MaxCalls = 6
	p := &admissionPoints{scriptedPointsTx: st.p, lots: []points.Lot{st.p.lot}}
	p.lots[0].EffectiveAt = p.now.Add(-time.Hour)
	store := &admissionStore{scriptedTaskStore: st, points: p}
	ledger, err := NewPointsService(&scriptedPointsStore{tx: st.p})
	if err != nil {
		t.Fatal(err)
	}
	core, err := NewAITaskService(store, ledger)
	if err != nil {
		t.Fatal(err)
	}
	core.entropy = strings.NewReader(strings.Repeat("a", 32))
	c := &admissionControl{store: store, p: task.AdmissionPlan{Scope: task.AdmissionScope{UserID: 1, AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1}, Price: price, Budget: budget, SubjectName: "示例题材", ModelName: "测试模型", FilingNo: "test-only-filing", ProfileDigest: strings.Repeat("a", 64), ApprovalRef: "test-only-model", QualityRef: "test-only-quality", HardBudgetRef: "test-only-budget", PriceRef: "test-only-price", SourceRef: "test-only-source", Materials: []task.AdmissionMaterial{{SnapshotID: 1, Digest: strings.Repeat("b", 64), Domain: "evidence.example.test", Category: sources.CategoryAnnouncement, URL: "https://evidence.example.test/test", CollectedAt: p.now.Add(-time.Hour)}}}}
	c.p.ExecutionDigest = task.StageDigest([]byte("test-only-admission-execution"))
	s, err := NewAITaskCreateService(core, c, true)
	if err != nil {
		t.Fatal(err)
	}
	return s, store, c, dto.AITaskCreateInput{AnalysisType: "light", SubjectType: task.Theme, SubjectID: 1, PriceVersion: price.Version}, request.Key
}
func quoteInput(in dto.AITaskCreateInput) dto.AITaskQuoteInput {
	return dto.AITaskQuoteInput{AnalysisType: in.AnalysisType, SubjectType: in.SubjectType, SubjectID: in.SubjectID}
}

func TestAITaskMemberQuoteUsesEligibleLotsWithoutWriting(t *testing.T) {
	s, store, ctl, input, _ := admissionFixture(t)
	store.points.lots[0].Total = 15
	store.points.lots[0].Remain = 15
	store.points.lots[0].Frozen = 3
	out, err := s.Quote(context.Background(), 1, quoteInput(input))
	if err != nil || out.AvailablePoints != 12 || out.PricePoints != 7 || out.QuotedAt != store.points.now.UnixMilli() || len(out.MaterialScope) != 1 || ctl.plans != 1 || ctl.checks != 2 {
		t.Fatal(out, err)
	}
	if out.MaterialScope[0].SourceType != sources.CategoryAnnouncement {
		t.Fatal("报价来源类别不是约定代码")
	}
	if store.exists || len(store.events) != 0 || len(store.points.entries) != 0 || store.points.account.Avail != 20 || store.points.original.ID != 0 {
		t.Fatal("报价修改账务/任务")
	}
	ctl.hook = func(p task.AdmissionPlan) { p.Materials[0].URL = "https://other.example.test/" }
	out, err = s.Quote(context.Background(), 1, quoteInput(input))
	if err != nil || out.MaterialScope[0].SourceURL != ctl.p.Materials[0].URL {
		t.Fatal("回调修改了内部计划", out, err)
	}
	store.points.account.UserID = 2
	out, err = s.Quote(context.Background(), 1, quoteInput(input))
	if err != nil || out.AvailablePoints != 0 {
		t.Fatal("不存在的账户不是零可用", out, err)
	}
}

func TestAITaskMemberCreateReplayAndReadOnlyReceipt(t *testing.T) {
	s, store, ctl, input, key := admissionFixture(t)
	first, err := s.Create(context.Background(), 1, key, input)
	if err != nil || first.TaskID != 40 || first.PricePoints != 7 || store.points.account.Frozen != 7 || len(store.events) != 1 {
		t.Fatal(first, err)
	}
	before := store.points.calls
	ctl.failPlan = errPointsInjected
	second, err := s.Create(context.Background(), 1, key, input)
	if err != nil || first != second || ctl.plans != 1 || len(store.points.entries) != 2 || store.points.account.Frozen != 7 {
		t.Fatal("旧创建未原价重放", second, err)
	}
	if store.points.calls <= before {
		t.Fatal("未核查持久化创建证据")
	}
	input.SubjectID++
	if out, err := s.Create(context.Background(), 1, key, input); !errors.Is(err, points.ErrIdempotency) || out != (dto.AITaskCreation{}) {
		t.Fatal("同键异体未拒绝", out, err)
	}
	closed, err := NewAITaskCreateService(s.tasks, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := closed.Receipt(context.Background(), 1, key); err != nil || out != first {
		t.Fatal("停新建后不能核查原键", out, err)
	}
	if out, err := closed.Receipt(context.Background(), 2, key); !errors.Is(err, points.ErrNotFound) || out != (dto.AITaskCreation{}) {
		t.Fatal("越权读取", out, err)
	}
	for _, state := range []int{task.Succeeded, task.Failed, task.Cancelled, task.Expired} {
		store.v.Status = state
		store.v.ErrorCode = ""
		store.points.hold.TerminalTxnID = 80
		store.points.hold.Captured = 0
		switch state {
		case task.Succeeded:
			store.points.hold.State = points.Settled
			store.points.hold.Captured = 7
		case task.Failed:
			store.points.hold.State = points.Released
			store.v.ErrorCode = "guard_rejected"
		case task.Cancelled:
			store.points.hold.State = points.Released
			store.v.ErrorCode = "cancelled"
		case task.Expired:
			store.points.hold.State = points.Expired
			store.v.ErrorCode = "expired"
		}
		if out, err := closed.Receipt(context.Background(), 1, key); err != nil || out != first {
			t.Fatal("终态改变了首次创建收据", out, err)
		}
	}
}

func TestAITaskMemberCreationUnknownRequiresOriginalKey(t *testing.T) {
	s, store, ctl, input, key := admissionFixture(t)
	store.failCommit = 2
	if out, err := s.Create(context.Background(), 1, key, input); err == nil || out != (dto.AITaskCreation{}) {
		t.Fatal("提交未知返回成功", out, err)
	}
	if !store.exists || store.points.account.Frozen != 7 {
		t.Fatal("替身没有保留已提交状态")
	}
	ctl.failPlan = task.ErrAdmissionUnavailable
	out, err := s.Receipt(context.Background(), 1, key)
	if err != nil || out.TaskID != 40 {
		t.Fatal(out, err)
	}
	replay, err := s.Create(context.Background(), 1, key, input)
	if err != nil || replay != out || ctl.plans != 1 || len(store.points.entries) != 2 {
		t.Fatal("原键核查后又冻结", replay, err)
	}
}

func TestAITaskMemberAdmissionFailuresNeverFreeze(t *testing.T) {
	for _, mode := range []string{"off", "unsupported", "bad_scope", "bad_key", "bad_plan", "plan_error", "check_error", "price", "balance", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			s, store, ctl, input, key := admissionFixture(t)
			ctx := context.Background()
			switch mode {
			case "off":
				s.enabled = false
			case "unsupported":
				input.AnalysisType = "deep"
			case "bad_scope":
				input.SubjectID = 0
			case "bad_key":
				key = "bad"
			case "bad_plan":
				ctl.p.QualityRef = ""
			case "plan_error":
				ctl.failPlan = errPointsInjected
			case "check_error":
				ctl.failCheck = errPointsInjected
			case "price":
				input.PriceVersion = "old"
			case "balance":
				store.points.account.Avail = 0
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			out, err := s.Create(ctx, 1, key, input)
			if err == nil || out != (dto.AITaskCreation{}) || store.exists || len(store.events) != 0 || len(store.points.entries) != 0 || store.points.account.Frozen != 0 {
				t.Fatal(out, err)
			}
		})
	}
}

func TestAITaskMemberQuoteFailureReturnsNoCandidate(t *testing.T) {
	for _, mode := range []string{"off", "invalid", "plan", "before", "after", "commit", "lot", "duplicate", "sum", "future", "account", "clock"} {
		t.Run(mode, func(t *testing.T) {
			s, store, ctl, input, _ := admissionFixture(t)
			switch mode {
			case "off":
				s.enabled = false
			case "invalid":
				input.AnalysisType = "bad"
			case "plan":
				ctl.failPlan = errPointsInjected
			case "before":
				ctl.denyAt = 1
			case "after":
				ctl.denyAt = 2
			case "commit":
				store.failCommit = 1
			case "lot":
				at := store.points.now.Add(-time.Second)
				store.points.lots[0].ExpiresAt = &at
			case "duplicate":
				store.points.lots = append(store.points.lots, store.points.lots[0])
			case "sum":
				store.points.account.Avail = 1
			case "future":
				ctl.p.Materials[0].CollectedAt = store.points.now.Add(time.Hour)
			case "account":
				store.points.account.Debt = -1
			case "clock":
				store.points.now = time.Time{}
			}
			out, err := s.Quote(context.Background(), 1, quoteInput(input))
			if err == nil || !reflect.DeepEqual(out, dto.AITaskQuote{}) || len(store.points.entries) != 0 {
				t.Fatal(out, err)
			}
		})
	}
}

func TestAITaskMemberReceiptRejectsBrokenOriginalEvidence(t *testing.T) {
	for _, mode := range []string{"hash", "number", "hold", "amount", "state", "freeze", "freeze_hash", "freeze_key", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, store, _, input, key := admissionFixture(t)
			if _, err := s.Create(context.Background(), 1, key, input); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "hash":
				store.v.Hash = strings.Repeat("f", 64)
			case "number":
				store.v.No = strings.Repeat("z", 32)
			case "hold":
				store.points.hold.ID++
			case "amount":
				store.points.hold.Amount++
			case "state":
				store.points.hold.State = points.Settled
			case "freeze":
				store.points.original.ID++
			case "freeze_hash":
				store.points.original.Hash = strings.Repeat("f", 64)
			case "freeze_key":
				store.points.original.Key = "test_wrong_freeze_01"
			case "commit":
				store.failCommit = store.calls + 1
			}
			if out, err := s.Receipt(context.Background(), 1, key); err == nil || out != (dto.AITaskCreation{}) {
				t.Fatal(out, err)
			}
		})
	}
}

func TestAITaskMemberCreateConstructorsAndBoundaries(t *testing.T) {
	if _, err := NewAITaskCreateService(nil, nil, false); err == nil {
		t.Fatal("空任务核")
	}
	s, _, _, input, key := admissionFixture(t)
	if _, err := NewAITaskCreateService(s.tasks, nil, true); err == nil {
		t.Fatal("没有控制器仍启用")
	}
	var absent *AITaskCreateService
	if _, err := absent.Quote(context.Background(), 1, quoteInput(input)); err == nil {
		t.Fatal("空服务")
	}
	if _, err := absent.Create(context.Background(), 1, key, input); err == nil {
		t.Fatal("空服务")
	}
	if _, err := absent.Receipt(context.Background(), 1, key); err == nil {
		t.Fatal("空服务")
	}
	for _, uid := range []int64{0, -1} {
		if _, err := s.Receipt(context.Background(), uid, key); err == nil {
			t.Fatal("坏身份")
		}
	}
	if _, err := s.Receipt(context.Background(), 1, "bad"); err == nil {
		t.Fatal("坏键")
	}
	if _, err := creationProjection(task.Created{}, 1, "v1"); err == nil {
		t.Fatal("空收据")
	}
}

func TestAITaskMemberEveryStorageErrorReturnsEmptyReceipt(t *testing.T) {
	for _, operation := range []string{"quote", "create", "receipt"} {
		prepare := func() (*admissionStore, func() error) {
			s, store, _, input, key := admissionFixture(t)
			if operation == "receipt" {
				if _, err := s.Create(context.Background(), 1, key, input); err != nil {
					t.Fatal(err)
				}
			}
			return store, func() error {
				if operation == "quote" {
					out, err := s.Quote(context.Background(), 1, quoteInput(input))
					if err != nil && !reflect.DeepEqual(out, dto.AITaskQuote{}) {
						t.Fatal("报价故障泄漏候选")
					}
					return err
				}
				var out dto.AITaskCreation
				var err error
				if operation == "receipt" {
					out, err = s.Receipt(context.Background(), 1, key)
				} else {
					out, err = s.Create(context.Background(), 1, key, input)
				}
				if err != nil && out != (dto.AITaskCreation{}) {
					t.Fatal("故障泄漏候选收据")
				}
				return err
			}
		}
		base, run := prepare()
		start := base.points.calls
		if err := run(); err != nil {
			t.Fatal(err)
		}
		count := base.points.calls - start
		for n := 1; n <= count; n++ {
			store, run := prepare()
			store.points.failAt = store.points.calls + n
			if err := run(); err == nil {
				t.Fatal("数据访问错误被吞", operation, n)
			}
		}
	}
}
