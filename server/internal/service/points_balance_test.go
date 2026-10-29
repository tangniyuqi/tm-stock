package service

import (
	"context"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type balanceReadStore struct {
	tx          points.Tx
	commitError error
}

func (s balanceReadStore) WithinPointsTx(ctx context.Context, fn func(points.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fn(s.tx); err != nil {
		return err
	}
	return s.commitError
}

func TestPointsBalanceReadOnlyAndFailure(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "invalid", "cancelled", "account_error", "clock_error", "lots_error", "expired", "commit"} {
		t.Run(mode, func(t *testing.T) {
			_, fixture, _, _, _ := admissionFixture(t)
			p := fixture.points
			p.account.Frozen = 3
			p.lots[0].Total, p.lots[0].Remain, p.lots[0].Frozen = 15, 15, 3
			store := balanceReadStore{tx: p}
			uid := int64(1)
			ctx := context.Background()
			switch mode {
			case "missing":
				uid = 2
			case "invalid":
				uid = 0
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "account_error":
				p.failAt = 1
			case "clock_error":
				p.failAt = 2
			case "lots_error":
				p.failAt = 3
			case "expired":
				at := p.now
				p.lots[0].ExpiresAt = &at
			case "commit":
				store.commitError = errPointsInjected
			}
			svc, err := NewPointsService(store)
			if err != nil {
				t.Fatal(err)
			}
			out, err := svc.Balance(ctx, uid)
			if mode == "valid" {
				if err != nil || out != (PointsBalance{Available: 12, Frozen: 3, CheckedAt: p.now.UnixMilli()}) {
					t.Fatal(out, err)
				}
				if p.calls != 3 {
					t.Fatal("余额应只读一次账户、时钟和批次", p.calls)
				}
			} else if mode == "missing" {
				if err != nil || out != (PointsBalance{CheckedAt: p.now.UnixMilli()}) {
					t.Fatal(out, err)
				}
			} else if err == nil || out != (PointsBalance{}) {
				t.Fatal("失败不得返回可展示的余额", out, err)
			}
			if len(p.entries) != 0 || p.original.ID != 0 || p.account.Avail != 20 || p.account.Frozen != 3 {
				t.Fatal("余额读取改变账本")
			}
		})
	}
}
