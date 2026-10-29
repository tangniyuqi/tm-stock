//go:build integration

package points_test

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 验证报告报价共用的新账本余额读取；不执行模型或报告生成。
func TestIntegrationAIReportBalanceRead(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, svc, now := pointsFixture(t, level)
			g := pointGrant(now, 20)
			if _, err := svc.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			hold, err := svc.Freeze(e.ctx, pointFreeze(now, 1), points.Price{Version: "v1", Points: 7})
			if err != nil {
				t.Fatal(err)
			}
			// 尚未生效和已经过期的批次仍计入汇总，但不能算作当前可消费额。
			g.Key, g.BizID, g.Amount = "test_grant_future1", "future", 5
			g.EffectiveAt = now.Add(time.Hour)
			if _, err := svc.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			g.Key, g.BizID, g.Amount = "test_grant_expired1", "expired", 4
			g.EffectiveAt = now.Add(-2 * time.Hour)
			expired := now.Add(-time.Hour)
			g.ExpiresAt = &expired
			if _, err := svc.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			snapshot := func() []int {
				return []int{
					e.queryInt("SELECT COUNT(*) FROM tm_points_accounts"),
					e.queryInt("SELECT COUNT(*) FROM tm_points_txns"),
					e.queryInt("SELECT COUNT(*) FROM tm_points_entries"),
					e.queryInt("SELECT SUM(remain) FROM tm_points_lots"),
					e.queryInt("SELECT SUM(frozen) FROM tm_points_lots"),
					e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1"),
					e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1"),
				}
			}
			before := snapshot()
			for i := 0; i < 2; i++ {
				balance, err := svc.Balance(e.ctx, 1)
				if err != nil || balance.Available != 13 || balance.Frozen != 7 || balance.CheckedAt < now.UnixMilli() {
					t.Fatal(balance, err)
				}
				missing, err := svc.Balance(e.ctx, 2)
				if err != nil || missing.Available != 0 || missing.Frozen != 0 || missing.CheckedAt < now.UnixMilli() {
					t.Fatal(missing, err)
				}
			}
			if !reflect.DeepEqual(before, snapshot()) {
				t.Fatal("余额查询创建账户、清扫批次或改变账本")
			}
			if _, err := svc.Finalize(e.ctx, points.FinalizeRequest{UserID: 1, Key: "test_balance_settle1", HoldID: hold.HoldID, State: points.Settled}); err != nil {
				t.Fatal(err)
			}
			balance, err := svc.Balance(e.ctx, 1)
			if err != nil || balance.Available != 13 || balance.Frozen != 0 {
				t.Fatal("结算后余额错误", balance, err)
			}
			requirePointAudit(t, e, svc)
		})
	}
}
