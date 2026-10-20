package repository

import (
	"context"
	"math"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 已过期记账不能再次算作欠款。必须当前读：上层 RR 事务可能已建立早于过期记账的快照。
func (t *pointsTx) ReadLotExpired(ctx context.Context, uid, lotID int64) (int64, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT e.delta FROM tm_points_entries e JOIN tm_points_txns x ON x.id=e.txn_id AND x.user_id=e.user_id WHERE e.user_id=? AND e.lot_id=? AND x.kind='expire' AND e.bucket=1 FOR UPDATE`, uid, lotID)
	if err != nil {
		return 0, pointsDBError("读取批次已记账过期额", err)
	}
	defer rows.Close()
	var amount int64
	for rows.Next() {
		var delta int64
		if err := rows.Scan(&delta); err != nil {
			return 0, pointsDBError("扫描批次过期分录", err)
		}
		if delta >= 0 {
			return 0, points.ErrInvariant
		}
		if delta == math.MinInt64 {
			return 0, points.ErrOverflow
		}
		amount, err = points.AddNonnegative(amount, -delta)
		if err != nil {
			return 0, err
		}
	}
	return amount, pointsDBError("遍历批次过期分录", rows.Err())
}

func (t *pointsTx) LockOutstandingDebts(ctx context.Context, uid int64) ([]points.Debt, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT id,user_id,reversal_txn_id,amount,outstanding FROM tm_points_debts WHERE user_id=? AND outstanding>0 ORDER BY id FOR UPDATE`, uid)
	if err != nil {
		return nil, pointsDBError("锁定待抵扣欠款", err)
	}
	defer rows.Close()
	var out []points.Debt
	for rows.Next() {
		var v points.Debt
		if err := rows.Scan(&v.ID, &v.UserID, &v.ReversalTxnID, &v.Amount, &v.Outstanding); err != nil {
			return nil, pointsDBError("扫描待抵扣欠款", err)
		}
		out = append(out, v)
	}
	return out, pointsDBError("遍历待抵扣欠款", rows.Err())
}

func (t *pointsTx) InsertDebt(ctx context.Context, v points.Debt) (int64, error) {
	r, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_debts(user_id,reversal_txn_id,amount,outstanding,created_at,updated_at) VALUES(?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.ReversalTxnID, v.Amount, v.Outstanding)
	if err != nil {
		return 0, pointsDBError("记录冲正欠款", err)
	}
	return r.LastInsertId()
}

func (t *pointsTx) SetDebt(ctx context.Context, old, next points.Debt) error {
	return onePointsRow(t.tx.ExecContext(ctx, `UPDATE tm_points_debts SET outstanding=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND user_id=? AND outstanding=?`, next.Outstanding, old.ID, old.UserID, old.Outstanding))
}

func (t *pointsTx) InsertDebtAllocation(ctx context.Context, a points.DebtAllocation) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_debt_allocs(txn_id,debt_id,lot_id,user_id,amount,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, a.TxnID, a.DebtID, a.LotID, a.UserID, a.Amount)
	return pointsDBError("记录欠款抵扣分摊", err)
}
