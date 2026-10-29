//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func pointsFixture(t *testing.T, level sql.IsolationLevel) (*pointsTestEnv, *sql.DB, *repository.PointsRepo, *service.PointsService, time.Time) {
	return pointsFixtureWithLocal(t, level, false)
}
func pointsFixtureWithLocal(t *testing.T, level sql.IsolationLevel, local bool) (*pointsTestEnv, *sql.DB, *repository.PointsRepo, *service.PointsService, time.Time) {
	t.Helper()
	e, db := setupPointsLedgerWithLocal(t, local)
	db.SetMaxOpenConns(16)
	repo := repository.NewPointsRepoWithIsolation(db, level)
	s, err := service.NewPointsService(repo)
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := e.conn.QueryRowContext(e.ctx, "SELECT CAST(UTC_TIMESTAMP(3) AS CHAR)").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	now, err := time.Parse("2006-01-02 15:04:05.999999", raw)
	if err != nil {
		t.Fatal(err)
	}
	return e, db, repo, s, now
}
func pointGrant(now time.Time, amount int64) points.GrantRequest {
	return points.GrantRequest{UserID: 1, Key: "test_grant_0000001", BizID: "grant-1", Amount: amount, Source: points.Gift, EffectiveAt: now.Add(-time.Minute)}
}
func pointFreeze(now time.Time, number int) points.FreezeRequest {
	return points.FreezeRequest{UserID: 1, Key: fmt.Sprintf("test_freeze_%06d", number), BizType: "ai_task", BizID: fmt.Sprintf("task-%d", number), PriceVersion: "v1", ExpiresAt: now.Add(time.Hour)}
}
func requirePointAudit(t *testing.T, e *pointsTestEnv, s *service.PointsService) {
	t.Helper()
	report, err := s.Reconcile(e.ctx, 1)
	if err != nil || !report.Healthy() {
		t.Fatalf("账本对账失败: %+v；%v", report, err)
	}
}

func TestIntegrationPointsRoundTripAndReplay(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		for _, state := range []int{points.Settled, points.Released, points.Expired} {
			t.Run(fmt.Sprintf("%s_%d", level, state), func(t *testing.T) {
				e, _, _, s, now := pointsFixture(t, level)
				grantReq := pointGrant(now, 20)
				grant, err := s.Grant(e.ctx, grantReq)
				if err != nil {
					t.Fatal(err)
				}
				freezeReq := pointFreeze(now, 1)
				if state == points.Expired {
					freezeReq.ExpiresAt = now.Add(2 * time.Second)
				}
				frozen, err := s.Freeze(e.ctx, freezeReq, points.Price{Version: "v1", Points: 7})
				if err != nil {
					t.Fatal(err)
				}
				if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 13 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
					t.Fatal("冻结账户错误")
				}
				requirePointAudit(t, e, s)
				again, err := s.Grant(e.ctx, grantReq)
				if err != nil || again != grant {
					t.Fatal("入账重放收据变化", err)
				}
				again, err = s.Freeze(e.ctx, freezeReq, points.Price{Version: "v2", Points: 99})
				if err != nil || again != frozen {
					t.Fatal("价格变化后未返回首次冻结结果", err)
				}
				if state == points.Expired {
					timer := time.NewTimer(time.Until(freezeReq.ExpiresAt) + 20*time.Millisecond)
					defer timer.Stop()
					select {
					case <-timer.C:
					case <-e.ctx.Done():
						t.Fatal(e.ctx.Err())
					}
				}
				finalReq := points.FinalizeRequest{UserID: 1, Key: "test_final_0000001", HoldID: frozen.HoldID, State: state}
				final, err := s.Finalize(e.ctx, finalReq)
				if err != nil {
					t.Fatal(err)
				}
				again, err = s.Finalize(e.ctx, finalReq)
				if err != nil || again != final {
					t.Fatal("终态重放变化", err)
				}
				if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 3 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
					t.Fatal("重放新增流水或遗留冻结")
				}
				want := int64(20)
				if state == points.Settled {
					want = 13
				}
				if int64(e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1")) != want {
					t.Fatal("终结余额错误")
				}
				requirePointAudit(t, e, s)
				finalReq.Key = "test_final_0000002"
				if _, err := s.Finalize(e.ctx, finalReq); !errors.Is(err, points.ErrClosed) {
					t.Fatal("冻结单重复终结未拒绝", err)
				}
			})
		}
	}
}

func TestIntegrationPointsTwoHundredConcurrentFreezes(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, s, now := pointsFixture(t, level)
			if _, err := s.Grant(e.ctx, pointGrant(now, 100)); err != nil {
				t.Fatal(err)
			}
			var success atomic.Int64
			start := make(chan struct{})
			var wg sync.WaitGroup
			failures := make(chan error, 200)
			for n := 0; n < 200; n++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					select {
					case <-start:
					case <-e.ctx.Done():
						return
					}
					_, err := s.Freeze(e.ctx, pointFreeze(now, n), points.Price{Version: "v1", Points: 7})
					if err == nil {
						success.Add(1)
					} else if !errors.Is(err, points.ErrInsufficient) {
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
			if success.Load() != 14 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 2 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 98 || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 14 {
				t.Fatalf("200 并发超扣或少冻: 成功=%d", success.Load())
			}
			requirePointAudit(t, e, s)
		})
	}
}

func TestIntegrationPointsIdempotencyAndTerminalRace(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, s, now := pointsFixture(t, level)
			ctx := e.ctx
			var wg sync.WaitGroup
			receipts := make(chan points.Receipt, 40)
			failures := make(chan error, 40)
			start := make(chan struct{})
			for n := 0; n < 40; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					r, err := s.Grant(ctx, pointGrant(now, 100))
					if err != nil {
						failures <- err
					} else {
						receipts <- r
					}
				}()
			}
			close(start)
			wg.Wait()
			close(receipts)
			close(failures)
			for err := range failures {
				t.Error(err)
			}
			var original points.Receipt
			for r := range receipts {
				if original.TxnID == 0 {
					original = r
				}
				if r != original {
					t.Fatal("并发幂等返回不一致")
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_lots") != 1 {
				t.Fatal("并发入账重复")
			}
			r := pointGrant(now, 101)
			if _, err := s.Grant(ctx, r); !errors.Is(err, points.ErrIdempotency) {
				t.Fatal("同键不同请求未拒绝", err)
			}
			f, err := s.Freeze(ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
			if err != nil {
				t.Fatal(err)
			}
			var success atomic.Int64
			start = make(chan struct{})
			failures = make(chan error, 40)
			for n := 0; n < 40; n++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					<-start
					state := points.Settled
					if n%2 == 0 {
						state = points.Released
					}
					_, err := s.Finalize(ctx, points.FinalizeRequest{UserID: 1, Key: fmt.Sprintf("test_final_%06d", n), HoldID: f.HoldID, State: state})
					if err == nil {
						success.Add(1)
					} else if !errors.Is(err, points.ErrClosed) {
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
			if success.Load() != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 3 {
				t.Fatal("结算/释放竞态产生多个终态")
			}
			requirePointAudit(t, e, s)
		})
	}
}

func TestIntegrationPointsGuardsParentRollbackAndCorruption(t *testing.T) {
	e, _, repo, s, now := pointsFixture(t, sql.LevelRepeatableRead)
	ctx := e.ctx
	g := pointGrant(now, 20)
	grant, err := s.Grant(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		price points.Price
		want  error
	}{{points.Price{Version: "v2", Points: 7}, points.ErrPriceChanged}, {points.Price{Version: "v1", Points: 21}, points.ErrInsufficient}} {
		if _, err := s.Freeze(ctx, pointFreeze(now, 1), tc.price); !errors.Is(err, tc.want) {
			t.Fatal("守卫失败", err)
		}
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 1 {
		t.Fatal("报价/余额失败留下流水")
	}
	duplicate := g
	duplicate.Key = "test_grant_otherkey"
	if _, err := s.Grant(ctx, duplicate); !errors.Is(err, points.ErrConflict) {
		t.Fatal("同一业务编号换键重复入账未拒绝", err)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 1 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 {
		t.Fatal("重复业务改变余额")
	}
	fault := fmt.Errorf("测试模拟上层任务写入失败")
	err = repo.WithinPointsTx(ctx, func(tx points.Tx) error {
		if _, err := s.FreezeInTx(ctx, tx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7}); err != nil {
			return err
		}
		return fault
	})
	if !errors.Is(err, fault) || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 0 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 {
		t.Fatal("上层任务失败未回滚冻结", err)
	}
	e.mustExec("CREATE TRIGGER tm_points_test_fail BEFORE INSERT ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected ledger failure'")
	_, err = s.Freeze(ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
	if err == nil {
		t.Fatal("分录故障未传播")
	}
	e.mustExec("DROP TRIGGER tm_points_test_fail")
	if e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 1 {
		t.Fatal("真实分录故障遗留冻结或流水")
	}
	requirePointAudit(t, e, s)
	// 账面余额仍含尚未生效和已过期批次，但这些批次不能参与新冻结。
	future := pointGrant(now.Add(24*time.Hour), 5)
	future.Key = "test_grant_future1"
	future.BizID = "future"
	if _, err := s.Grant(ctx, future); err != nil {
		t.Fatal(err)
	}
	expired := pointGrant(now.Add(-24*time.Hour), 5)
	expiry := now.Add(-time.Hour)
	expired.ExpiresAt = &expiry
	expired.Key = "test_grant_expired1"
	expired.BizID = "expired"
	if _, err := s.Grant(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Freeze(ctx, pointFreeze(now, 2), points.Price{Version: "v1", Points: 21}); !errors.Is(err, points.ErrInsufficient) {
		t.Fatal("不可用批次被冻结", err)
	}
	if _, err := s.Freeze(ctx, pointFreeze(now, 2), points.Price{Version: "v1", Points: 7}); err != nil {
		t.Fatal(err)
	}
	wrongUser := pointFreeze(now, 3)
	wrongUser.UserID = 2
	if _, err := s.Freeze(ctx, wrongUser, points.Price{Version: "v1", Points: 1}); !errors.Is(err, points.ErrInsufficient) {
		t.Fatal(err)
	}
	requirePointAudit(t, e, s)
	e.mustExec("UPDATE tm_points_accounts SET avail=avail+1 WHERE user_id=1")
	audit, err := s.Reconcile(ctx, 1)
	if err != nil || audit.Healthy() {
		t.Fatal("人工破坏未检出", err)
	}
	e.mustExec("UPDATE tm_points_accounts SET avail=avail-1 WHERE user_id=1")
	e.mustExec("UPDATE tm_points_lots SET remain=remain-1 WHERE id=?", grant.LotID)
	audit, err = s.Reconcile(ctx, 1)
	if err != nil || audit.Healthy() {
		t.Fatal("批次破坏未检出", err)
	}
	// 金额上限由服务先拦截，而不是依赖驱动/数据库报溢出。
	big := pointGrant(now, math.MaxInt64)
	big.UserID = 9
	big.Key = "test_grant_maximum1"
	big.BizID = "maximum"
	if _, err := s.Grant(ctx, big); err != nil {
		t.Fatal(err)
	}
	big.Key = "test_grant_overflow"
	big.BizID = "overflow"
	big.Amount = 1
	if _, err := s.Grant(ctx, big); !errors.Is(err, points.ErrOverflow) {
		t.Fatal("整数溢出未拒绝", err)
	}
}

func TestIntegrationPointsMultipleLotsAndTerminalRollback(t *testing.T) {
	e, _, repo, s, now := pointsFixture(t, sql.LevelReadCommitted)
	g := pointGrant(now, 5)
	g.Priority = 2
	first, err := s.Grant(e.ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	g.Key = "test_grant_0000002"
	g.BizID = "grant-2"
	g.Priority = 1
	second, err := s.Grant(e.ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
	if err != nil {
		t.Fatal(err)
	}
	if e.queryInt("SELECT frozen FROM tm_points_lots WHERE id=?", second.LotID) != 5 || e.queryInt("SELECT frozen FROM tm_points_lots WHERE id=?", first.LotID) != 2 {
		t.Fatal("批次优先级/跨批次分摊错误")
	}
	before := []int{e.queryInt("SELECT COUNT(*) FROM tm_points_txns"), e.queryInt("SELECT COUNT(*) FROM tm_points_entries"), e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1")}
	finalReq := points.FinalizeRequest{UserID: 1, Key: "test_final_0000001", HoldID: f.HoldID, State: points.Settled}
	err = repo.WithinPointsTx(e.ctx, func(tx points.Tx) error {
		if _, err := s.FinalizeInTx(e.ctx, tx, finalReq); err != nil {
			return err
		}
		return context.Canceled
	})
	after := []int{e.queryInt("SELECT COUNT(*) FROM tm_points_txns"), e.queryInt("SELECT COUNT(*) FROM tm_points_entries"), e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1")}
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(before, after) {
		t.Fatal("报告事务失败后没有回滚结算", err)
	}
	requirePointAudit(t, e, s)
	if _, err := s.Finalize(e.ctx, finalReq); err != nil {
		t.Fatal(err)
	}
	requirePointAudit(t, e, s)
}

func TestIntegrationPointsDriverLocationCannotShiftUTCLedger(t *testing.T) {
	e, _, _, _, now := pointsFixture(t, sql.LevelReadCommitted)
	for n, name := range []string{"Asia/Shanghai", "America/New_York"} {
		t.Run(name, func(t *testing.T) {
			zone, err := time.LoadLocation(name)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
			if err != nil {
				t.Fatal("隔离 DSN 解析失败")
			}
			cfg.Loc = zone
			cfg.DBName = e.database
			if cfg.Params == nil {
				cfg.Params = make(map[string]string)
			}
			cfg.Params["time_zone"] = "'+08:00'"
			db, err := sql.Open("mysql", cfg.FormatDSN())
			if err != nil {
				t.Fatal("隔离连接装配失败")
			}
			defer db.Close()
			s, err := service.NewPointsService(repository.NewPointsRepo(db))
			if err != nil {
				t.Fatal(err)
			}
			g := pointGrant(now, 10)
			g.UserID = int64(n + 10)
			g.Key = fmt.Sprintf("test_zone_grant_%03d", n)
			g.BizID = fmt.Sprintf("zone-grant-%d", n)
			g.EffectiveAt = g.EffectiveAt.In(zone)
			grant, err := s.Grant(e.ctx, g)
			if err != nil {
				t.Fatal(err)
			}
			var actual string
			if err := e.conn.QueryRowContext(e.ctx, "SELECT CAST(eff_at AS CHAR) FROM tm_points_lots WHERE id=?", grant.LotID).Scan(&actual); err != nil || actual != g.EffectiveAt.UTC().Format("2006-01-02 15:04:05.000") {
				t.Fatal("驱动 loc 改变 UTC 存储", actual, err)
			}
			f := pointFreeze(now, n)
			f.UserID = g.UserID
			f.Key = fmt.Sprintf("test_zone_freeze_%03d", n)
			f.BizID = fmt.Sprintf("zone-freeze-%d", n)
			f.ExpiresAt = f.ExpiresAt.In(zone)
			freeze, err := s.Freeze(e.ctx, f, points.Price{Version: "v1", Points: 7})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Finalize(e.ctx, points.FinalizeRequest{UserID: g.UserID, Key: fmt.Sprintf("test_zone_final_%03d", n), HoldID: freeze.HoldID, State: points.Settled}); err != nil {
				t.Fatal(err)
			}
			g.EffectiveAt = g.EffectiveAt.UTC()
			if replay, err := s.Grant(e.ctx, g); err != nil || replay != grant {
				t.Fatal("同一时刻不同输入时区改变请求摘要", err)
			}
			report, err := s.Reconcile(e.ctx, g.UserID)
			if err != nil || !report.Healthy() {
				t.Fatal(report, err)
			}
		})
	}
}
