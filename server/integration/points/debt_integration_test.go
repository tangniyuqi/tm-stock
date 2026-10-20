//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func TestIntegrationPointsRefundRewardRoundTrip(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, s, now := pointsFixture(t, level)
			g := pointGrant(now, 20)
			g.Source = points.Purchase
			original, err := s.Grant(e.ctx, g)
			if err != nil {
				t.Fatal(err)
			}
			g.Key, g.BizID, g.Amount = "test_purchase_002", "purchase:2", 30
			other, err := s.Grant(e.ctx, g)
			if err != nil {
				t.Fatal(err)
			}
			frozen, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Finalize(e.ctx, points.FinalizeRequest{UserID: 1, Key: "test_settle_00001", HoldID: frozen.HoldID, State: points.Settled}); err != nil {
				t.Fatal(err)
			}
			r := points.ReverseRequest{UserID: 1, GrantTxnID: original.TxnID, Key: "test_reverse_0001", BizID: "refund:1"}
			reversal, err := s.Reverse(e.ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 30 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT remain FROM tm_points_lots WHERE id=?", other.LotID) != 30 {
				t.Fatal("冲正误扣其他购买批次或未记消费欠款")
			}
			requirePointAudit(t, e, s)
			if again, err := s.Reverse(e.ctx, r); err != nil || again != reversal {
				t.Fatal("冲正重放改变收据", err)
			}
			r.Key = "test_reverse_newkey"
			if _, err := s.Reverse(e.ctx, r); !errors.Is(err, points.ErrConflict) {
				t.Fatal("换键重复冲正未拒绝", err)
			}
			r.Key, r.GrantTxnID = "test_reverse_0001", other.TxnID
			if _, err := s.Reverse(e.ctx, r); !errors.Is(err, points.ErrIdempotency) {
				t.Fatal("同键不同原单未拒绝", err)
			}
			r.Key, r.GrantTxnID, r.BizID = "test_reverse_badkind", frozen.TxnID, "refund:badkind"
			if _, err := s.Reverse(e.ctx, r); !errors.Is(err, points.ErrInvalid) {
				t.Fatal("非入账流水被冲正", err)
			}
			g.Source, g.Amount, g.Key, g.BizID = points.Signin, 5, "test_reward_00001", "signin:1:1"
			reward, err := s.Grant(e.ctx, g)
			if err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 2 || e.queryInt("SELECT remain FROM tm_points_lots WHERE id=?", reward.LotID) != 0 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 30 {
				t.Fatal("奖励未优先抵扣")
			}
			requirePointAudit(t, e, s)
			g.Source, g.Amount, g.Key, g.BizID = points.Purchase, 4, "test_purchase_003", "purchase:3"
			if _, err := s.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 2 {
				t.Fatal("购买本金参与抵扣")
			}
			g.Source, g.Amount, g.Key, g.BizID = points.Contribution, 10, "test_reward_00002", "contribution:1"
			if _, err := s.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 42 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_debt_allocs") != 2 {
				t.Fatal("部分抵扣剩余奖励或分摊错误")
			}
			g.Source, g.Amount, g.Key, g.BizID = points.Signin, 5, "test_reward_00001", "signin:1:1"
			if again, err := s.Grant(e.ctx, g); err != nil || again != reward {
				t.Fatal("欠款变化后奖励重放变化", err)
			}
			// 被抵扣用掉的奖励撤销后，恢复等额欠款，不改历史抵扣与旧欠款原额。
			reversedReward, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 1, GrantTxnID: reward.TxnID, Key: "test_reverse_reward", BizID: "refund:reward"})
			if err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 42 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 5 {
				t.Fatal("已抵扣奖励冲正错误")
			}
			requirePointAudit(t, e, s)
			// 冲正另一笔已部分抵扣的贡献奖励，产生第二笔欠款，后续奖励按欠款 id 顺序跨单抵扣。
			var contributionID int64
			if err := e.conn.QueryRowContext(e.ctx, "SELECT id FROM tm_points_txns WHERE idem_key='test_reward_00002'").Scan(&contributionID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 1, GrantTxnID: contributionID, Key: "test_reverse_contribution", BizID: "refund:contribution"}); err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 7 {
				t.Fatal("跨单欠款原额错误")
			}
			g.Source, g.Amount, g.Key, g.BizID = points.Gift, 6, "test_reward_multidebt", "reward:multi"
			if _, err := s.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT outstanding FROM tm_points_debts WHERE reversal_txn_id=?", reversedReward.TxnID) != 0 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 1 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 34 {
				t.Fatal("多欠款分摊顺序或余额错误")
			}
			g.Amount, g.Key, g.BizID = 3, "test_reward_finishdebt", "reward:finish"
			if _, err := s.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 36 || e.queryInt("SELECT COUNT(*) FROM tm_points_debt_allocs") != 5 {
				t.Fatal("多欠款最终抵扣错误")
			}
			requirePointAudit(t, e, s)
			g.UserID, g.Key, g.BizID = 2, "test_otheruser_grant", "grant:user2"
			if _, err := s.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 2, GrantTxnID: original.TxnID, Key: "test_cross_reverse", BizID: "refund:user2"}); !errors.Is(err, points.ErrNotFound) {
				t.Fatal("跨用户冲正未拒绝", err)
			}
		})
	}
}

func TestIntegrationPointsDebtIntegerLimit(t *testing.T) {
	e, _, _, s, now := pointsFixture(t, sql.LevelReadCommitted)
	g := pointGrant(now, math.MaxInt64)
	grant, err := s.Grant(e.ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: math.MaxInt64})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Finalize(e.ctx, points.FinalizeRequest{UserID: 1, Key: "test_settle_maximum", HoldID: f.HoldID, State: points.Settled}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 1, GrantTxnID: grant.TxnID, Key: "test_reverse_maximum", BizID: "refund:maximum"}); err != nil {
		t.Fatal(err)
	}
	if int64(e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1")) != math.MaxInt64 {
		t.Fatal("最大整数欠款错误")
	}
	requirePointAudit(t, e, s)
	g.Key, g.BizID = "test_reward_maximum", "reward:maximum"
	if _, err := s.Grant(e.ctx, g); err != nil {
		t.Fatal(err)
	}
	if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 0 || int64(e.queryInt("SELECT SUM(amount) FROM tm_points_debt_allocs")) != math.MaxInt64 {
		t.Fatal("最大整数抵扣错误")
	}
	requirePointAudit(t, e, s)
}

func TestIntegrationPointsParentRepeatableReadSnapshot(t *testing.T) {
	for _, mode := range []string{"grant_after_snapshot", "expiry_after_snapshot", "allocation_after_snapshot", "audit_after_snapshot"} {
		t.Run(mode, func(t *testing.T) {
			e, db, _, s, now := pointsFixture(t, sql.LevelRepeatableRead)
			g := pointGrant(now, 20)
			if mode == "expiry_after_snapshot" || mode == "audit_after_snapshot" {
				expiry := now.Add(-time.Hour)
				g.EffectiveAt, g.ExpiresAt = now.Add(-2*time.Hour), &expiry
			}
			original, err := s.Grant(e.ctx, g)
			if err != nil {
				t.Fatal(err)
			}
			parent, err := db.BeginTx(e.ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
			if err != nil {
				t.Fatal(err)
			}
			// 失败时回滚；成功提交后的 ErrTxDone 不改变提交事实。
			defer func() { _ = parent.Rollback() }()
			var count int
			if err := parent.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM tm_points_txns").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if mode == "allocation_after_snapshot" {
				f, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.FinalizeInTx(e.ctx, repository.PointsTx(parent), points.FinalizeRequest{UserID: 1, Key: "test_parent_settle_1", HoldID: f.HoldID, State: points.Settled}); err != nil {
					t.Fatal("父事务旧快照不能漏掉新冻结分摊", err)
				}
			} else {
				if mode == "grant_after_snapshot" {
					g.Amount, g.Key, g.BizID = 5, "test_parent_grant_2", "grant:parent:2"
					original, err = s.Grant(e.ctx, g)
					if err != nil {
						t.Fatal(err)
					}
					if replay, err := s.GrantInTx(e.ctx, repository.PointsTx(parent), g); err != nil || replay != original {
						t.Fatal("父事务旧快照漏掉已提交入账的重放批次", err)
					}
				} else {
					e.mustExec(`INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,created_at,updated_at) VALUES(1,'test_parent_expiry001',?,'expire','lot_expiry','expiry:parent',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, strings.Repeat("b", 64))
					e.mustExec(`INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta,created_at,updated_at) VALUES(1,(SELECT id FROM tm_points_txns WHERE idem_key='test_parent_expiry001'),?,1,-20,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, original.LotID)
					e.mustExec("UPDATE tm_points_lots SET remain=0 WHERE id=?", original.LotID)
					e.mustExec("UPDATE tm_points_accounts SET avail=0 WHERE user_id=1")
				}
				if mode == "audit_after_snapshot" {
					rows, err := repository.PointsTx(parent).ReadAudit(e.ctx, 1)
					if err != nil {
						t.Fatal(err)
					}
					if audit := service.ReconcilePoints(rows); !audit.Healthy() {
						t.Fatal("父事务旧快照混入对账", audit)
					}
				}
				if _, err := s.ReverseInTx(e.ctx, repository.PointsTx(parent), points.ReverseRequest{UserID: 1, GrantTxnID: original.TxnID, Key: "test_parent_reverse_1", BizID: "refund:parent"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := parent.Commit(); err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("父事务旧快照把已记账过期额再次算成欠款")
			}
			requirePointAudit(t, e, s)
		})
	}
}

func TestIntegrationPointsAuditUserIndexScope(t *testing.T) {
	e, db, _, s, now := pointsFixture(t, sql.LevelRepeatableRead)
	grant, err := s.Grant(e.ctx, pointGrant(now, 20))
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Finalize(e.ctx, points.FinalizeRequest{UserID: 1, Key: "test_index_settle_1", HoldID: f.HoldID, State: points.Settled}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 1, Key: "test_index_reverse1", BizID: "refund:index", GrantTxnID: grant.TxnID}); err != nil {
		t.Fatal(err)
	}
	g := pointGrant(now, 5)
	g.Key, g.BizID = "test_index_reward1", "reward:index"
	if _, err := s.Grant(e.ctx, g); err != nil {
		t.Fatal(err)
	}
	g.UserID, g.Amount, g.Key, g.BizID = 2, 10, "test_index_user2_grant", "grant:index:user2"
	if _, err := s.Grant(e.ctx, g); err != nil {
		t.Fatal(err)
	}
	freeze := pointFreeze(now, 2)
	freeze.UserID, freeze.Key, freeze.BizID = 2, "test_index_user2_freeze", "task:index:user2"
	f, err = s.Freeze(e.ctx, freeze, points.Price{Version: "v1", Points: 1})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := db.BeginTx(e.ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	// 保持用户 1 的对账事务不提交；结束时只回滚这个测试事务。
	defer func() { _ = parent.Rollback() }()
	rows, err := repository.PointsTx(parent).ReadAudit(e.ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if audit := service.ReconcilePoints(rows); !audit.Healthy() {
		t.Fatal(audit)
	}
	ctx, cancel := context.WithTimeout(e.ctx, 3*time.Second)
	defer cancel()
	if _, err := s.Finalize(ctx, points.FinalizeRequest{UserID: 2, Key: "test_index_user2_final", HoldID: f.HoldID, State: points.Settled}); err != nil {
		t.Fatal("用户 1 对账不应通过分摊表全表扫描锁住用户 2 已有冻结", err)
	}
}

func TestIntegrationPointsDebtRollbackAndExpiryBasis(t *testing.T) {
	e, _, repo, s, now := pointsFixture(t, sql.LevelRepeatableRead)
	original, err := s.Grant(e.ctx, pointGrant(now, 20))
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
	if err != nil {
		t.Fatal(err)
	}
	r := points.ReverseRequest{UserID: 1, GrantTxnID: original.TxnID, Key: "test_reverse_0001", BizID: "refund:1"}
	if _, err := s.Reverse(e.ctx, r); !errors.Is(err, points.ErrConflict) {
		t.Fatal("冻结中的原批次被冲正", err)
	}
	final := points.FinalizeRequest{UserID: 1, Key: "test_settle_00001", HoldID: f.HoldID, State: points.Settled}
	if _, err := s.Finalize(e.ctx, final); err != nil {
		t.Fatal(err)
	}
	snapshot := func() []int {
		return []int{e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1"), e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1"), e.queryInt("SELECT COUNT(*) FROM tm_points_txns"), e.queryInt("SELECT COUNT(*) FROM tm_points_lots"), e.queryInt("SELECT COUNT(*) FROM tm_points_entries"), e.queryInt("SELECT COUNT(*) FROM tm_points_debt_allocs")}
	}
	before := snapshot()
	err = repo.WithinPointsTx(e.ctx, func(tx points.Tx) error {
		if _, err := s.ReverseInTx(e.ctx, tx, r); err != nil {
			return err
		}
		return context.Canceled
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("父事务失败没有回滚冲正", err)
	}
	if _, err := s.Reverse(e.ctx, r); err != nil {
		t.Fatal(err)
	}
	before = snapshot()
	g := pointGrant(now, 10)
	g.Key, g.BizID = "test_reward_offset", "reward:offset"
	err = repo.WithinPointsTx(e.ctx, func(tx points.Tx) error {
		if _, err := s.GrantInTx(e.ctx, tx, g); err != nil {
			return err
		}
		return context.Canceled
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("父事务失败没有回滚奖励与抵扣", err)
	}
	e.mustExec("CREATE TRIGGER tm_points_test_debt_fail BEFORE INSERT ON tm_points_debt_allocs FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected debt allocation failure'")
	if _, err := s.Grant(e.ctx, g); err == nil {
		t.Fatal("真实分摊故障未传播")
	}
	e.mustExec("DROP TRIGGER tm_points_test_debt_fail")
	if !reflect.DeepEqual(before, snapshot()) || e.queryInt("SELECT outstanding FROM tm_points_debts") != 7 {
		t.Fatal("分摊失败留下奖励或更改欠款")
	}
	requirePointAudit(t, e, s)
	offsetReward, err := s.Grant(e.ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	requirePointAudit(t, e, s)
	// 复刻已经完成过期记账的历史，不把它当作周期清扫器的验收。
	g.Key, g.BizID, g.Amount = "test_grant_expirybasis", "grant:expirybasis", 8
	expiry := now.Add(-time.Hour)
	g.EffectiveAt, g.ExpiresAt = now.Add(-2*time.Hour), &expiry
	old, err := s.Grant(e.ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(`INSERT INTO tm_points_txns(user_id,idem_key,request_hash,kind,biz_type,biz_id,created_at,updated_at) VALUES(1,'test_expiry_basis001',?,'expire','lot_expiry','expiry:history',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, strings.Repeat("a", 64))
	e.mustExec(`INSERT INTO tm_points_entries(user_id,txn_id,lot_id,bucket,delta,created_at,updated_at) VALUES(1,(SELECT id FROM tm_points_txns WHERE idem_key='test_expiry_basis001'),?,1,-8,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, old.LotID)
	e.mustExec("UPDATE tm_points_lots SET remain=0 WHERE id=?", old.LotID)
	e.mustExec("UPDATE tm_points_accounts SET avail=avail-8 WHERE user_id=1")
	requirePointAudit(t, e, s)
	if _, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 1, GrantTxnID: old.TxnID, Key: "test_reverse_expired", BizID: "refund:expired"}); err != nil {
		t.Fatal(err)
	}
	if e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 0 {
		t.Fatal("已记账过期额被再次记欠款")
	}
	requirePointAudit(t, e, s)
	if _, err := s.Reverse(e.ctx, points.ReverseRequest{UserID: 1, GrantTxnID: offsetReward.TxnID, Key: "test_reverse_offsetreward", BizID: "refund:offsetreward"}); err != nil {
		t.Fatal(err)
	}
	requirePointAudit(t, e, s)
	// 人工减少欠款快照且同步减少账户，使两者汇总相等；分摊恒等式仍须发现破坏。
	e.mustExec("UPDATE tm_points_debts SET outstanding=outstanding-1 WHERE outstanding>0")
	e.mustExec("UPDATE tm_points_accounts SET debt=debt-1 WHERE user_id=1")
	audit, err := s.Reconcile(e.ctx, 1)
	if err != nil || audit.Healthy() {
		t.Fatal("欠款与账户一起破坏未检出", err)
	}
}

func TestIntegrationPointsDebtConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, s, now := pointsFixture(t, level)
			original, err := s.Grant(e.ctx, pointGrant(now, 20))
			if err != nil {
				t.Fatal(err)
			}
			f, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Finalize(e.ctx, points.FinalizeRequest{UserID: 1, Key: "test_settle_00001", HoldID: f.HoldID, State: points.Settled}); err != nil {
				t.Fatal(err)
			}
			r := points.ReverseRequest{UserID: 1, GrantTxnID: original.TxnID, Key: "test_reverse_0001", BizID: "refund:1"}
			parallel := func(action func(int) error) {
				start, failures := make(chan struct{}), make(chan error, 40)
				var wg sync.WaitGroup
				for n := 0; n < 40; n++ {
					wg.Add(1)
					go func(n int) {
						defer wg.Done()
						select {
						case <-start:
						case <-e.ctx.Done():
							failures <- e.ctx.Err()
							return
						}
						if err := action(n); err != nil {
							failures <- err
						}
					}(n)
				}
				close(start)
				wg.Wait()
				close(failures)
				for err := range failures {
					t.Error(err)
				}
			}
			receipts := make(chan points.Receipt, 40)
			parallel(func(_ int) error {
				out, err := s.Reverse(e.ctx, r)
				if err == nil {
					receipts <- out
				}
				return err
			})
			close(receipts)
			var first points.Receipt
			for out := range receipts {
				if first.TxnID == 0 {
					first = out
				}
				if out != first {
					t.Fatal("并发冲正收据不一致")
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='reversal'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_debts") != 1 {
				t.Fatal("并发冲正重复")
			}
			parallel(func(n int) error {
				changed := r
				changed.Key = fmt.Sprintf("test_reverse_new_%03d", n)
				_, err := s.Reverse(e.ctx, changed)
				if !errors.Is(err, points.ErrConflict) {
					return fmt.Errorf("换键重复冲正未拒绝: %w", err)
				}
				return nil
			})
			parallel(func(n int) error {
				g := pointGrant(now, 1)
				g.Key = fmt.Sprintf("test_reward_new_%03d", n)
				g.BizID = fmt.Sprintf("reward:%d", n)
				_, err := s.Grant(e.ctx, g)
				return err
			})
			if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 33 || e.queryInt("SELECT debt FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='debt_offset'") != 7 || e.queryInt("SELECT SUM(amount) FROM tm_points_debt_allocs") != 7 {
				t.Fatal("并发奖励多抵扣或绕过欠款")
			}
			requirePointAudit(t, e, s)
		})
	}
}
