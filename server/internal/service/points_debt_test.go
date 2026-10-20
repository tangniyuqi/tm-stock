package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func scriptedPointsReverse(t *testing.T) (*PointsService, *scriptedPointsStore, points.ReverseRequest) {
	t.Helper()
	s, store, _, _, _ := scriptedPoints(t)
	store.tx.original = points.Transaction{ID: 8, UserID: 1, Kind: "grant", BizType: "points_grant"}
	store.tx.lot.GrantTxnID = 8
	store.tx.lot.Source = points.Gift
	store.tx.lot.Remain = 13
	store.tx.account.Avail = 13
	return s, store, points.ReverseRequest{UserID: 1, GrantTxnID: 8, Key: "test_reverse_0001", BizID: "refund:8"}
}

func TestPointsDebtEveryStorageBoundaryAndCommit(t *testing.T) {
	for _, kind := range []string{"reverse", "reverse_consumed", "reward_offset"} {
		prepare := func() (*scriptedPointsStore, func() (points.Receipt, error)) {
			if kind == "reward_offset" {
				s, store, g, _, _ := scriptedPoints(t)
				store.tx.account.Debt = 12
				store.tx.debts = []points.Debt{{ID: 1, UserID: 1, Amount: 5, Outstanding: 5}, {ID: 2, UserID: 1, Amount: 7, Outstanding: 7}}
				return store, func() (points.Receipt, error) { return s.Grant(context.Background(), g) }
			}
			s, store, r := scriptedPointsReverse(t)
			if kind == "reverse_consumed" {
				store.tx.lot.Remain, store.tx.account.Avail = 0, 0
			}
			return store, func() (points.Receipt, error) { return s.Reverse(context.Background(), r) }
		}
		baseline, run := prepare()
		if _, err := run(); err != nil {
			t.Fatal(err)
		}
		for n := 1; n <= baseline.tx.calls; n++ {
			t.Run(fmt.Sprintf("%s_%d", kind, n), func(t *testing.T) {
				store, run := prepare()
				store.tx.failAt = n
				if out, err := run(); !errors.Is(err, errPointsInjected) || out != (points.Receipt{}) {
					t.Fatal("冲正/抵扣故障被吞或返回成功收据", out, err)
				}
			})
		}
		store, run := prepare()
		store.commitError = errPointsInjected
		if out, err := run(); !errors.Is(err, errPointsInjected) || out != (points.Receipt{}) {
			t.Fatal("提交失败返回了成功收据", out, err)
		}
	}
}

func TestPointsReverseAmountsReplayAndGuards(t *testing.T) {
	for _, tc := range []struct{ remain, expired, debt int64 }{{20, 0, 0}, {13, 0, 7}, {0, 0, 20}, {5, 15, 0}, {0, 20, 0}} {
		s, store, r := scriptedPointsReverse(t)
		store.tx.lot.Remain, store.tx.account.Avail, store.tx.expired = tc.remain, tc.remain, tc.expired
		out, err := s.Reverse(context.Background(), r)
		if err != nil || out.Amount != 20 || out.Kind != "reversal" || out.LotID != 20 || store.tx.newDebt.Amount != tc.debt {
			t.Fatal(out, err, store.tx.newDebt)
		}
		if tc.remain+tc.debt > 0 && (store.tx.lastAccount.Avail != 0 || store.tx.lastAccount.Debt != tc.debt) {
			t.Fatal("冲正快照错误", store.tx.lastAccount)
		}
		if tc.remain > 0 && (len(store.tx.entries) != 1 || store.tx.entries[0].Delta != -tc.remain) {
			t.Fatal("冲正分录错误")
		}
		hash, _ := points.RequestHash("reversal", r)
		store.tx.existing = points.Transaction{ID: out.TxnID, UserID: 1, Kind: "reversal", ReversesID: 8, Hash: hash}
		if replay, err := s.Reverse(context.Background(), r); err != nil || replay != out {
			t.Fatal("冲正重放不稳定", replay, err)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*scriptedPointsTx)
		want   error
	}{
		{"非入账流水", func(tx *scriptedPointsTx) { tx.original.Kind = "settle" }, points.ErrInvalid},
		{"原流水非本人", func(tx *scriptedPointsTx) { tx.original.UserID = 2 }, points.ErrInvalid},
		{"冻结批次", func(tx *scriptedPointsTx) { tx.lot.Frozen = 7; tx.account.Avail = 6 }, points.ErrConflict},
		{"越界批次", func(tx *scriptedPointsTx) { tx.lot.Remain = 21 }, points.ErrInvariant},
		{"原额为零", func(tx *scriptedPointsTx) { tx.lot.Total = 0 }, points.ErrInvariant},
		{"跨用户批次", func(tx *scriptedPointsTx) { tx.lot.UserID = 2 }, points.ErrInvariant},
		{"批次来源错误", func(tx *scriptedPointsTx) { tx.lot.GrantTxnID = 9 }, points.ErrInvariant},
		{"快照不足", func(tx *scriptedPointsTx) { tx.account.Avail = 0 }, points.ErrInvariant},
		{"过期负数", func(tx *scriptedPointsTx) { tx.expired = -1 }, points.ErrInvariant},
		{"过期超额", func(tx *scriptedPointsTx) { tx.expired = 8 }, points.ErrInvariant},
		{"欠款溢出", func(tx *scriptedPointsTx) { tx.account.Debt = math.MaxInt64 }, points.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, store, r := scriptedPointsReverse(t)
			tc.change(store.tx)
			if _, err := s.Reverse(context.Background(), r); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	s, _, r := scriptedPointsReverse(t)
	if _, err := s.ReverseInTx(context.Background(), nil, r); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Reverse(context.Background(), points.ReverseRequest{}); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestPointsRewardOffsetPurchaseExclusionAndBounds(t *testing.T) {
	for _, source := range []int{points.Purchase, points.Gift, points.Contribution, points.Signin} {
		for _, amount := range []int64{5, 10, 15} {
			s, store, g, _, _ := scriptedPoints(t)
			store.tx.account.Debt = 10
			store.tx.debts = []points.Debt{{ID: 1, UserID: 1, Amount: 4, Outstanding: 4}, {ID: 2, UserID: 1, Amount: 6, Outstanding: 6}}
			g.Amount, g.Source = amount, source
			out, err := s.Grant(context.Background(), g)
			offset := min(amount, int64(10))
			if source == points.Purchase {
				offset = 0
			}
			if err != nil || out.Amount != amount || store.tx.lastAccount.Avail != 20+amount-offset || store.tx.lastAccount.Debt != 10-offset || store.tx.newLot.Remain != amount-offset {
				t.Fatal(out, err, store.tx.lastAccount, store.tx.newLot)
			}
			var allocated int64
			for _, a := range store.tx.debtAllocations {
				allocated += a.Amount
			}
			if allocated != offset {
				t.Fatal("抵扣分摊额错误")
			}
		}
	}
	for _, change := range []func(*scriptedPointsTx){
		func(tx *scriptedPointsTx) { tx.debts = nil },
		func(tx *scriptedPointsTx) { tx.debts[0].UserID = 2 },
		func(tx *scriptedPointsTx) { tx.debts[0].Outstanding = 0 },
		func(tx *scriptedPointsTx) { tx.debts[0].Amount = 4 },
		func(tx *scriptedPointsTx) { tx.debts[0].ID = 0 },
		func(tx *scriptedPointsTx) { tx.debts = append(tx.debts, tx.debts[0]) },
	} {
		s, store, g, _, _ := scriptedPoints(t)
		store.tx.account.Debt = 5
		store.tx.debts = []points.Debt{{ID: 1, UserID: 1, Amount: 5, Outstanding: 5}}
		change(store.tx)
		if _, err := s.Grant(context.Background(), g); !errors.Is(err, points.ErrInvariant) {
			t.Fatal("损坏欠款未拒绝", err)
		}
	}
	// 全额抵扣时不会暂时把账户加到 int64 上限以外。
	s, store, g, _, _ := scriptedPoints(t)
	store.tx.account.Avail, store.tx.account.Debt = math.MaxInt64, 20
	store.tx.debts = []points.Debt{{ID: 1, UserID: 1, Amount: 20, Outstanding: 20}}
	if _, err := s.Grant(context.Background(), g); err != nil || store.tx.lastAccount.Avail != math.MaxInt64 {
		t.Fatal(err)
	}
	store.tx.debts = []points.Debt{{ID: 1, UserID: 1, Amount: math.MaxInt64, Outstanding: math.MaxInt64}, {ID: 2, UserID: 1, Amount: 1, Outstanding: 1}}
	if _, err := s.Grant(context.Background(), g); !errors.Is(err, points.ErrOverflow) {
		t.Fatal("欠款求和溢出未拒绝", err)
	}
}
