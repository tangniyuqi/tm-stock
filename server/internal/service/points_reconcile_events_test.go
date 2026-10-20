package service

import (
	"math"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func pointsAuditDebtFixture() points.AuditRows {
	return points.AuditRows{
		Account: points.Account{UserID: 1, Debt: 2},
		Transactions: []points.Transaction{
			{ID: 1, UserID: 1, Kind: "grant", BizType: "points_grant"},
			{ID: 2, UserID: 1, Kind: "freeze", BizType: "ai_task", BizID: "task:1"},
			{ID: 3, UserID: 1, Kind: "settle", BizType: "ai_task", BizID: "task:1"},
			{ID: 4, UserID: 1, Kind: "reversal", BizType: "points_refund", ReversesID: 1},
			{ID: 5, UserID: 1, Kind: "grant", BizType: "points_grant"},
			{ID: 6, UserID: 1, Kind: "debt_offset", BizType: "debt_offset", BizID: "grant:5"},
		},
		Lots:            []points.Lot{{ID: 1, UserID: 1, GrantTxnID: 1, Source: points.Gift, Total: 20}, {ID: 2, UserID: 1, GrantTxnID: 5, Source: points.Signin, Total: 5}},
		Holds:           []points.Hold{{ID: 1, UserID: 1, FreezeTxnID: 2, TerminalTxnID: 3, Amount: 7, Captured: 7, State: points.Settled, BizType: "ai_task", BizID: "task:1"}},
		Allocations:     []points.Allocation{{HoldID: 1, LotID: 1, UserID: 1, Amount: 7}},
		Entries:         []points.Entry{{TxnID: 1, LotID: 1, UserID: 1, Bucket: points.Available, Delta: 20}, {TxnID: 2, LotID: 1, UserID: 1, Bucket: points.Available, Delta: -7}, {TxnID: 2, LotID: 1, UserID: 1, Bucket: points.Frozen, Delta: 7}, {TxnID: 3, LotID: 1, UserID: 1, Bucket: points.Frozen, Delta: -7}, {TxnID: 4, LotID: 1, UserID: 1, Bucket: points.Available, Delta: -13}, {TxnID: 5, LotID: 2, UserID: 1, Bucket: points.Available, Delta: 5}, {TxnID: 6, LotID: 2, UserID: 1, Bucket: points.Available, Delta: -5}},
		Debts:           []points.Debt{{ID: 1, UserID: 1, ReversalTxnID: 4, Amount: 7, Outstanding: 2}},
		DebtAllocations: []points.DebtAllocation{{TxnID: 6, DebtID: 1, LotID: 2, UserID: 1, Amount: 5}},
	}
}

func TestPointsReconcileEventSemanticsAndDebtPartitions(t *testing.T) {
	if report := ReconcilePoints(pointsAuditDebtFixture()); !report.Healthy() {
		t.Fatal("完整事件正例失败", report)
	}
	for _, tc := range []struct {
		name, code string
		change     func(*points.AuditRows)
	}{
		{"账户身份", "account_snapshot", func(r *points.AuditRows) { r.Account.UserID = 0 }},
		{"流水归属", "txn_owner", func(r *points.AuditRows) { r.Transactions[0].UserID = 2 }},
		{"流水重复", "txn_duplicate", func(r *points.AuditRows) { r.Transactions = append(r.Transactions, r.Transactions[0]) }},
		{"未知流水", "txn_kind", func(r *points.AuditRows) { r.Transactions[0].Kind = "unknown" }},
		{"入账业务", "grant_event", func(r *points.AuditRows) { r.Transactions[0].BizType = "ai_task" }},
		{"冲正关联", "reversal_event", func(r *points.AuditRows) { r.Transactions[3].ReversesID = 2 }},
		{"冲正业务", "reversal_event", func(r *points.AuditRows) { r.Transactions[3].BizType = "ai_task" }},
		{"非冲正关联", "txn_reverses", func(r *points.AuditRows) { r.Transactions[0].ReversesID = 1 }},
		{"重复冲正", "reversal_event", func(r *points.AuditRows) {
			x := r.Transactions[3]
			x.ID = 7
			r.Transactions = append(r.Transactions, x)
		}},
		{"超额欠款", "reversal_amount", func(r *points.AuditRows) { r.Debts[0].Amount = math.MaxInt64 }},
		{"批次来源", "lot_source", func(r *points.AuditRows) { r.Lots[1].Source = 9 }},
		{"批次购买属性", "lot_source", func(r *points.AuditRows) { r.Lots[1].CashBacked = true }},
		{"批次入账事件", "lot_origin", func(r *points.AuditRows) { r.Lots[1].GrantTxnID = 2 }},
		{"重复批次入账", "grant_duplicate", func(r *points.AuditRows) { r.Lots[1].GrantTxnID = 1 }},
		{"缺少批次", "grant_event", func(r *points.AuditRows) { r.Lots = r.Lots[:1] }},
		{"冻结流水", "hold_freeze_event", func(r *points.AuditRows) { r.Holds[0].FreezeTxnID = 3 }},
		{"终态流水", "hold_terminal_event", func(r *points.AuditRows) { r.Holds[0].TerminalTxnID = 2 }},
		{"重复冻结事件", "hold_freeze_duplicate", func(r *points.AuditRows) { r.Holds = append(r.Holds, r.Holds[0]) }},
		{"重复终态事件", "hold_terminal_duplicate", func(r *points.AuditRows) { r.Holds = append(r.Holds, r.Holds[0]) }},
		{"空冻结事件", "freeze_event", func(r *points.AuditRows) {
			r.Transactions = append(r.Transactions, points.Transaction{ID: 99, UserID: 1, Kind: "freeze"})
		}},
		{"空终态事件", "terminal_event", func(r *points.AuditRows) {
			r.Transactions = append(r.Transactions, points.Transaction{ID: 99, UserID: 1, Kind: "release"})
		}},
		{"空过期事件", "expiry_empty", func(r *points.AuditRows) {
			r.Transactions = append(r.Transactions, points.Transaction{ID: 99, UserID: 1, Kind: "expire"})
		}},
		{"空抵扣事件", "offset_empty", func(r *points.AuditRows) {
			r.Transactions = append(r.Transactions, points.Transaction{ID: 99, UserID: 1, Kind: "debt_offset"})
		}},
		{"分录事件不存在", "entry_txn", func(r *points.AuditRows) { r.Entries[0].TxnID = 99 }},
		{"重复分录", "entry_duplicate", func(r *points.AuditRows) { r.Entries = append(r.Entries, r.Entries[0]) }},
		{"多余分录", "entry_unexpected", func(r *points.AuditRows) {
			r.Entries = append(r.Entries, points.Entry{TxnID: 6, LotID: 2, UserID: 1, Bucket: points.Frozen, Delta: 1})
		}},
		{"缺少分录", "txn_entries", func(r *points.AuditRows) { r.Entries = r.Entries[:len(r.Entries)-1] }},
		{"欠款身份", "debt_snapshot", func(r *points.AuditRows) { r.Debts[0].UserID = 2 }},
		{"欠款和账户同减", "debt_allocations", func(r *points.AuditRows) { r.Debts[0].Outstanding = 1; r.Account.Debt = 1 }},
		{"欠款重复", "debt_duplicate", func(r *points.AuditRows) { r.Debts = append(r.Debts, r.Debts[0]) }},
		{"欠款事件重复", "debt_reversal_duplicate", func(r *points.AuditRows) { d := r.Debts[0]; d.ID = 2; r.Debts = append(r.Debts, d) }},
		{"欠款来源事件", "debt_origin", func(r *points.AuditRows) { r.Debts[0].ReversalTxnID = 5 }},
		{"欠款分摊归属", "debt_allocation_owner", func(r *points.AuditRows) { r.DebtAllocations[0].UserID = 2 }},
		{"欠款分摊原单", "debt_allocation_owner", func(r *points.AuditRows) { r.DebtAllocations[0].DebtID = 99 }},
		{"欠款分摊批次", "debt_allocation_owner", func(r *points.AuditRows) { r.DebtAllocations[0].LotID = 99 }},
		{"购买抵扣", "debt_allocation_owner", func(r *points.AuditRows) { r.Lots[1].Source = points.Purchase; r.Lots[1].CashBacked = true }},
		{"重复抵扣分摊", "debt_allocation_duplicate", func(r *points.AuditRows) { r.DebtAllocations = append(r.DebtAllocations, r.DebtAllocations[0]) }},
		{"抵扣事件", "offset_event", func(r *points.AuditRows) { r.DebtAllocations[0].TxnID = 5 }},
		{"抵扣业务", "offset_empty", func(r *points.AuditRows) { r.Transactions[5].BizType = "points_grant" }},
		{"抵扣批次事件", "offset_event", func(r *points.AuditRows) { r.Transactions[5].BizID = "grant:1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := pointsAuditDebtFixture()
			tc.change(&r)
			report := ReconcilePoints(r)
			for _, issue := range report.Issues {
				if issue.Code == tc.code {
					return
				}
			}
			t.Fatal("未检出指定损坏", tc.code, report)
		})
	}
}

func TestPointsReconcileExpiryAndReleaseEvents(t *testing.T) {
	date := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	base := func() points.AuditRows {
		return points.AuditRows{Account: points.Account{UserID: 1}, Transactions: []points.Transaction{{ID: 1, UserID: 1, Kind: "grant", BizType: "points_grant"}, {ID: 2, UserID: 1, Kind: "expire", BizType: "lot_expiry"}, {ID: 3, UserID: 1, Kind: "reversal", BizType: "points_refund", ReversesID: 1}}, Lots: []points.Lot{{ID: 1, UserID: 1, GrantTxnID: 1, Source: points.Gift, Total: 5, ExpiresAt: &date}}, Entries: []points.Entry{{TxnID: 1, LotID: 1, UserID: 1, Bucket: points.Available, Delta: 5}, {TxnID: 2, LotID: 1, UserID: 1, Bucket: points.Available, Delta: -5}}}
	}
	if report := ReconcilePoints(base()); !report.Healthy() {
		t.Fatal(report)
	}
	for _, change := range []func(*points.AuditRows){func(r *points.AuditRows) { r.Entries[1].Delta = 5 }, func(r *points.AuditRows) { r.Entries[1].Bucket = points.Frozen }, func(r *points.AuditRows) { r.Lots[0].ExpiresAt = nil }, func(r *points.AuditRows) { r.Transactions[1].BizType = "ai_task" }} {
		r := base()
		change(&r)
		if ReconcilePoints(r).Healthy() {
			t.Fatal("非法过期事件未检出")
		}
	}
	r := pointsAuditDebtFixture()
	r.Transactions = r.Transactions[:3]
	r.Transactions[2].Kind = "release"
	r.Lots = r.Lots[:1]
	r.Lots[0].Remain = 20
	r.Account.Avail = 20
	r.Account.Debt = 0
	r.Holds[0].State = points.Released
	r.Holds[0].Captured = 0
	r.Entries = r.Entries[:4]
	r.Entries = append(r.Entries, points.Entry{TxnID: 3, LotID: 1, UserID: 1, Bucket: points.Available, Delta: 7})
	r.Debts = nil
	r.DebtAllocations = nil
	if report := ReconcilePoints(r); !report.Healthy() {
		t.Fatal("完整释放事件正例失败", report)
	}
}
