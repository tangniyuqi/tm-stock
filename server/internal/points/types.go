// Package points 定义专用积分账本契约，不含 HTTP、旧积分或支付接入。
package points

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"
)

var (
	ErrInvalid      = fmt.Errorf("账务请求无效")
	ErrNotFound     = fmt.Errorf("账务记录不存在")
	ErrIdempotency  = fmt.Errorf("账务幂等键与原请求不一致")
	ErrConflict     = fmt.Errorf("账务业务编号冲突")
	ErrInsufficient = fmt.Errorf("可用积分不足")
	ErrPriceChanged = fmt.Errorf("价格版本已变化")
	ErrClosed       = fmt.Errorf("冻结单已经终结")
	ErrExpired      = fmt.Errorf("冻结单已到期")
	ErrOverflow     = fmt.Errorf("积分金额超出整数范围")
	ErrInvariant    = fmt.Errorf("账本恒等式不成立")
)

const (
	Purchase     = 1
	Gift         = 2
	Contribution = 3
	Signin       = 4
	Available    = 1
	Frozen       = 2
	Holding      = 1
	Settled      = 2
	Released     = 3
	Expired      = 4
)

type Account struct{ UserID, Avail, Frozen, Debt int64 }
type Transaction struct {
	ID, UserID, ReversesID          int64
	Key, Hash, Kind, BizType, BizID string
}
type Lot struct {
	ID, UserID, GrantTxnID, Total, Remain, Frozen int64
	Source                                        int
	CashBacked                                    bool
	Priority                                      int16
	EffectiveAt                                   time.Time
	ExpiresAt                                     *time.Time
}
type Hold struct {
	ID, UserID, FreezeTxnID, TerminalTxnID, Amount, Captured int64
	BizType, BizID, PriceVersion                             string
	State                                                    int
	ExpiresAt                                                time.Time
}
type Allocation struct{ HoldID, LotID, UserID, Amount int64 }
type Debt struct{ ID, UserID, ReversalTxnID, Amount, Outstanding int64 }
type DebtAllocation struct{ TxnID, DebtID, LotID, UserID, Amount int64 }
type Entry struct {
	TxnID, LotID, UserID, Delta int64
	Bucket                      int
}

// Receipt 只含首次操作的不可变结果，不返回重放时已变化的账户/批次余额。
type Receipt struct {
	TxnID, UserID, LotID, HoldID, Amount int64
	Kind, PriceVersion                   string
}
type GrantRequest struct {
	UserID      int64
	Key, BizID  string
	Amount      int64
	Source      int
	Priority    int16
	EffectiveAt time.Time
	ExpiresAt   *time.Time
}
type FreezeRequest struct {
	UserID                            int64
	Key, BizType, BizID, PriceVersion string
	ExpiresAt                         time.Time
}

// Price 由服务端价格簿提供，不接受 C 端自行填积分数量。
type Price struct {
	Version string
	Points  int64
}
type FinalizeRequest struct {
	UserID int64
	Key    string
	HoldID int64
	State  int
}

// ReverseRequest 只允许整笔冲正原始入账，不接受客户端自填回收额或改写历史流水。
type ReverseRequest struct {
	UserID, GrantTxnID int64
	Key, BizID         string
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

func validID(value string, max int) bool {
	return len(value) > 0 && len(value) <= max && identifierPattern.MatchString(value)
}
func ValidKey(key string) bool { return keyPattern.MatchString(key) }
func validTime(t time.Time) bool {
	return !t.IsZero() && t.Year() >= 1000 && t.Year() <= 9999 && t.Nanosecond()%int(time.Millisecond) == 0
}
func (r GrantRequest) Validate() error {
	if r.UserID <= 0 || !ValidKey(r.Key) || !validID(r.BizID, 64) || r.Amount <= 0 || r.Source < Purchase || r.Source > Signin || !validTime(r.EffectiveAt) || (r.ExpiresAt != nil && (!validTime(*r.ExpiresAt) || !r.ExpiresAt.After(r.EffectiveAt))) {
		return ErrInvalid
	}
	// 购买批次有效期仍待法务/产品确认，当前拒绝设置，不偷偷选择未批准政策。
	if r.Source == Purchase && r.ExpiresAt != nil {
		return ErrInvalid
	}
	return nil
}
func (r FreezeRequest) Validate() error {
	if r.UserID <= 0 || !ValidKey(r.Key) || (r.BizType != "ai_task" && r.BizType != "content_unlock") || !validID(r.BizID, 64) || !validID(r.PriceVersion, 32) || !validTime(r.ExpiresAt) {
		return ErrInvalid
	}
	return nil
}
func (r FinalizeRequest) Validate() error {
	if r.UserID <= 0 || !ValidKey(r.Key) || r.HoldID <= 0 || (r.State != Settled && r.State != Released && r.State != Expired) {
		return ErrInvalid
	}
	return nil
}
func (r ReverseRequest) Validate() error {
	if r.UserID <= 0 || r.GrantTxnID <= 0 || !ValidKey(r.Key) || !validID(r.BizID, 64) {
		return ErrInvalid
	}
	return nil
}
func (p Price) Validate() error {
	if !validID(p.Version, 32) || p.Points <= 0 {
		return ErrInvalid
	}
	return nil
}

// RequestHash 的显式操作域防止不同类型的同形请求发生幂等混淆。
func RequestHash(kind string, request any) (string, error) {
	b, err := json.Marshal(struct {
		Version int
		Kind    string
		Request any
	}{1, kind, request})
	if err != nil {
		return "", fmt.Errorf("编码账务请求摘要失败: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func AddNonnegative(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return 0, ErrOverflow
	}
	return a + b, nil
}

// Store 定义事务边界，业务服务决定何时执行及提交；仓储仅实现数据访问。
type Store interface {
	WithinPointsTx(context.Context, func(Tx) error) error
}

// Tx 可由任务仓储在同一 *sql.Tx 上装配；方法不自行提交或开启嵌套事务。
type Tx interface {
	Now(context.Context) (time.Time, error)
	EnsureAccount(context.Context, int64) error
	LockAccount(context.Context, int64) (Account, error)
	FindTransaction(context.Context, string) (Transaction, error)
	FindTransactionByID(context.Context, int64, int64) (Transaction, error)
	InsertTransaction(context.Context, Transaction) (int64, error)
	SetAccount(context.Context, Account, Account) error
	InsertLot(context.Context, Lot) (int64, error)
	FindLotByGrant(context.Context, int64) (Lot, error)
	LockUsableLots(context.Context, int64, time.Time) ([]Lot, error)
	LockLot(context.Context, int64, int64) (Lot, error)
	ReadLotExpired(context.Context, int64, int64) (int64, error)
	SetLot(context.Context, Lot, Lot) error
	InsertEntry(context.Context, Entry) error
	InsertHold(context.Context, Hold) (int64, error)
	LockHold(context.Context, int64, int64) (Hold, error)
	FindHoldByTxn(context.Context, int64, bool) (Hold, error)
	SetHold(context.Context, Hold, Hold) error
	InsertAllocation(context.Context, Allocation) error
	ListAllocations(context.Context, int64) ([]Allocation, error)
	LockOutstandingDebts(context.Context, int64) ([]Debt, error)
	InsertDebt(context.Context, Debt) (int64, error)
	SetDebt(context.Context, Debt, Debt) error
	InsertDebtAllocation(context.Context, DebtAllocation) error
	ReadAudit(context.Context, int64) (AuditRows, error)
}

type AuditRows struct {
	Account         Account
	Lots            []Lot
	Entries         []Entry
	Holds           []Hold
	Allocations     []Allocation
	Transactions    []Transaction
	Debts           []Debt
	DebtAllocations []DebtAllocation
}
type AuditIssue struct {
	Code     string `json:"code"`
	RecordID int64  `json:"record_id"`
}
type AuditReport struct {
	UserID int64
	Issues []AuditIssue
}

// AuditScanner 使用固定用户上界和主键游标分页，不以 OFFSET 扫描不断增长的账户表。
// 每个账户独立对账；整轮不是跨用户的同一时点快照，新增账户由下一轮覆盖。
type AuditScanner interface {
	AuditUpperBound(context.Context) (int64, error)
	ListAuditUsers(ctx context.Context, after, upper int64, limit int) ([]int64, error)
}

type AuditSweep struct {
	UpperUserID int64 `json:"upper_user_id"`
	LastUserID  int64 `json:"last_user_id"`
	Checked     int64 `json:"checked"`
	Unhealthy   int64 `json:"unhealthy"`
	Issues      int64 `json:"issues"`
	Complete    bool  `json:"complete"`
}

func (a AuditReport) Healthy() bool { return len(a.Issues) == 0 }
