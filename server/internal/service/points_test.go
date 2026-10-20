package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 脚本事务用于逐个数据访问边界注入失败，不模拟数据库隔离或回滚；后二者由 MySQL 取证。
type scriptedPointsTx struct {
	points.Tx
	now             time.Time
	account         points.Account
	lot             points.Lot
	hold            points.Hold
	existing        points.Transaction
	allocations     []points.Allocation
	debts           []points.Debt
	expired         int64
	original        points.Transaction
	lastAccount     points.Account
	newLot          points.Lot
	newDebt         points.Debt
	entries         []points.Entry
	debtAllocations []points.DebtAllocation
	calls, failAt   int
}
type scriptedPointsStore struct {
	tx          *scriptedPointsTx
	commitError error
}

var errPointsInjected = fmt.Errorf("测试账务故障")

func (s *scriptedPointsStore) WithinPointsTx(ctx context.Context, fn func(points.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 30*time.Second {
		return fmt.Errorf("账务事务未受期限保护")
	}
	if err := fn(s.tx); err != nil {
		return err
	}
	return s.commitError
}
func (s *scriptedPointsTx) step(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		return fmt.Errorf("数据访问未收到受限上下文")
	}
	s.calls++
	if s.calls == s.failAt {
		return errPointsInjected
	}
	return nil
}
func (s *scriptedPointsTx) Now(ctx context.Context) (time.Time, error)       { return s.now, s.step(ctx) }
func (s *scriptedPointsTx) EnsureAccount(ctx context.Context, _ int64) error { return s.step(ctx) }
func (s *scriptedPointsTx) LockAccount(ctx context.Context, uid int64) (points.Account, error) {
	if err := s.step(ctx); err != nil {
		return points.Account{}, err
	}
	if uid != s.account.UserID {
		return points.Account{}, points.ErrNotFound
	}
	return s.account, nil
}
func (s *scriptedPointsTx) FindTransaction(ctx context.Context, _ string) (points.Transaction, error) {
	if err := s.step(ctx); err != nil {
		return points.Transaction{}, err
	}
	if s.existing.ID == 0 {
		return points.Transaction{}, points.ErrNotFound
	}
	return s.existing, nil
}
func (s *scriptedPointsTx) InsertTransaction(ctx context.Context, _ points.Transaction) (int64, error) {
	return 10, s.step(ctx)
}
func (s *scriptedPointsTx) SetAccount(ctx context.Context, _ points.Account, next points.Account) error {
	s.lastAccount = next
	return s.step(ctx)
}
func (s *scriptedPointsTx) InsertLot(ctx context.Context, lot points.Lot) (int64, error) {
	s.newLot = lot
	return 20, s.step(ctx)
}
func (s *scriptedPointsTx) FindLotByGrant(ctx context.Context, _ int64) (points.Lot, error) {
	return s.lot, s.step(ctx)
}
func (s *scriptedPointsTx) LockUsableLots(ctx context.Context, _ int64, _ time.Time) ([]points.Lot, error) {
	return []points.Lot{s.lot}, s.step(ctx)
}
func (s *scriptedPointsTx) LockLot(ctx context.Context, _, _ int64) (points.Lot, error) {
	return s.lot, s.step(ctx)
}
func (s *scriptedPointsTx) SetLot(ctx context.Context, _, _ points.Lot) error { return s.step(ctx) }
func (s *scriptedPointsTx) InsertEntry(ctx context.Context, e points.Entry) error {
	s.entries = append(s.entries, e)
	return s.step(ctx)
}
func (s *scriptedPointsTx) FindTransactionByID(ctx context.Context, _, _ int64) (points.Transaction, error) {
	return s.original, s.step(ctx)
}
func (s *scriptedPointsTx) ReadLotExpired(ctx context.Context, _, _ int64) (int64, error) {
	return s.expired, s.step(ctx)
}
func (s *scriptedPointsTx) LockOutstandingDebts(ctx context.Context, _ int64) ([]points.Debt, error) {
	return s.debts, s.step(ctx)
}
func (s *scriptedPointsTx) InsertDebt(ctx context.Context, d points.Debt) (int64, error) {
	s.newDebt = d
	return 40, s.step(ctx)
}
func (s *scriptedPointsTx) SetDebt(ctx context.Context, _, _ points.Debt) error { return s.step(ctx) }
func (s *scriptedPointsTx) InsertDebtAllocation(ctx context.Context, a points.DebtAllocation) error {
	s.debtAllocations = append(s.debtAllocations, a)
	return s.step(ctx)
}
func (s *scriptedPointsTx) InsertHold(ctx context.Context, _ points.Hold) (int64, error) {
	return 30, s.step(ctx)
}
func (s *scriptedPointsTx) LockHold(ctx context.Context, uid, _ int64) (points.Hold, error) {
	if err := s.step(ctx); err != nil {
		return points.Hold{}, err
	}
	if uid != s.hold.UserID {
		return points.Hold{}, points.ErrNotFound
	}
	return s.hold, nil
}
func (s *scriptedPointsTx) FindHoldByTxn(ctx context.Context, _ int64, _ bool) (points.Hold, error) {
	return s.hold, s.step(ctx)
}
func (s *scriptedPointsTx) SetHold(ctx context.Context, _, _ points.Hold) error { return s.step(ctx) }
func (s *scriptedPointsTx) InsertAllocation(ctx context.Context, _ points.Allocation) error {
	return s.step(ctx)
}
func (s *scriptedPointsTx) ListAllocations(ctx context.Context, _ int64) ([]points.Allocation, error) {
	return s.allocations, s.step(ctx)
}
func (s *scriptedPointsTx) ReadAudit(ctx context.Context, _ int64) (points.AuditRows, error) {
	return points.AuditRows{Account: points.Account{UserID: 1}}, s.step(ctx)
}

func scriptedPoints(t *testing.T) (*PointsService, *scriptedPointsStore, points.GrantRequest, points.FreezeRequest, points.FinalizeRequest) {
	t.Helper()
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	tx := &scriptedPointsTx{now: now, account: points.Account{UserID: 1, Avail: 20}, lot: points.Lot{ID: 20, UserID: 1, Total: 20, Remain: 20}, hold: points.Hold{ID: 30, UserID: 1, Amount: 7, State: points.Holding, BizType: "ai_task", BizID: "task-1", PriceVersion: "v1", ExpiresAt: now.Add(time.Hour)}, allocations: []points.Allocation{{HoldID: 30, LotID: 20, UserID: 1, Amount: 7}}}
	store := &scriptedPointsStore{tx: tx}
	s, err := NewPointsService(store)
	if err != nil {
		t.Fatal(err)
	}
	return s, store, points.GrantRequest{UserID: 1, Key: "test_grant_0000001", BizID: "grant-1", Amount: 20, Source: points.Gift, EffectiveAt: now}, points.FreezeRequest{UserID: 1, Key: "test_freeze_000001", BizType: "ai_task", BizID: "task-1", PriceVersion: "v1", ExpiresAt: now.Add(time.Hour)}, points.FinalizeRequest{UserID: 1, Key: "test_final_0000001", HoldID: 30, State: points.Settled}
}

func TestPointsEveryStorageBoundaryPropagatesFailure(t *testing.T) {
	for _, kind := range []string{"grant", "freeze", "settle", "release"} {
		prepare := func() (*scriptedPointsStore, func() (points.Receipt, error)) {
			s, store, g, f, last := scriptedPoints(t)
			ctx := context.Background()
			switch kind {
			case "grant":
				return store, func() (points.Receipt, error) { return s.Grant(ctx, g) }
			case "freeze":
				return store, func() (points.Receipt, error) { return s.Freeze(ctx, f, points.Price{Version: "v1", Points: 7}) }
			default:
				store.tx.account.Avail = 13
				store.tx.account.Frozen = 7
				store.tx.lot.Frozen = 7
				if kind == "release" {
					last.State = points.Released
				}
				return store, func() (points.Receipt, error) { return s.Finalize(ctx, last) }
			}
		}
		baseline, run := prepare()
		if _, err := run(); err != nil {
			t.Fatal(err)
		}
		for fail := 1; fail <= baseline.tx.calls; fail++ {
			t.Run(fmt.Sprintf("%s_%d", kind, fail), func(t *testing.T) {
				store, run := prepare()
				store.tx.failAt = fail
				if out, err := run(); !errors.Is(err, errPointsInjected) || out != (points.Receipt{}) {
					t.Fatal("数据访问故障被吞或返回成功收据", err)
				}
			})
		}
		store, run := prepare()
		store.commitError = errPointsInjected
		if out, err := run(); !errors.Is(err, errPointsInjected) || out != (points.Receipt{}) {
			t.Fatal("提交未知结果被误报成功")
		}
	}
}

func TestPointsServiceGuardsReplayAndCancellation(t *testing.T) {
	s, store, g, f, last := scriptedPoints(t)
	ctx := context.Background()
	if _, err := NewPointsService(nil); err == nil {
		t.Fatal("空仓储放行")
	}
	var nilService *PointsService
	if _, err := nilService.Grant(ctx, g); err == nil {
		t.Fatal("空服务放行")
	}
	if _, err := s.GrantInTx(ctx, nil, g); err == nil {
		t.Fatal("空事务放行")
	}
	for _, fn := range []func() error{func() error { _, err := s.Grant(ctx, points.GrantRequest{}); return err }, func() error { _, err := s.Freeze(ctx, points.FreezeRequest{}, points.Price{}); return err }, func() error { _, err := s.Finalize(ctx, points.FinalizeRequest{}); return err }} {
		if fn() == nil {
			t.Fatal("非法请求放行")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Grant(canceled, g); !errors.Is(err, context.Canceled) {
		t.Fatal("取消未传播", err)
	}
	hash, _ := points.RequestHash("grant", g)
	store.tx.existing = points.Transaction{ID: 10, UserID: 1, Kind: "grant", Hash: hash}
	if r, err := s.Grant(ctx, g); err != nil || r.Amount != 20 || r.LotID != 20 {
		t.Fatal("入账重放失败", err)
	}
	g.Amount++
	if _, err := s.Grant(ctx, g); !errors.Is(err, points.ErrIdempotency) {
		t.Fatal("不同请求未拒绝", err)
	}
	hash, _ = points.RequestHash("freeze", f)
	store.tx.existing = points.Transaction{ID: 10, UserID: 1, Kind: "freeze", Hash: hash}
	if r, err := s.Freeze(ctx, f, points.Price{Version: "v2", Points: 99}); err != nil || r.Amount != 7 || r.PriceVersion != "v1" {
		t.Fatal("冻结重放变价", err)
	}
	hash, _ = points.RequestHash("finalize", last)
	store.tx.existing = points.Transaction{ID: 10, UserID: 1, Kind: "settle", Hash: hash}
	if r, err := s.Finalize(ctx, last); err != nil || r.Amount != 7 {
		t.Fatal("终态重放失败", err)
	}
	store.tx.existing = points.Transaction{}
	if _, err := s.Freeze(ctx, f, points.Price{Version: "v2", Points: 7}); !errors.Is(err, points.ErrPriceChanged) {
		t.Fatal(err)
	}
	if _, err := s.Freeze(ctx, f, points.Price{Version: "v1", Points: 21}); !errors.Is(err, points.ErrInsufficient) {
		t.Fatal(err)
	}
	store.tx.account.Avail = math.MaxInt64
	g.Amount = 1
	g.Key = "test_grant_0000002"
	if _, err := s.Grant(ctx, g); !errors.Is(err, points.ErrOverflow) {
		t.Fatal("溢出未拒绝", err)
	}
	store.tx.account.Avail = 20
	store.tx.account.Frozen = 7
	store.tx.lot.Frozen = 7
	store.tx.hold.State = points.Released
	if _, err := s.Finalize(ctx, last); !errors.Is(err, points.ErrClosed) {
		t.Fatal(err)
	}
	store.tx.hold.State = points.Holding
	last.State = points.Expired
	if _, err := s.Finalize(ctx, last); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("提前过期放行", err)
	}
	store.tx.now = store.tx.hold.ExpiresAt
	last.State = points.Settled
	if _, err := s.Finalize(ctx, last); !errors.Is(err, points.ErrExpired) {
		t.Fatal("过期后扣费放行", err)
	}
	last.State = points.Expired
	if _, err := s.Finalize(ctx, last); err != nil {
		t.Fatal("过期释放失败", err)
	}
	store.tx.allocations[0].Amount = 6
	if _, err := s.Finalize(ctx, last); !errors.Is(err, points.ErrInvariant) {
		t.Fatal("分摊损坏未拒绝", err)
	}
	if _, err := s.Reconcile(ctx, 0); err == nil {
		t.Fatal("非法对账用户放行")
	}
	store.tx.failAt = store.tx.calls + 1
	if _, err := s.Reconcile(ctx, 1); err == nil {
		t.Fatal("对账错误被吞")
	}
	store.tx.failAt = 0
	if a, err := s.Reconcile(ctx, 1); err != nil || !a.Healthy() {
		t.Fatal(a, err)
	}
}

func TestPointsReconcileDetectsTamperingWithoutOverflow(t *testing.T) {
	base := func() points.AuditRows {
		return points.AuditRows{Account: points.Account{UserID: 1, Avail: 13, Frozen: 7}, Transactions: []points.Transaction{{ID: 1, UserID: 1, Kind: "grant", BizType: "points_grant"}, {ID: 3, UserID: 1, Kind: "freeze", BizType: "ai_task", BizID: "task-1"}}, Lots: []points.Lot{{ID: 2, UserID: 1, GrantTxnID: 1, Source: points.Gift, Total: 20, Remain: 20, Frozen: 7}}, Entries: []points.Entry{{UserID: 1, LotID: 2, TxnID: 1, Bucket: points.Available, Delta: 20}, {UserID: 1, LotID: 2, TxnID: 3, Bucket: points.Available, Delta: -7}, {UserID: 1, LotID: 2, TxnID: 3, Bucket: points.Frozen, Delta: 7}}, Holds: []points.Hold{{ID: 4, UserID: 1, Amount: 7, FreezeTxnID: 3, BizType: "ai_task", BizID: "task-1", State: points.Holding}}, Allocations: []points.Allocation{{HoldID: 4, LotID: 2, UserID: 1, Amount: 7}}}
	}
	if report := ReconcilePoints(base()); !report.Healthy() {
		t.Fatal(report)
	}
	for _, change := range []func(*points.AuditRows){func(r *points.AuditRows) { r.Account.Avail++ }, func(r *points.AuditRows) { r.Account.Frozen++ }, func(r *points.AuditRows) { r.Account.Debt++ }, func(r *points.AuditRows) { r.Lots[0].Remain-- }, func(r *points.AuditRows) {
		r.Entries[0].Delta = math.MaxInt64
		r.Entries = append(r.Entries, r.Entries[0])
	}, func(r *points.AuditRows) { r.Entries[0].Bucket = 3 }, func(r *points.AuditRows) { r.Entries[0].UserID = 2 }, func(r *points.AuditRows) { r.Allocations[0].Amount++ }, func(r *points.AuditRows) { r.Allocations[0].LotID = 9 }, func(r *points.AuditRows) { r.Holds[0].State = 99 }, func(r *points.AuditRows) { r.Holds[0].Captured = 1 }, func(r *points.AuditRows) { r.Debts = []points.Debt{{Outstanding: -1}} }} {
		r := base()
		change(&r)
		if ReconcilePoints(r).Healthy() {
			t.Fatal("对账未发现人为破坏")
		}
	}
}
