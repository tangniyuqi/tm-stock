package service

import (
	"context"
	"math/big"
	"sort"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func (s *PointsService) Reconcile(ctx context.Context, uid int64) (points.AuditReport, error) {
	if s == nil || s.store == nil || uid <= 0 {
		return points.AuditReport{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var out points.AuditReport
	err := s.store.WithinPointsTx(ctx, func(tx points.Tx) error {
		rows, err := tx.ReadAudit(ctx, uid)
		if err != nil {
			return err
		}
		out = ReconcilePoints(rows)
		return nil
	})
	if err != nil {
		return points.AuditReport{}, err
	}
	return out, nil
}

// ReconcilePoints 使用任意精度累加，不能因溢出把人为破坏的数据对成零。
// 当前核对账本，任务锁价之和须在任务表落地后补齐，不能把此结果当作 AC-B4 全量通过。
func ReconcilePoints(rows points.AuditRows) points.AuditReport {
	out := points.AuditReport{UserID: rows.Account.UserID, Issues: make([]points.AuditIssue, 0)}
	issue := func(code string, id int64) {
		out.Issues = append(out.Issues, points.AuditIssue{Code: code, RecordID: id})
	}
	if rows.Account.UserID <= 0 || rows.Account.Avail < 0 || rows.Account.Frozen < 0 || rows.Account.Debt < 0 {
		issue("account_snapshot", rows.Account.UserID)
	}
	type buckets struct{ avail, frozen big.Int }
	entries := make(map[int64]*buckets)
	lots := make(map[int64]points.Lot)
	holds := make(map[int64]points.Hold)
	allocated := make(map[int64]*big.Int)
	activeLots := make(map[int64]*big.Int)
	add := func(target map[int64]*big.Int, id, value int64) {
		if target[id] == nil {
			target[id] = new(big.Int)
		}
		target[id].Add(target[id], big.NewInt(value))
	}
	for _, lot := range rows.Lots {
		lots[lot.ID] = lot
	}
	for _, hold := range rows.Holds {
		holds[hold.ID] = hold
	}
	for _, e := range rows.Entries {
		if _, ok := lots[e.LotID]; !ok || e.UserID != rows.Account.UserID || e.Delta == 0 {
			issue("entry_owner", e.LotID)
		}
		if entries[e.LotID] == nil {
			entries[e.LotID] = new(buckets)
		}
		switch e.Bucket {
		case points.Available:
			entries[e.LotID].avail.Add(&entries[e.LotID].avail, big.NewInt(e.Delta))
		case points.Frozen:
			entries[e.LotID].frozen.Add(&entries[e.LotID].frozen, big.NewInt(e.Delta))
		default:
			issue("entry_bucket", e.LotID)
		}
	}
	for _, a := range rows.Allocations {
		h, exists := holds[a.HoldID]
		_, lotExists := lots[a.LotID]
		if !exists || !lotExists || a.UserID != rows.Account.UserID || a.Amount <= 0 {
			issue("allocation_owner", a.HoldID)
		}
		add(allocated, a.HoldID, a.Amount)
		if h.State == points.Holding {
			add(activeLots, a.LotID, a.Amount)
		}
	}
	var avail, frozen, active, debt big.Int
	for _, lot := range rows.Lots {
		if lot.UserID != rows.Account.UserID || lot.Total <= 0 || lot.Remain < 0 || lot.Frozen < 0 || lot.Frozen > lot.Remain || lot.Remain > lot.Total {
			issue("lot_snapshot", lot.ID)
		}
		b := entries[lot.ID]
		if b == nil {
			b = new(buckets)
		}
		var remain big.Int
		remain.Add(&b.avail, &b.frozen)
		if remain.Cmp(big.NewInt(lot.Remain)) != 0 || b.frozen.Cmp(big.NewInt(lot.Frozen)) != 0 {
			issue("lot_entries", lot.ID)
		}
		avail.Add(&avail, new(big.Int).Sub(big.NewInt(lot.Remain), big.NewInt(lot.Frozen)))
		frozen.Add(&frozen, big.NewInt(lot.Frozen))
		allocation := activeLots[lot.ID]
		if allocation == nil {
			allocation = new(big.Int)
		}
		if allocation.Cmp(big.NewInt(lot.Frozen)) != 0 {
			issue("lot_holds", lot.ID)
		}
	}
	for _, h := range rows.Holds {
		allocatedAmount := allocated[h.ID]
		if allocatedAmount == nil {
			allocatedAmount = new(big.Int)
		}
		if h.UserID != rows.Account.UserID || h.Amount <= 0 || allocatedAmount.Cmp(big.NewInt(h.Amount)) != 0 {
			issue("hold_allocation", h.ID)
		}
		switch h.State {
		case points.Holding:
			active.Add(&active, big.NewInt(h.Amount))
			if h.Captured != 0 || h.TerminalTxnID != 0 {
				issue("hold_state", h.ID)
			}
		case points.Settled:
			if h.Captured != h.Amount || h.TerminalTxnID <= 0 {
				issue("hold_state", h.ID)
			}
		case points.Released, points.Expired:
			if h.Captured != 0 || h.TerminalTxnID <= 0 {
				issue("hold_state", h.ID)
			}
		default:
			issue("hold_state", h.ID)
		}
	}
	debts := make(map[int64]points.Debt)
	debtPaid := make(map[int64]*big.Int)
	for _, value := range rows.Debts {
		if value.ID <= 0 || value.UserID != rows.Account.UserID || value.Amount <= 0 || value.Outstanding < 0 || value.Outstanding > value.Amount {
			issue("debt_snapshot", value.ID)
		}
		if _, duplicate := debts[value.ID]; duplicate {
			issue("debt_duplicate", value.ID)
		}
		debts[value.ID] = value
		debt.Add(&debt, big.NewInt(value.Outstanding))
	}
	for _, a := range rows.DebtAllocations {
		d, exists := debts[a.DebtID]
		lot, lotExists := lots[a.LotID]
		if !exists || !lotExists || a.UserID != rows.Account.UserID || d.UserID != a.UserID || a.Amount <= 0 || lot.Source == points.Purchase {
			issue("debt_allocation_owner", a.DebtID)
		}
		add(debtPaid, a.DebtID, a.Amount)
	}
	for _, d := range rows.Debts {
		paid := debtPaid[d.ID]
		if paid == nil {
			paid = new(big.Int)
		}
		if new(big.Int).Add(paid, big.NewInt(d.Outstanding)).Cmp(big.NewInt(d.Amount)) != 0 {
			issue("debt_allocations", d.ID)
		}
	}
	if avail.Cmp(big.NewInt(rows.Account.Avail)) != 0 {
		issue("account_available", rows.Account.UserID)
	}
	if frozen.Cmp(big.NewInt(rows.Account.Frozen)) != 0 || active.Cmp(&frozen) != 0 {
		issue("account_frozen", rows.Account.UserID)
	}
	if debt.Cmp(big.NewInt(rows.Account.Debt)) != 0 {
		issue("account_debt", rows.Account.UserID)
	}
	reconcilePointsEvents(rows, issue)
	sort.Slice(out.Issues, func(i, j int) bool {
		if out.Issues[i].Code != out.Issues[j].Code {
			return out.Issues[i].Code < out.Issues[j].Code
		}
		return out.Issues[i].RecordID < out.Issues[j].RecordID
	})
	return out
}
