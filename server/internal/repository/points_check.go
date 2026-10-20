package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
)

// PointsSchemaVerifier 只核对专用账本，不读取 GVA 旧积分，不运行迁移。
// 启用账务服务的启动路径必须先调用 Verify；当前没有公开收费入口。
type PointsSchemaVerifier struct{ db *sql.DB }

func NewPointsSchemaVerifier(db *sql.DB) *PointsSchemaVerifier { return &PointsSchemaVerifier{db: db} }

var mysqlLedgerVersion = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:[-+.]|$)`)

func validLedgerMySQLVersion(version, comment string) bool {
	if !strings.Contains(strings.ToLower(comment), "mysql") || strings.Contains(strings.ToLower(version+comment), "mariadb") {
		return false
	}
	match := mysqlLedgerVersion.FindStringSubmatch(version)
	if len(match) != 4 {
		return false
	}
	parts := [3]int{}
	for i := range parts {
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return false
		}
		parts[i] = n
	}
	return parts[0] > 8 || (parts[0] == 8 && (parts[1] > 0 || (parts[1] == 0 && parts[2] >= 16)))
}

// Verify 的业务表探针全部在单个事务内回滚。正例失败或非预期错误同样拒绝启动，
// 不把权限错误、外键错误误判成 CHECK 真正执行。会消耗自增编号，不承诺编号连续。
func (v *PointsSchemaVerifier) Verify(ctx context.Context) error {
	if v == nil || v.db == nil {
		return fmt.Errorf("账本自检数据库未初始化")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("开启账本约束探针失败: %w", err)
	}
	// 各种失败路径都不提交探针数据；database/sql 在上下文取消时也负责回滚。
	defer func() { _ = tx.Rollback() }()
	if err := verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	if err := verifyPointsSchemaMetadata(ctx, tx); err != nil {
		return err
	}
	if err := probePointsConstraints(ctx, tx); err != nil {
		return err
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("回滚账本探针失败: %w", err)
	}
	return nil
}

// 任务核与账本探针必须验证实际执行写入的同一个连接，不能先检查另一个连接。
func verifyPointsSession(ctx context.Context, tx *sql.Tx) error {
	var version, comment, mode string
	var foreignKeys int
	if err := tx.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment, @@SESSION.foreign_key_checks, @@SESSION.sql_mode").Scan(&version, &comment, &foreignKeys, &mode); err != nil {
		return fmt.Errorf("读取账本数据库版本失败: %w", err)
	}
	if !validLedgerMySQLVersion(version, comment) {
		return fmt.Errorf("账本要求 MySQL 8.0.16 或更新版本")
	}
	strict := false
	for _, item := range strings.Split(mode, ",") {
		if item == "STRICT_TRANS_TABLES" || item == "STRICT_ALL_TABLES" {
			strict = true
		}
	}
	if foreignKeys != 1 || !strict {
		return fmt.Errorf("账本连接必须启用外键检查和严格 SQL 模式")
	}
	return nil
}

type pointsTableContract struct {
	name, check string
	immutable   bool
}

var pointsTables = []pointsTableContract{
	{"tm_points_accounts", "chk_tm_points_account", false},
	{"tm_points_txns", "chk_tm_points_txn", true},
	{"tm_points_lots", "chk_tm_points_lot", false},
	{"tm_points_entries", "chk_tm_points_entry", true},
	{"tm_points_holds", "chk_tm_points_hold", false},
	{"tm_points_hold_allocs", "chk_tm_points_alloc", true},
	{"tm_points_debts", "chk_tm_points_debt", false},
	{"tm_points_debt_allocs", "chk_tm_points_debt_alloc", true},
}

func verifyPointsSchemaMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, table := range pointsTables {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=? AND engine='InnoDB'`, table.name).Scan(&count); err != nil {
			return fmt.Errorf("核对账本表引擎失败: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("账本表 %s 缺失或不是 InnoDB", table.name)
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=? AND constraint_type='CHECK' AND enforced='YES'`, table.name, table.check).Scan(&count); err != nil {
			return fmt.Errorf("核对账本 CHECK 失败: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("账本表 %s 缺少强制执行的 CHECK", table.name)
		}
	}
	// information_schema.triggers 的可见性需要 TRIGGER 权限，业务账号不应有改触发器权限。
	// 保护行为由下方真实拒写探针核对；部署账号另在迁移脚本里核对触发器数量。
	// 金额不能由未知旧表换成浮点/可空列；SQL 自检不替代服务层溢出检查。
	for _, column := range []struct{ table, column string }{
		{"tm_points_accounts", "avail"}, {"tm_points_accounts", "frozen"}, {"tm_points_accounts", "debt"},
		{"tm_points_lots", "total"}, {"tm_points_lots", "remain"}, {"tm_points_lots", "frozen"},
		{"tm_points_entries", "delta"}, {"tm_points_holds", "amount"}, {"tm_points_holds", "captured"},
		{"tm_points_hold_allocs", "amount"}, {"tm_points_debts", "amount"}, {"tm_points_debts", "outstanding"}, {"tm_points_debt_allocs", "amount"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND data_type='bigint' AND column_type NOT LIKE '%unsigned%' AND is_nullable='NO'`, column.table, column.column).Scan(&count); err != nil {
			return fmt.Errorf("核对账本整数金额失败: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("账本金额 %s.%s 不是非空有符号 BIGINT", column.table, column.column)
		}
	}
	for _, fk := range []struct{ table, name, target, columns string }{
		{"tm_points_txns", "fk_tm_points_txn_account", "tm_points_accounts", "user_id:user_id"},
		{"tm_points_txns", "fk_tm_points_txn_reversal", "tm_points_txns", "reverses_id:id,user_id:user_id"},
		{"tm_points_lots", "fk_tm_points_lot_account", "tm_points_accounts", "user_id:user_id"},
		{"tm_points_lots", "fk_tm_points_lot_grant", "tm_points_txns", "grant_txn_id:id,user_id:user_id"},
		{"tm_points_entries", "fk_tm_points_entry_txn", "tm_points_txns", "txn_id:id,user_id:user_id"},
		{"tm_points_entries", "fk_tm_points_entry_lot", "tm_points_lots", "lot_id:id,user_id:user_id"},
		{"tm_points_holds", "fk_tm_points_hold_account", "tm_points_accounts", "user_id:user_id"},
		{"tm_points_holds", "fk_tm_points_hold_freeze", "tm_points_txns", "freeze_txn_id:id,user_id:user_id"},
		{"tm_points_holds", "fk_tm_points_hold_terminal", "tm_points_txns", "terminal_txn_id:id,user_id:user_id"},
		{"tm_points_hold_allocs", "fk_tm_points_alloc_hold", "tm_points_holds", "hold_id:id,user_id:user_id"},
		{"tm_points_hold_allocs", "fk_tm_points_alloc_lot", "tm_points_lots", "lot_id:id,user_id:user_id"},
		{"tm_points_debts", "fk_tm_points_debt_account", "tm_points_accounts", "user_id:user_id"},
		{"tm_points_debts", "fk_tm_points_debt_reversal", "tm_points_txns", "reversal_txn_id:id,user_id:user_id"},
		{"tm_points_debt_allocs", "fk_tm_points_debt_alloc_txn", "tm_points_txns", "txn_id:id,user_id:user_id"},
		{"tm_points_debt_allocs", "fk_tm_points_debt_alloc_debt", "tm_points_debts", "debt_id:id,user_id:user_id"},
		{"tm_points_debt_allocs", "fk_tm_points_debt_alloc_lot", "tm_points_lots", "lot_id:id,user_id:user_id"},
	} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(column_name,':',referenced_column_name) ORDER BY ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=? AND referenced_table_schema=DATABASE() AND referenced_table_name=?`, fk.table, fk.name, fk.target).Scan(&actual); err != nil {
			return fmt.Errorf("核对账本归属外键失败: %w", err)
		}
		if actual != fk.columns {
			return fmt.Errorf("账本外键 %s 缺失或归属列不一致", fk.name)
		}
	}
	for _, index := range []struct{ table, name, columns string }{
		{"tm_points_txns", "uk_tm_points_txn_idem", "idem_key"},
		{"tm_points_txns", "uk_tm_points_txn_business", "kind,biz_type,biz_id"},
		{"tm_points_txns", "uk_tm_points_txn_reverses", "reverses_id"},
		{"tm_points_lots", "uk_tm_points_lot_grant", "grant_txn_id"},
		{"tm_points_entries", "uk_tm_points_entry", "txn_id,lot_id,bucket"},
		{"tm_points_holds", "uk_tm_points_hold_biz", "biz_type,biz_id"},
		{"tm_points_holds", "uk_tm_points_hold_freeze", "freeze_txn_id"},
		{"tm_points_holds", "uk_tm_points_hold_terminal", "terminal_txn_id"},
		{"tm_points_hold_allocs", "PRIMARY", "hold_id,lot_id"},
		{"tm_points_debts", "uk_tm_points_debt_reversal", "reversal_txn_id"},
		{"tm_points_debt_allocs", "PRIMARY", "txn_id,debt_id,lot_id"},
	} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=0 AND sub_part IS NULL`, index.table, index.name).Scan(&actual); err != nil {
			return fmt.Errorf("核对账本唯一索引失败: %w", err)
		}
		if actual != index.columns {
			return fmt.Errorf("账本唯一索引 %s 缺失或被削弱", index.name)
		}
	}
	// 当前读必须限定用户索引范围，不能让分摊表在持锁对账时退化为全表扫描。
	for _, index := range []struct{ table, name, columns string }{
		{"tm_points_txns", "idx_tm_points_txn_history", "user_id,id"},
		{"tm_points_lots", "idx_tm_points_lot_use", "user_id,prio,exp_at,id"},
		{"tm_points_entries", "idx_tm_points_entry_user", "user_id,id"},
		{"tm_points_holds", "idx_tm_points_hold_user", "user_id,state,id"},
		{"tm_points_hold_allocs", "idx_tm_points_hold_alloc_user", "user_id,hold_id,lot_id"},
		{"tm_points_debts", "idx_tm_points_debt_user", "user_id,id"},
		{"tm_points_debt_allocs", "idx_tm_points_debt_alloc_user", "user_id,txn_id,debt_id,lot_id"},
	} {
		var actual string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=1 AND sub_part IS NULL AND is_visible='YES'`, index.table, index.name).Scan(&actual); err != nil {
			return fmt.Errorf("核对账本用户范围索引失败: %w", err)
		}
		if actual != index.columns {
			return fmt.Errorf("账本用户范围索引 %s 缺失或不可用", index.name)
		}
	}
	return nil
}

func pointsProbeError(err error, number uint16) bool {
	var me *mysqldrv.MySQLError
	return errors.As(err, &me) && me.Number == number
}

func probePointsConstraints(ctx context.Context, tx *sql.Tx) error {
	// 用随机高位会员编号做探针；只 INSERT，不覆盖既有账户。碰撞或任何异常都失败关闭。
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("生成账本探针编号失败: %w", err)
	}
	uid := int64(binary.BigEndian.Uint64(random[:])&((1<<62)-2)) | (1 << 62)
	// datetime 不做时区转换，显式写 UTC，不依赖连接的 CURRENT_TIMESTAMP 默认时区。
	if _, err := tx.ExecContext(ctx, "INSERT INTO tm_points_accounts(user_id,created_at,updated_at) VALUES(?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)),(?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))", uid, uid+1); err != nil {
		return fmt.Errorf("写入账本正例探针失败: %w", err)
	}
	prefix := fmt.Sprintf("probe_%x_", uid)
	hash := strings.Repeat("0", 64)
	insertTxn := func(key, kind, biz string, reverses any) (int64, error) {
		result, err := tx.ExecContext(ctx, `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,reverses_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, prefix+key, hash, kind, biz, prefix+key, reverses)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}
	grant, err := insertTxn("grant", "grant", "points_grant", nil)
	if err != nil {
		return fmt.Errorf("写入账本入账探针失败: %w", err)
	}
	freeze, err := insertTxn("freeze", "freeze", "ai_task", nil)
	if err != nil {
		return fmt.Errorf("写入账本冻结探针失败: %w", err)
	}
	release, err := insertTxn("release", "release", "ai_task", nil)
	if err != nil {
		return fmt.Errorf("写入账本释放探针失败: %w", err)
	}
	reversal, err := insertTxn("reversal", "reversal", "points_refund", grant)
	if err != nil {
		return fmt.Errorf("写入账本冲正探针失败: %w", err)
	}
	offset, err := insertTxn("offset", "debt_offset", "debt_offset", nil)
	if err != nil {
		return fmt.Errorf("写入账本抵扣探针失败: %w", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO tm_points_lots(user_id,grant_txn_id,src,cash_backed,total,remain,frozen,eff_at,created_at,updated_at) VALUES(?,?,2,0,10,10,10,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, grant)
	if err != nil {
		return fmt.Errorf("写入账本批次探针失败: %w", err)
	}
	lot, err := result.LastInsertId()
	if err != nil {
		return err
	}
	spareGrant, err := insertTxn("spare_grant", "grant", "points_grant", nil)
	if err != nil {
		return fmt.Errorf("写入账本额外批次入账探针失败: %w", err)
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO tm_points_lots(user_id,grant_txn_id,src,cash_backed,total,remain,eff_at,created_at,updated_at) VALUES(?,?,2,0,10,10,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, spareGrant)
	if err != nil {
		return fmt.Errorf("写入账本额外批次探针失败: %w", err)
	}
	spareLot, err := result.LastInsertId()
	if err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO tm_points_holds(user_id,biz_type,biz_id,freeze_txn_id,amount,price_ver,expire_at,created_at,updated_at) VALUES(?,'ai_task',?, ?,10,'probe',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, prefix+"task", freeze)
	if err != nil {
		return fmt.Errorf("写入账本冻结单探针失败: %w", err)
	}
	hold, err := result.LastInsertId()
	if err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO tm_points_debts(user_id,reversal_txn_id,amount,outstanding,created_at,updated_at) VALUES(?,?,5,5,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, uid, reversal)
	if err != nil {
		return fmt.Errorf("写入账本欠款探针失败: %w", err)
	}
	debt, err := result.LastInsertId()
	if err != nil {
		return err
	}
	positives := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta,created_at,updated_at) VALUES(?,?,?,1,10,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, []any{uid, grant, lot}},
		{`INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta,created_at,updated_at) VALUES(?,?,?,1,-10,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)),(?,?,?,2,10,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, []any{uid, freeze, lot, uid, freeze, lot}},
		{`INSERT INTO tm_points_hold_allocs(user_id,hold_id,lot_id,amount,created_at,updated_at) VALUES(?,?,?,10,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, []any{uid, hold, lot}},
		{`INSERT INTO tm_points_debt_allocs(user_id,txn_id,debt_id,lot_id,amount,created_at,updated_at) VALUES(?,?,?,?,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, []any{uid, offset, debt, lot}},
	}
	for _, p := range positives {
		if _, err := tx.ExecContext(ctx, p.query, p.args...); err != nil {
			return fmt.Errorf("写入账本分录正例失败: %w", err)
		}
	}
	negatives := []struct {
		name, query string
		args        []any
		code        uint16
	}{
		{"账户可用余额", `UPDATE tm_points_accounts SET avail=-1 WHERE user_id=?`, []any{uid}, 3819},
		{"账户冻结余额", `UPDATE tm_points_accounts SET frozen=-1 WHERE user_id=?`, []any{uid}, 3819},
		{"账户欠款", `UPDATE tm_points_accounts SET debt=-1 WHERE user_id=?`, []any{uid}, 3819},
		{"批次剩余", `UPDATE tm_points_lots SET remain=-1 WHERE id=?`, []any{lot}, 3819},
		{"批次超发", `UPDATE tm_points_lots SET remain=11 WHERE id=?`, []any{lot}, 3819},
		{"批次冻结", `UPDATE tm_points_lots SET frozen=-1 WHERE id=?`, []any{lot}, 3819},
		{"批次超冻", `UPDATE tm_points_lots SET frozen=11 WHERE id=?`, []any{lot}, 3819},
		{"分录零额", `INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta) VALUES(?,?,?,1,0)`, []any{uid, release, lot}, 3819},
		{"分录桶", `INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta) VALUES(?,?,?,3,1)`, []any{uid, release, lot}, 3819},
		{"冻结单终态", `UPDATE tm_points_holds SET state=2,captured=1,terminal_txn_id=? WHERE id=?`, []any{release, hold}, 3819},
		{"冻结单无终态事件", `UPDATE tm_points_holds SET state=3 WHERE id=?`, []any{hold}, 3819},
		{"欠款非负", `UPDATE tm_points_debts SET outstanding=-1 WHERE id=?`, []any{debt}, 3819},
		{"冻结分摊零额", `INSERT INTO tm_points_hold_allocs(user_id,hold_id,lot_id,amount) VALUES(?,?,?,0)`, []any{uid, hold, spareLot}, 3819},
		{"抵扣分摊零额", `INSERT INTO tm_points_debt_allocs(user_id,txn_id,debt_id,lot_id,amount) VALUES(?,?,?,?,0)`, []any{uid, offset, debt, spareLot}, 3819},
		{"流水业务白名单", `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id) VALUES(?,?,?,'freeze','membership','probe')`, []any{uid, prefix + "bad_business", hash}, 3819},
		{"流水空摘要", `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id) VALUES(?,?,'','grant','points_grant','probe')`, []any{uid, prefix + "bad_digest"}, 3819},
		{"流水跨账户引用", `INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta) VALUES(?,?,?,1,1)`, []any{uid + 1, release, lot}, 1452},
		{"流水幂等唯一", `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id) VALUES(?,?,?,'grant','points_grant','probe')`, []any{uid, prefix + "grant", hash}, 1062},
		{"流水业务唯一", `INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id) VALUES(?,?,?,'grant','points_grant',?)`, []any{uid, prefix + "duplicate_business", hash, prefix + "grant"}, 1062},
	}
	for _, p := range negatives {
		_, err := tx.ExecContext(ctx, p.query, p.args...)
		if !pointsProbeError(err, p.code) {
			return fmt.Errorf("账本探针 %s 未按预期约束拒绝: %w", p.name, errOrMissing(err))
		}
	}
	// 不能只相信 information_schema 里的触发器名字；对已有探针行真的试改/删。
	for _, table := range pointsTables {
		key, value := "id", any(grant)
		switch table.name {
		case "tm_points_accounts":
			key, value = "user_id", uid+1
		case "tm_points_lots":
			value = lot
		case "tm_points_entries":
			key, value = "txn_id", grant
		case "tm_points_holds":
			value = hold
		case "tm_points_hold_allocs":
			key, value = "hold_id", hold
		case "tm_points_debts":
			value = debt
		case "tm_points_debt_allocs":
			key, value = "txn_id", offset
		}
		// 表/列名来自上面的封闭代码常量，不含用户输入。
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table.name+" WHERE "+key+"=?", value); !pointsProbeError(err, 1644) {
			return fmt.Errorf("账本表 %s 删除保护失效: %w", table.name, errOrMissing(err))
		}
		if table.immutable {
			if _, err := tx.ExecContext(ctx, "UPDATE "+table.name+" SET updated_at=updated_at WHERE "+key+"=?", value); !pointsProbeError(err, 1644) {
				return fmt.Errorf("账本表 %s 修改保护失效: %w", table.name, errOrMissing(err))
			}
		}
	}
	identityProbes := []struct {
		query string
		args  []any
	}{
		{`UPDATE tm_points_accounts SET created_at=created_at+INTERVAL 1 SECOND WHERE user_id=?`, []any{uid}},
		{`UPDATE tm_points_lots SET total=11 WHERE id=?`, []any{lot}},
		{`UPDATE tm_points_holds SET price_ver='other' WHERE id=?`, []any{hold}},
		{`UPDATE tm_points_debts SET amount=6 WHERE id=?`, []any{debt}},
	}
	for _, p := range identityProbes {
		if _, err := tx.ExecContext(ctx, p.query, p.args...); !pointsProbeError(err, 1644) {
			return fmt.Errorf("账本快照身份保护失效: %w", errOrMissing(err))
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_points_holds SET state=3,terminal_txn_id=? WHERE id=?`, release, hold); err != nil {
		return fmt.Errorf("账本冻结单正例终结失败: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tm_points_holds SET updated_at=updated_at WHERE id=?`, hold); !pointsProbeError(err, 1644) {
		return fmt.Errorf("账本冻结单终态保护失效: %w", errOrMissing(err))
	}
	return nil
}

func errOrMissing(err error) error {
	if err != nil {
		return err
	}
	return errors.New("非法写入意外成功")
}
