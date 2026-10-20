package service

import (
	"fmt"
	"math/big"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 不仅核对汇总，还从不可变事件重建应有分录，防止「错误入账但所有快照一起对平」。
func reconcilePointsEvents(rows points.AuditRows, issue func(string, int64)) {
	type entryKey struct {
		txn, lot int64
		bucket   int
	}
	actual, expected := make(map[entryKey]*big.Int), make(map[entryKey]*big.Int)
	txns := make(map[int64]points.Transaction)
	lots := make(map[int64]points.Lot)
	origins := make(map[int64]points.Lot)
	freeze, terminal := make(map[int64]points.Hold), make(map[int64]points.Hold)
	holds := make(map[int64]points.Hold)
	debts := make(map[int64]points.Debt)
	expiryEntries := make(map[int64][]points.Entry)
	reversalDebt := make(map[int64]points.Debt)
	expired := make(map[int64]*big.Int)
	seenReversals := make(map[int64]bool)
	add := func(dst map[entryKey]*big.Int, key entryKey, value *big.Int) {
		if dst[key] == nil {
			dst[key] = new(big.Int)
		}
		dst[key].Add(dst[key], value)
	}
	expect := func(txn, lot int64, bucket int, value int64, negative bool) {
		n := big.NewInt(value)
		if negative {
			n.Neg(n)
		}
		add(expected, entryKey{txn, lot, bucket}, n)
	}
	for _, x := range rows.Transactions {
		if x.ID <= 0 || x.UserID != rows.Account.UserID {
			issue("txn_owner", x.ID)
		}
		if _, exists := txns[x.ID]; exists {
			issue("txn_duplicate", x.ID)
		}
		txns[x.ID] = x
	}
	for _, lot := range rows.Lots {
		if _, exists := origins[lot.GrantTxnID]; exists {
			issue("grant_duplicate", lot.ID)
		}
		if lot.Source < points.Purchase || lot.Source > points.Signin || lot.CashBacked != (lot.Source == points.Purchase) {
			issue("lot_source", lot.ID)
		}
		lots[lot.ID], origins[lot.GrantTxnID] = lot, lot
		if x := txns[lot.GrantTxnID]; x.Kind != "grant" {
			issue("lot_origin", lot.ID)
		}
	}
	for _, h := range rows.Holds {
		holds[h.ID] = h
		if _, exists := freeze[h.FreezeTxnID]; exists {
			issue("hold_freeze_duplicate", h.ID)
		}
		freeze[h.FreezeTxnID] = h
		if h.TerminalTxnID != 0 {
			if _, exists := terminal[h.TerminalTxnID]; exists {
				issue("hold_terminal_duplicate", h.ID)
			}
			terminal[h.TerminalTxnID] = h
		}
		if x := txns[h.FreezeTxnID]; x.Kind != "freeze" || x.BizID != h.BizID || x.BizType != h.BizType {
			issue("hold_freeze_event", h.ID)
		}
		if h.TerminalTxnID != 0 {
			x := txns[h.TerminalTxnID]
			want := "release"
			if h.State == points.Settled {
				want = "settle"
			}
			if x.Kind != want || x.BizID != h.BizID || x.BizType != h.BizType {
				issue("hold_terminal_event", h.ID)
			}
		}
	}
	for _, d := range rows.Debts {
		debts[d.ID] = d
		if _, exists := reversalDebt[d.ReversalTxnID]; exists {
			issue("debt_reversal_duplicate", d.ID)
		}
		reversalDebt[d.ReversalTxnID] = d
		if x := txns[d.ReversalTxnID]; x.Kind != "reversal" {
			issue("debt_origin", d.ID)
		}
	}
	for _, e := range rows.Entries {
		key := entryKey{e.TxnID, e.LotID, e.Bucket}
		if actual[key] != nil {
			issue("entry_duplicate", e.TxnID)
		}
		add(actual, key, big.NewInt(e.Delta))
		if x, exists := txns[e.TxnID]; !exists {
			issue("entry_txn", e.TxnID)
		} else if x.Kind == "expire" {
			expiryEntries[e.TxnID] = append(expiryEntries[e.TxnID], e)
			if expired[e.LotID] == nil {
				expired[e.LotID] = new(big.Int)
			}
			expired[e.LotID].Sub(expired[e.LotID], big.NewInt(e.Delta))
		}
	}
	for _, a := range rows.Allocations {
		hold := holds[a.HoldID]
		expect(hold.FreezeTxnID, a.LotID, points.Available, a.Amount, true)
		expect(hold.FreezeTxnID, a.LotID, points.Frozen, a.Amount, false)
		if hold.TerminalTxnID != 0 {
			expect(hold.TerminalTxnID, a.LotID, points.Frozen, a.Amount, true)
			if hold.State != points.Settled {
				expect(hold.TerminalTxnID, a.LotID, points.Available, a.Amount, false)
			}
		}
	}
	type debtKey struct{ txn, debt, lot int64 }
	seenDebt := make(map[debtKey]bool)
	offsetCount := make(map[int64]int)
	for _, a := range rows.DebtAllocations {
		key := debtKey{a.TxnID, a.DebtID, a.LotID}
		if seenDebt[key] {
			issue("debt_allocation_duplicate", a.TxnID)
		}
		seenDebt[key] = true
		x, lot := txns[a.TxnID], lots[a.LotID]
		if x.Kind != "debt_offset" || x.BizID != fmt.Sprintf("grant:%d", lot.GrantTxnID) || x.ID <= lot.GrantTxnID || x.ID <= debts[a.DebtID].ReversalTxnID {
			issue("offset_event", a.TxnID)
		}
		offsetCount[a.TxnID]++
		expect(a.TxnID, a.LotID, points.Available, a.Amount, true)
	}
	for _, x := range rows.Transactions {
		if (x.Kind == "reversal") != (x.ReversesID > 0) {
			issue("txn_reverses", x.ID)
		}
		switch x.Kind {
		case "grant":
			lot, exists := origins[x.ID]
			if !exists || x.BizType != "points_grant" {
				issue("grant_event", x.ID)
			}
			expect(x.ID, lot.ID, points.Available, lot.Total, false)
		case "freeze":
			if _, exists := freeze[x.ID]; !exists {
				issue("freeze_event", x.ID)
			}
		case "settle", "release":
			if _, exists := terminal[x.ID]; !exists {
				issue("terminal_event", x.ID)
			}
		case "reversal":
			lot, exists := origins[x.ReversesID]
			if !exists || txns[x.ReversesID].Kind != "grant" || x.BizType != "points_refund" || seenReversals[x.ReversesID] {
				issue("reversal_event", x.ID)
			}
			seenReversals[x.ReversesID] = true
			recovered := big.NewInt(lot.Total)
			recovered.Sub(recovered, big.NewInt(reversalDebt[x.ID].Amount))
			if expired[lot.ID] != nil {
				recovered.Sub(recovered, expired[lot.ID])
			}
			if recovered.Sign() < 0 || recovered.Cmp(big.NewInt(lot.Total)) > 0 {
				issue("reversal_amount", x.ID)
			}
			if recovered.Sign() > 0 {
				add(expected, entryKey{x.ID, lot.ID, points.Available}, new(big.Int).Neg(recovered))
			}
		case "debt_offset":
			if x.BizType != "debt_offset" || offsetCount[x.ID] == 0 {
				issue("offset_empty", x.ID)
			}
		case "expire":
			found := false
			for _, e := range expiryEntries[x.ID] {
				found = true
				lot := lots[e.LotID]
				if e.Bucket != points.Available || e.Delta >= 0 || lot.ExpiresAt == nil || lot.Source == points.Purchase || x.BizType != "lot_expiry" {
					issue("expiry_event", x.ID)
				}
				add(expected, entryKey{e.TxnID, e.LotID, e.Bucket}, big.NewInt(e.Delta))
			}
			if !found {
				issue("expiry_empty", x.ID)
			}
		default:
			issue("txn_kind", x.ID)
		}
	}
	for key, value := range expected {
		if actual[key] == nil || actual[key].Cmp(value) != 0 {
			issue("txn_entries", key.txn)
		}
	}
	for key := range actual {
		if expected[key] == nil {
			issue("entry_unexpected", key.txn)
		}
	}
}
