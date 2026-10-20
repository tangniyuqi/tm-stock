package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// PointsService 只由可信内部调用方装配，不提供支付、充值或公开扣费路由。
type PointsService struct{ store points.Store }

func NewPointsService(store points.Store) (*PointsService, error) {
	if store == nil {
		return nil, fmt.Errorf("账务仓储不能为空")
	}
	return &PointsService{store: store}, nil
}
func (s *PointsService) operate(ctx context.Context, action func(context.Context, points.Tx) (points.Receipt, error)) (points.Receipt, error) {
	if s == nil || s.store == nil {
		return points.Receipt{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var out points.Receipt
	err := s.store.WithinPointsTx(ctx, func(tx points.Tx) error { var err error; out, err = action(ctx, tx); return err })
	if err != nil {
		return points.Receipt{}, fmt.Errorf("执行账务操作失败: %w", err)
	}
	return out, nil
}
func (s *PointsService) Grant(ctx context.Context, r points.GrantRequest) (points.Receipt, error) {
	return s.operate(ctx, func(ctx context.Context, tx points.Tx) (points.Receipt, error) { return s.GrantInTx(ctx, tx, r) })
}
func (s *PointsService) Freeze(ctx context.Context, r points.FreezeRequest, p points.Price) (points.Receipt, error) {
	return s.operate(ctx, func(ctx context.Context, tx points.Tx) (points.Receipt, error) { return s.FreezeInTx(ctx, tx, r, p) })
}
func (s *PointsService) Finalize(ctx context.Context, r points.FinalizeRequest) (points.Receipt, error) {
	return s.operate(ctx, func(ctx context.Context, tx points.Tx) (points.Receipt, error) { return s.FinalizeInTx(ctx, tx, r) })
}
func (s *PointsService) Reverse(ctx context.Context, r points.ReverseRequest) (points.Receipt, error) {
	return s.operate(ctx, func(ctx context.Context, tx points.Tx) (points.Receipt, error) { return s.ReverseInTx(ctx, tx, r) })
}

func replayPoints(ctx context.Context, tx points.Tx, uid int64, key, hash string) (points.Receipt, bool, error) {
	v, err := tx.FindTransaction(ctx, key)
	if errors.Is(err, points.ErrNotFound) {
		return points.Receipt{}, false, nil
	}
	if err != nil {
		return points.Receipt{}, false, err
	}
	if v.UserID != uid || v.Hash != hash {
		return points.Receipt{}, false, points.ErrIdempotency
	}
	out := points.Receipt{TxnID: v.ID, UserID: uid, Kind: v.Kind}
	if v.Kind == "grant" || v.Kind == "reversal" {
		grantID := v.ID
		if v.Kind == "reversal" {
			grantID = v.ReversesID
		}
		lot, err := tx.FindLotByGrant(ctx, grantID)
		if err != nil {
			return out, false, err
		}
		out.LotID, out.Amount = lot.ID, lot.Total
	} else if v.Kind == "freeze" || v.Kind == "settle" || v.Kind == "release" {
		h, err := tx.FindHoldByTxn(ctx, v.ID, v.Kind != "freeze")
		if err != nil {
			return out, false, err
		}
		out.HoldID, out.Amount, out.PriceVersion = h.ID, h.Amount, h.PriceVersion
	} else {
		return points.Receipt{}, false, points.ErrInvariant
	}
	return out, true, nil
}

// GrantInTx 不提交事务；后续奖励审计/支付占位服务须与本操作共用同一事务。
func (s *PointsService) GrantInTx(ctx context.Context, tx points.Tx, r points.GrantRequest) (points.Receipt, error) {
	if tx == nil {
		return points.Receipt{}, points.ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return points.Receipt{}, err
	}
	r.EffectiveAt = r.EffectiveAt.UTC()
	if r.ExpiresAt != nil {
		utc := r.ExpiresAt.UTC()
		r.ExpiresAt = &utc
	}
	hash, err := points.RequestHash("grant", r)
	if err != nil {
		return points.Receipt{}, err
	}
	if err := tx.EnsureAccount(ctx, r.UserID); err != nil {
		return points.Receipt{}, err
	}
	a, err := tx.LockAccount(ctx, r.UserID)
	if err != nil {
		return points.Receipt{}, err
	}
	if out, found, err := replayPoints(ctx, tx, r.UserID, r.Key, hash); found || err != nil {
		return out, err
	}
	parts, offset, err := planPointsDebtOffset(ctx, tx, a, r)
	if err != nil {
		return points.Receipt{}, err
	}
	next := a
	next.Debt -= offset
	next.Avail, err = points.AddNonnegative(a.Avail, r.Amount-offset)
	if err != nil {
		return points.Receipt{}, err
	}
	if _, err := points.AddNonnegative(next.Avail, a.Frozen); err != nil {
		return points.Receipt{}, err
	}
	txn, err := tx.InsertTransaction(ctx, points.Transaction{UserID: r.UserID, Key: r.Key, Hash: hash, Kind: "grant", BizType: "points_grant", BizID: r.BizID})
	if err != nil {
		return points.Receipt{}, err
	}
	lot, err := tx.InsertLot(ctx, points.Lot{UserID: r.UserID, GrantTxnID: txn, Source: r.Source, CashBacked: r.Source == points.Purchase, Total: r.Amount, Remain: r.Amount - offset, Priority: r.Priority, EffectiveAt: r.EffectiveAt, ExpiresAt: r.ExpiresAt})
	if err != nil {
		return points.Receipt{}, err
	}
	if err := tx.InsertEntry(ctx, points.Entry{UserID: r.UserID, TxnID: txn, LotID: lot, Bucket: points.Available, Delta: r.Amount}); err != nil {
		return points.Receipt{}, err
	}
	if err := applyPointsDebtOffset(ctx, tx, r.UserID, txn, lot, parts, offset); err != nil {
		return points.Receipt{}, err
	}
	if err := tx.SetAccount(ctx, a, next); err != nil {
		return points.Receipt{}, err
	}
	return points.Receipt{TxnID: txn, UserID: r.UserID, LotID: lot, Amount: r.Amount, Kind: "grant"}, nil
}

// FreezeInTx 要与建任务原子提交；重放先于新报价判断，不能因价格变化再冻一次。
func (s *PointsService) FreezeInTx(ctx context.Context, tx points.Tx, r points.FreezeRequest, p points.Price) (points.Receipt, error) {
	if tx == nil {
		return points.Receipt{}, points.ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return points.Receipt{}, err
	}
	r.ExpiresAt = r.ExpiresAt.UTC()
	hash, err := points.RequestHash("freeze", r)
	if err != nil {
		return points.Receipt{}, err
	}
	a, err := tx.LockAccount(ctx, r.UserID)
	if errors.Is(err, points.ErrNotFound) {
		if r.PriceVersion != p.Version {
			return points.Receipt{}, points.ErrPriceChanged
		}
		return points.Receipt{}, points.ErrInsufficient
	}
	if err != nil {
		return points.Receipt{}, err
	}
	if out, found, err := replayPoints(ctx, tx, r.UserID, r.Key, hash); found || err != nil {
		return out, err
	}
	if err := p.Validate(); err != nil {
		return points.Receipt{}, err
	}
	if r.PriceVersion != p.Version {
		return points.Receipt{}, points.ErrPriceChanged
	}
	now, err := tx.Now(ctx)
	if err != nil {
		return points.Receipt{}, err
	}
	if !r.ExpiresAt.After(now) {
		return points.Receipt{}, points.ErrExpired
	}
	if a.Avail < p.Points {
		return points.Receipt{}, points.ErrInsufficient
	}
	lots, err := tx.LockUsableLots(ctx, r.UserID, now)
	if err != nil {
		return points.Receipt{}, err
	}
	type part struct {
		old    points.Lot
		amount int64
	}
	var parts []part
	remaining := p.Points
	for _, lot := range lots {
		if lot.Remain < lot.Frozen || lot.Frozen < 0 || lot.UserID != r.UserID {
			return points.Receipt{}, points.ErrInvariant
		}
		amount := min(remaining, lot.Remain-lot.Frozen)
		if amount > 0 {
			parts = append(parts, part{lot, amount})
			remaining -= amount
		}
		if remaining == 0 {
			break
		}
	}
	if remaining != 0 {
		return points.Receipt{}, points.ErrInsufficient
	}
	next := a
	next.Avail -= p.Points
	next.Frozen, err = points.AddNonnegative(a.Frozen, p.Points)
	if err != nil {
		return points.Receipt{}, err
	}
	txn, err := tx.InsertTransaction(ctx, points.Transaction{UserID: r.UserID, Key: r.Key, Hash: hash, Kind: "freeze", BizType: r.BizType, BizID: r.BizID})
	if err != nil {
		return points.Receipt{}, err
	}
	hold, err := tx.InsertHold(ctx, points.Hold{UserID: r.UserID, BizType: r.BizType, BizID: r.BizID, FreezeTxnID: txn, Amount: p.Points, PriceVersion: p.Version, State: points.Holding, ExpiresAt: r.ExpiresAt})
	if err != nil {
		return points.Receipt{}, err
	}
	for _, part := range parts {
		nextLot := part.old
		nextLot.Frozen += part.amount
		if err := tx.SetLot(ctx, part.old, nextLot); err != nil {
			return points.Receipt{}, err
		}
		if err := tx.InsertAllocation(ctx, points.Allocation{HoldID: hold, LotID: part.old.ID, UserID: r.UserID, Amount: part.amount}); err != nil {
			return points.Receipt{}, err
		}
		for _, e := range []points.Entry{{UserID: r.UserID, TxnID: txn, LotID: part.old.ID, Bucket: points.Available, Delta: -part.amount}, {UserID: r.UserID, TxnID: txn, LotID: part.old.ID, Bucket: points.Frozen, Delta: part.amount}} {
			if err := tx.InsertEntry(ctx, e); err != nil {
				return points.Receipt{}, err
			}
		}
	}
	if err := tx.SetAccount(ctx, a, next); err != nil {
		return points.Receipt{}, err
	}
	return points.Receipt{TxnID: txn, UserID: r.UserID, HoldID: hold, Amount: p.Points, Kind: "freeze", PriceVersion: p.Version}, nil
}

// FinalizeInTx 不接受追加金额，只能按冻结单锁价全额结算或全额释放。
func (s *PointsService) FinalizeInTx(ctx context.Context, tx points.Tx, r points.FinalizeRequest) (points.Receipt, error) {
	if tx == nil {
		return points.Receipt{}, points.ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return points.Receipt{}, err
	}
	hash, err := points.RequestHash("finalize", r)
	if err != nil {
		return points.Receipt{}, err
	}
	a, err := tx.LockAccount(ctx, r.UserID)
	if err != nil {
		return points.Receipt{}, err
	}
	if out, found, err := replayPoints(ctx, tx, r.UserID, r.Key, hash); found || err != nil {
		return out, err
	}
	h, err := tx.LockHold(ctx, r.UserID, r.HoldID)
	if err != nil {
		return points.Receipt{}, err
	}
	if h.State != points.Holding {
		return points.Receipt{}, points.ErrClosed
	}
	now, err := tx.Now(ctx)
	if err != nil {
		return points.Receipt{}, err
	}
	if r.State == points.Settled && !h.ExpiresAt.After(now) {
		return points.Receipt{}, points.ErrExpired
	}
	if r.State == points.Expired && h.ExpiresAt.After(now) {
		return points.Receipt{}, points.ErrInvalid
	}
	allocations, err := tx.ListAllocations(ctx, h.ID)
	if err != nil {
		return points.Receipt{}, err
	}
	var total int64
	for _, part := range allocations {
		if part.Amount <= 0 || part.UserID != r.UserID || part.HoldID != h.ID {
			return points.Receipt{}, points.ErrInvariant
		}
		total, err = points.AddNonnegative(total, part.Amount)
		if err != nil {
			return points.Receipt{}, err
		}
	}
	if total != h.Amount || a.Frozen < h.Amount {
		return points.Receipt{}, points.ErrInvariant
	}
	kind := "release"
	if r.State == points.Settled {
		kind = "settle"
	}
	txn, err := tx.InsertTransaction(ctx, points.Transaction{UserID: r.UserID, Key: r.Key, Hash: hash, Kind: kind, BizType: h.BizType, BizID: h.BizID})
	if err != nil {
		return points.Receipt{}, err
	}
	for _, part := range allocations {
		lot, err := tx.LockLot(ctx, r.UserID, part.LotID)
		if err != nil {
			return points.Receipt{}, err
		}
		if lot.Frozen < part.Amount || lot.Remain < part.Amount {
			return points.Receipt{}, points.ErrInvariant
		}
		next := lot
		next.Frozen -= part.Amount
		if r.State == points.Settled {
			next.Remain -= part.Amount
		}
		if err := tx.SetLot(ctx, lot, next); err != nil {
			return points.Receipt{}, err
		}
		if err := tx.InsertEntry(ctx, points.Entry{UserID: r.UserID, TxnID: txn, LotID: lot.ID, Bucket: points.Frozen, Delta: -part.Amount}); err != nil {
			return points.Receipt{}, err
		}
		if r.State != points.Settled {
			if err := tx.InsertEntry(ctx, points.Entry{UserID: r.UserID, TxnID: txn, LotID: lot.ID, Bucket: points.Available, Delta: part.Amount}); err != nil {
				return points.Receipt{}, err
			}
		}
	}
	nextHold := h
	nextHold.State = r.State
	nextHold.TerminalTxnID = txn
	if r.State == points.Settled {
		nextHold.Captured = h.Amount
	}
	if err := tx.SetHold(ctx, h, nextHold); err != nil {
		return points.Receipt{}, err
	}
	nextAccount := a
	nextAccount.Frozen -= h.Amount
	if r.State != points.Settled {
		nextAccount.Avail, err = points.AddNonnegative(a.Avail, h.Amount)
		if err != nil {
			return points.Receipt{}, err
		}
	}
	if err := tx.SetAccount(ctx, a, nextAccount); err != nil {
		return points.Receipt{}, err
	}
	return points.Receipt{TxnID: txn, UserID: r.UserID, HoldID: h.ID, Amount: h.Amount, Kind: kind, PriceVersion: h.PriceVersion}, nil
}
