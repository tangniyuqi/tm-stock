package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type PointsRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewPointsRepo(db *sql.DB) *PointsRepo {
	return &PointsRepo{db: db, isolation: sql.LevelReadCommitted}
}

// NewPointsRepoWithIsolation 只支持已纳入并发验证的两种隔离级别。
func NewPointsRepoWithIsolation(db *sql.DB, level sql.IsolationLevel) *PointsRepo {
	return &PointsRepo{db: db, isolation: level}
}

var _ points.Store = (*PointsRepo)(nil)

func (r *PointsRepo) WithinPointsTx(ctx context.Context, action func(points.Tx) error) error {
	if r == nil || r.db == nil || action == nil || (r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead) {
		return points.ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return fmt.Errorf("开启账务事务失败: %w", err)
	}
	// 各失败路径都回滚；提交后 ErrTxDone 不改变已经提交的事实。
	defer func() { _ = tx.Rollback() }()
	if err := action(PointsTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交账务事务失败，结果须用原幂等键核查: %w", err)
	}
	return nil
}

type pointsTx struct{ tx *sql.Tx }

// PointsTx 供任务事务复用同一数据库事务，调用方负责提交/回滚与期限。
func PointsTx(tx *sql.Tx) points.Tx {
	if tx == nil {
		return nil
	}
	return &pointsTx{tx: tx}
}
func (t *pointsTx) Now(ctx context.Context) (time.Time, error) {
	var raw string
	if err := t.tx.QueryRowContext(ctx, "SELECT CAST(UTC_TIMESTAMP(3) AS CHAR)").Scan(&raw); err != nil {
		return time.Time{}, pointsDBError("读取账务数据库时点", err)
	}
	return parsePointsUTC(raw)
}

// DATETIME 是无时区的 UTC 墙上时间。字符串参数/读取避免驱动 loc 再做一次隐式时区转换。
func pointsUTC(value time.Time) string { return value.UTC().Format("2006-01-02 15:04:05.000") }
func parsePointsUTC(value string) (time.Time, error) {
	out, err := time.Parse("2006-01-02 15:04:05.999999", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("解析账本 UTC 时点失败: %w", err)
	}
	return out, nil
}
func pointsDBError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return points.ErrNotFound
	}
	var me *mysqldrv.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return points.ErrConflict
	}
	return fmt.Errorf("%s失败: %w", operation, err)
}
func (t *pointsTx) EnsureAccount(ctx context.Context, uid int64) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_accounts(user_id,created_at,updated_at) VALUES(?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE user_id=user_id`, uid)
	return pointsDBError("确保账务账户", err)
}
func (t *pointsTx) LockAccount(ctx context.Context, uid int64) (points.Account, error) {
	a := points.Account{UserID: uid}
	err := t.tx.QueryRowContext(ctx, `SELECT avail,frozen,debt FROM tm_points_accounts WHERE user_id=? FOR UPDATE`, uid).Scan(&a.Avail, &a.Frozen, &a.Debt)
	return a, pointsDBError("锁定账务账户", err)
}
func (t *pointsTx) FindTransaction(ctx context.Context, key string) (points.Transaction, error) {
	return scanPointsTransaction(t.tx.QueryRowContext(ctx, `SELECT `+pointsTransactionColumns+` FROM tm_points_txns WHERE idem_key=? FOR UPDATE`, key))
}

const pointsTransactionColumns = `id,user_id,idem_key,request_hash,kind,biz_type,biz_id,reverses_id`

func scanPointsTransaction(s pointsScanner) (points.Transaction, error) {
	var out points.Transaction
	var reversal sql.NullInt64
	err := s.Scan(&out.ID, &out.UserID, &out.Key, &out.Hash, &out.Kind, &out.BizType, &out.BizID, &reversal)
	if reversal.Valid {
		out.ReversesID = reversal.Int64
	}
	return out, pointsDBError("读取账务流水", err)
}
func (t *pointsTx) FindTransactionByID(ctx context.Context, uid, id int64) (points.Transaction, error) {
	return scanPointsTransaction(t.tx.QueryRowContext(ctx, `SELECT `+pointsTransactionColumns+` FROM tm_points_txns WHERE id=? AND user_id=? FOR UPDATE`, id, uid))
}
func (t *pointsTx) InsertTransaction(ctx context.Context, v points.Transaction) (int64, error) {
	var reversal any
	if v.ReversesID != 0 {
		reversal = v.ReversesID
	}
	r, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,reverses_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.Key, v.Hash, v.Kind, v.BizType, v.BizID, reversal)
	if err != nil {
		return 0, pointsDBError("插入账务流水", err)
	}
	return r.LastInsertId()
}
func onePointsRow(r sql.Result, err error) error {
	if err != nil {
		return pointsDBError("账务条件更新", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取账务影响行数失败: %w", err)
	}
	if n != 1 {
		return points.ErrInvariant
	}
	return nil
}
func (t *pointsTx) SetAccount(ctx context.Context, old, next points.Account) error {
	return onePointsRow(t.tx.ExecContext(ctx, `UPDATE tm_points_accounts SET avail=?,frozen=?,debt=?,updated_at=UTC_TIMESTAMP(3) WHERE user_id=? AND avail=? AND frozen=? AND debt=?`, next.Avail, next.Frozen, next.Debt, old.UserID, old.Avail, old.Frozen, old.Debt))
}

const pointsLotColumns = `id,user_id,grant_txn_id,src,cash_backed,total,remain,frozen,prio,CAST(eff_at AS CHAR),CAST(exp_at AS CHAR)`

type pointsScanner interface{ Scan(...any) error }

func scanPointsLot(s pointsScanner) (points.Lot, error) {
	var v points.Lot
	var expiry sql.NullString
	var effective string
	err := s.Scan(&v.ID, &v.UserID, &v.GrantTxnID, &v.Source, &v.CashBacked, &v.Total, &v.Remain, &v.Frozen, &v.Priority, &effective, &expiry)
	if err != nil {
		return v, pointsDBError("读取积分批次", err)
	}
	v.EffectiveAt, err = parsePointsUTC(effective)
	if err != nil {
		return v, err
	}
	if expiry.Valid {
		date, err := parsePointsUTC(expiry.String)
		if err != nil {
			return v, err
		}
		v.ExpiresAt = &date
	}
	return v, nil
}
func (t *pointsTx) InsertLot(ctx context.Context, v points.Lot) (int64, error) {
	var expiry any
	if v.ExpiresAt != nil {
		expiry = pointsUTC(*v.ExpiresAt)
	}
	r, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_lots(user_id,grant_txn_id,src,cash_backed,total,remain,frozen,prio,eff_at,exp_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.GrantTxnID, v.Source, v.CashBacked, v.Total, v.Remain, v.Frozen, v.Priority, pointsUTC(v.EffectiveAt), expiry)
	if err != nil {
		return 0, pointsDBError("插入积分批次", err)
	}
	return r.LastInsertId()
}
func (t *pointsTx) FindLotByGrant(ctx context.Context, id int64) (points.Lot, error) {
	return scanPointsLot(t.tx.QueryRowContext(ctx, `SELECT `+pointsLotColumns+` FROM tm_points_lots WHERE grant_txn_id=? FOR UPDATE`, id))
}
func (t *pointsTx) LockLot(ctx context.Context, uid, id int64) (points.Lot, error) {
	return scanPointsLot(t.tx.QueryRowContext(ctx, `SELECT `+pointsLotColumns+` FROM tm_points_lots WHERE id=? AND user_id=? FOR UPDATE`, id, uid))
}
func (t *pointsTx) LockUsableLots(ctx context.Context, uid int64, now time.Time) ([]points.Lot, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT `+pointsLotColumns+` FROM tm_points_lots WHERE user_id=? AND remain>frozen AND eff_at<=? AND (exp_at IS NULL OR exp_at>?) ORDER BY prio,exp_at,id FOR UPDATE`, uid, pointsUTC(now), pointsUTC(now))
	if err != nil {
		return nil, pointsDBError("锁定可用积分批次", err)
	}
	defer rows.Close()
	var out []points.Lot
	for rows.Next() {
		v, err := scanPointsLot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, pointsDBError("遍历积分批次", rows.Err())
}
func (t *pointsTx) SetLot(ctx context.Context, old, next points.Lot) error {
	return onePointsRow(t.tx.ExecContext(ctx, `UPDATE tm_points_lots SET remain=?,frozen=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND user_id=? AND remain=? AND frozen=?`, next.Remain, next.Frozen, old.ID, old.UserID, old.Remain, old.Frozen))
}
func (t *pointsTx) InsertEntry(ctx context.Context, e points.Entry) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta,created_at,updated_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, e.UserID, e.TxnID, e.LotID, e.Bucket, e.Delta)
	return pointsDBError("插入积分分录", err)
}

const pointsHoldColumns = `id,user_id,biz_type,biz_id,freeze_txn_id,terminal_txn_id,amount,captured,price_ver,state,CAST(expire_at AS CHAR)`

func scanPointsHold(s pointsScanner) (points.Hold, error) {
	var v points.Hold
	var terminal sql.NullInt64
	var expiry string
	err := s.Scan(&v.ID, &v.UserID, &v.BizType, &v.BizID, &v.FreezeTxnID, &terminal, &v.Amount, &v.Captured, &v.PriceVersion, &v.State, &expiry)
	if err != nil {
		return v, pointsDBError("读取积分冻结单", err)
	}
	v.ExpiresAt, err = parsePointsUTC(expiry)
	if err != nil {
		return v, err
	}
	if terminal.Valid {
		v.TerminalTxnID = terminal.Int64
	}
	return v, nil
}
func (t *pointsTx) InsertHold(ctx context.Context, v points.Hold) (int64, error) {
	r, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_holds(user_id,biz_type,biz_id,freeze_txn_id,amount,price_ver,expire_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, v.UserID, v.BizType, v.BizID, v.FreezeTxnID, v.Amount, v.PriceVersion, pointsUTC(v.ExpiresAt))
	if err != nil {
		return 0, pointsDBError("插入积分冻结单", err)
	}
	return r.LastInsertId()
}
func (t *pointsTx) LockHold(ctx context.Context, uid, id int64) (points.Hold, error) {
	return scanPointsHold(t.tx.QueryRowContext(ctx, `SELECT `+pointsHoldColumns+` FROM tm_points_holds WHERE id=? AND user_id=? FOR UPDATE`, id, uid))
}
func (t *pointsTx) FindHoldByTxn(ctx context.Context, id int64, terminal bool) (points.Hold, error) {
	if terminal {
		return scanPointsHold(t.tx.QueryRowContext(ctx, `SELECT `+pointsHoldColumns+` FROM tm_points_holds WHERE terminal_txn_id=? FOR UPDATE`, id))
	}
	return scanPointsHold(t.tx.QueryRowContext(ctx, `SELECT `+pointsHoldColumns+` FROM tm_points_holds WHERE freeze_txn_id=? FOR UPDATE`, id))
}
func (t *pointsTx) SetHold(ctx context.Context, old, next points.Hold) error {
	return onePointsRow(t.tx.ExecContext(ctx, `UPDATE tm_points_holds SET state=?,captured=?,terminal_txn_id=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND user_id=? AND state=? AND terminal_txn_id IS NULL`, next.State, next.Captured, next.TerminalTxnID, old.ID, old.UserID, old.State))
}
func (t *pointsTx) InsertAllocation(ctx context.Context, a points.Allocation) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_points_hold_allocs(hold_id,lot_id,user_id,amount,created_at,updated_at) VALUES(?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, a.HoldID, a.LotID, a.UserID, a.Amount)
	return pointsDBError("插入冻结分摊", err)
}
func (t *pointsTx) ListAllocations(ctx context.Context, id int64) ([]points.Allocation, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT hold_id,lot_id,user_id,amount FROM tm_points_hold_allocs WHERE hold_id=? ORDER BY lot_id FOR UPDATE`, id)
	if err != nil {
		return nil, pointsDBError("读取冻结分摊", err)
	}
	defer rows.Close()
	var out []points.Allocation
	for rows.Next() {
		var a points.Allocation
		if err := rows.Scan(&a.HoldID, &a.LotID, &a.UserID, &a.Amount); err != nil {
			return nil, pointsDBError("读取冻结分摊", err)
		}
		out = append(out, a)
	}
	return out, pointsDBError("遍历冻结分摊", rows.Err())
}

func pointsReadRows(ctx context.Context, tx *sql.Tx, query string, uid int64, scan func(*sql.Rows) error) error {
	rows, err := tx.QueryContext(ctx, query, uid)
	if err != nil {
		return pointsDBError("读取账本对账数据", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return pointsDBError("扫描账本对账数据", err)
		}
	}
	return pointsDBError("遍历账本对账数据", rows.Err())
}
func (t *pointsTx) ReadAudit(ctx context.Context, uid int64) (points.AuditRows, error) {
	var out points.AuditRows
	var err error
	out.Account, err = t.LockAccount(ctx, uid)
	if err != nil {
		return out, err
	}
	queries := []struct {
		query string
		scan  func(*sql.Rows) error
	}{
		{`SELECT ` + pointsLotColumns + ` FROM tm_points_lots FORCE INDEX (idx_tm_points_lot_use) WHERE user_id=? ORDER BY id FOR UPDATE`, func(rows *sql.Rows) error {
			v, err := scanPointsLot(rows)
			if err == nil {
				out.Lots = append(out.Lots, v)
			}
			return err
		}},
		{`SELECT txn_id,lot_id,user_id,bucket,delta FROM tm_points_entries FORCE INDEX (idx_tm_points_entry_user) WHERE user_id=? ORDER BY id FOR UPDATE`, func(rows *sql.Rows) error {
			var v points.Entry
			err := rows.Scan(&v.TxnID, &v.LotID, &v.UserID, &v.Bucket, &v.Delta)
			if err == nil {
				out.Entries = append(out.Entries, v)
			}
			return err
		}},
		{`SELECT ` + pointsHoldColumns + ` FROM tm_points_holds FORCE INDEX (idx_tm_points_hold_user) WHERE user_id=? ORDER BY id FOR UPDATE`, func(rows *sql.Rows) error {
			v, err := scanPointsHold(rows)
			if err == nil {
				out.Holds = append(out.Holds, v)
			}
			return err
		}},
		{`SELECT hold_id,lot_id,user_id,amount FROM tm_points_hold_allocs FORCE INDEX (idx_tm_points_hold_alloc_user) WHERE user_id=? ORDER BY hold_id,lot_id FOR UPDATE`, func(rows *sql.Rows) error {
			var v points.Allocation
			err := rows.Scan(&v.HoldID, &v.LotID, &v.UserID, &v.Amount)
			if err == nil {
				out.Allocations = append(out.Allocations, v)
			}
			return err
		}},
		{`SELECT ` + pointsTransactionColumns + ` FROM tm_points_txns FORCE INDEX (idx_tm_points_txn_history) WHERE user_id=? ORDER BY id FOR UPDATE`, func(rows *sql.Rows) error {
			v, err := scanPointsTransaction(rows)
			if err == nil {
				out.Transactions = append(out.Transactions, v)
			}
			return err
		}},
		{`SELECT id,user_id,reversal_txn_id,amount,outstanding FROM tm_points_debts FORCE INDEX (idx_tm_points_debt_user) WHERE user_id=? ORDER BY id FOR UPDATE`, func(rows *sql.Rows) error {
			var v points.Debt
			err := rows.Scan(&v.ID, &v.UserID, &v.ReversalTxnID, &v.Amount, &v.Outstanding)
			if err == nil {
				out.Debts = append(out.Debts, v)
			}
			return err
		}},
		{`SELECT txn_id,debt_id,lot_id,user_id,amount FROM tm_points_debt_allocs FORCE INDEX (idx_tm_points_debt_alloc_user) WHERE user_id=? ORDER BY txn_id,debt_id,lot_id FOR UPDATE`, func(rows *sql.Rows) error {
			var v points.DebtAllocation
			err := rows.Scan(&v.TxnID, &v.DebtID, &v.LotID, &v.UserID, &v.Amount)
			if err == nil {
				out.DebtAllocations = append(out.DebtAllocations, v)
			}
			return err
		}},
	}
	for _, q := range queries {
		if err := pointsReadRows(ctx, t.tx, q.query, uid, q.scan); err != nil {
			return out, err
		}
	}
	return out, nil
}
