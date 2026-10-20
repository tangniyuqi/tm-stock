package service

import (
	"context"
	"fmt"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type pointsDebtPart struct {
	old    points.Debt
	amount int64
}

// 奖励在入账时先抵扣欠款；生效时间仅约束剩余奖励的消费。购买本金不参与抵扣。
func planPointsDebtOffset(ctx context.Context, tx points.Tx, account points.Account, grant points.GrantRequest) ([]pointsDebtPart, int64, error) {
	if account.Debt == 0 || grant.Source == points.Purchase {
		return nil, 0, nil
	}
	debts, err := tx.LockOutstandingDebts(ctx, grant.UserID)
	if err != nil {
		return nil, 0, err
	}
	var total, previous int64
	for _, debt := range debts {
		if debt.ID <= previous || debt.UserID != grant.UserID || debt.Outstanding <= 0 || debt.Outstanding > debt.Amount {
			return nil, 0, points.ErrInvariant
		}
		previous = debt.ID
		total, err = points.AddNonnegative(total, debt.Outstanding)
		if err != nil {
			return nil, 0, err
		}
	}
	if total != account.Debt {
		return nil, 0, points.ErrInvariant
	}
	offset := min(total, grant.Amount)
	remaining := offset
	var parts []pointsDebtPart
	for _, debt := range debts {
		amount := min(remaining, debt.Outstanding)
		if amount > 0 {
			parts = append(parts, pointsDebtPart{debt, amount})
			remaining -= amount
		}
		if remaining == 0 {
			break
		}
	}
	return parts, offset, nil
}

func applyPointsDebtOffset(ctx context.Context, tx points.Tx, uid, grantID, lotID int64, parts []pointsDebtPart, amount int64) error {
	if amount == 0 {
		return nil
	}
	hash, err := points.RequestHash("debt_offset", struct{ UserID, GrantTxnID int64 }{uid, grantID})
	if err != nil {
		return err
	}
	offsetID, err := tx.InsertTransaction(ctx, points.Transaction{UserID: uid, Key: "points_offset_" + hash, Hash: hash, Kind: "debt_offset", BizType: "debt_offset", BizID: fmt.Sprintf("grant:%d", grantID)})
	if err != nil {
		return err
	}
	for _, part := range parts {
		next := part.old
		next.Outstanding -= part.amount
		if err := tx.SetDebt(ctx, part.old, next); err != nil {
			return err
		}
		if err := tx.InsertDebtAllocation(ctx, points.DebtAllocation{UserID: uid, TxnID: offsetID, DebtID: part.old.ID, LotID: lotID, Amount: part.amount}); err != nil {
			return err
		}
	}
	return tx.InsertEntry(ctx, points.Entry{UserID: uid, TxnID: offsetID, LotID: lotID, Bucket: points.Available, Delta: -amount})
}

// ReverseInTx 整笔冲正一次，冻结批次须先终结任务；回收原批次，不挪用其他批次本金。
func (s *PointsService) ReverseInTx(ctx context.Context, tx points.Tx, r points.ReverseRequest) (points.Receipt, error) {
	if tx == nil {
		return points.Receipt{}, points.ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return points.Receipt{}, err
	}
	hash, err := points.RequestHash("reversal", r)
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
	original, err := tx.FindTransactionByID(ctx, r.UserID, r.GrantTxnID)
	if err != nil {
		return points.Receipt{}, err
	}
	if original.Kind != "grant" || original.UserID != r.UserID {
		return points.Receipt{}, points.ErrInvalid
	}
	lot, err := tx.FindLotByGrant(ctx, original.ID)
	if err != nil {
		return points.Receipt{}, err
	}
	lot, err = tx.LockLot(ctx, r.UserID, lot.ID)
	if err != nil {
		return points.Receipt{}, err
	}
	if lot.UserID != r.UserID || lot.GrantTxnID != original.ID || lot.Total <= 0 || lot.Remain < 0 || lot.Remain > lot.Total || lot.Frozen < 0 || lot.Frozen > lot.Remain {
		return points.Receipt{}, points.ErrInvariant
	}
	if lot.Frozen != 0 {
		return points.Receipt{}, fmt.Errorf("原批次仍有任务冻结，须先终结相关任务: %w", points.ErrConflict)
	}
	if a.Avail < lot.Remain {
		return points.Receipt{}, points.ErrInvariant
	}
	expired, err := tx.ReadLotExpired(ctx, r.UserID, lot.ID)
	if err != nil {
		return points.Receipt{}, err
	}
	if expired < 0 || expired > lot.Total-lot.Remain {
		return points.Receipt{}, points.ErrInvariant
	}
	debt := lot.Total - lot.Remain - expired
	next := a
	next.Avail -= lot.Remain
	next.Debt, err = points.AddNonnegative(a.Debt, debt)
	if err != nil {
		return points.Receipt{}, err
	}
	reversal, err := tx.InsertTransaction(ctx, points.Transaction{UserID: r.UserID, Key: r.Key, Hash: hash, Kind: "reversal", BizType: "points_refund", BizID: r.BizID, ReversesID: original.ID})
	if err != nil {
		return points.Receipt{}, err
	}
	if lot.Remain > 0 {
		newLot := lot
		newLot.Remain = 0
		if err := tx.SetLot(ctx, lot, newLot); err != nil {
			return points.Receipt{}, err
		}
		if err := tx.InsertEntry(ctx, points.Entry{UserID: r.UserID, TxnID: reversal, LotID: lot.ID, Bucket: points.Available, Delta: -lot.Remain}); err != nil {
			return points.Receipt{}, err
		}
	}
	if debt > 0 {
		if _, err := tx.InsertDebt(ctx, points.Debt{UserID: r.UserID, ReversalTxnID: reversal, Amount: debt, Outstanding: debt}); err != nil {
			return points.Receipt{}, err
		}
	}
	// 全部已经记账过期且无欠款时账户无变化，避免 MySQL 将无变化更新计为 0 行。
	if next != a {
		if err := tx.SetAccount(ctx, a, next); err != nil {
			return points.Receipt{}, err
		}
	}
	return points.Receipt{TxnID: reversal, UserID: r.UserID, LotID: lot.ID, Amount: lot.Total, Kind: "reversal"}, nil
}
