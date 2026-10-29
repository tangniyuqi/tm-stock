//go:build integration

package points_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 只观测原30秒worker的自然执行期限；不改任务300秒预算、一分钟租约或数据库时钟。
func (g *h5ReportGateway) confirmWorkerTimeout(t *testing.T) task.Task {
	t.Helper()
	if g.workerDone == nil {
		t.Fatal("尚未启动原worker")
	}
	select {
	case <-g.workerDone:
		if !errors.Is(g.workerErr, context.DeadlineExceeded) {
			t.Fatal("worker没有自然超时", g.workerErr)
		}
	case <-g.e.ctx.Done():
		t.Fatal("等待原worker超时失败", g.e.ctx.Err())
	}
	v, err := g.e.tasks.Inspect(g.e.ctx, 1, g.e.v.ID)
	if err != nil || v.Status != task.Running || v.LeaseUntil == nil || v.CancelRequested || v.Terminal() || g.e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || g.e.submits.Load() != 1 || g.e.server.Count() != 0 {
		t.Fatal("执行超时被冒充为任务终态、退款或重新执行", err)
	}
	r, err := g.e.agent.Show(g.e.ctx, g.e.planner.p.Agent.ActorID, g.e.planner.p.Agent.RequestID())
	if err != nil || r.State != agents.Running || r.CancelRequested {
		t.Fatal("本地执行超时篡改了上游运行事实", err)
	}
	t.Log("原worker已自然超时；原任务仍Running、测试冻结7、CN提交1、模型0；尚未取消或释放")
	return v
}

// 原恢复服务只在实际租约到期后接管已登记的取消。计划器不应被调用，更不提供外部执行器。
func (g *h5ReportGateway) recoverTimeoutCancellation(t *testing.T) {
	t.Helper()
	select {
	case <-g.workerDone:
		if !errors.Is(g.workerErr, context.DeadlineExceeded) {
			t.Fatal("取消必须发生在原worker自然超时之后", g.workerErr)
		}
	default:
		t.Fatal("原worker仍运行，不能算超时恢复")
	}
	v, err := g.e.tasks.Inspect(g.e.ctx, 1, g.e.v.ID)
	if err != nil || v.Status != task.Running || !v.CancelRequested || v.LeaseUntil == nil || !g.cancelAck || g.e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
		t.Fatal("取消回执没有保留原运行事实与冻结", err)
	}
	planner := recoveryPlanFunc(func(context.Context, task.Task) (task.RecoveryPlan, error) {
		t.Error("已登记取消的恢复不应读取资料或生成新计划")
		return task.RecoveryPlan{}, errCheckTest
	})
	w, err := service.NewAIRecoveryWorker(g.e.tasks, g.e.repo, planner, "timeout_recovery_0002", task.RecoveryOptions{Lease: time.Second, Heartbeat: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// 用数据库真实当前时点证明早抢失败，不能用客户端等待或手工回拨租约作证。
	var now time.Time
	if err := g.e.db.QueryRowContext(g.e.ctx, "SELECT UTC_TIMESTAMP(3)").Scan(&now); err != nil || !now.Before(*v.LeaseUntil) {
		t.Fatal("未在原租约有效窗口验证拒绝提前接管", err)
	}
	if _, err := w.Recover(g.e.ctx, 1, v.ID); !errors.Is(err, task.ErrNotReady) {
		t.Fatal("有效租约被提前接管", err)
	}
	t.Log("取消已登记但非终态；原恢复服务拒绝提前接管，冻结仍保留；等待数据库原租约自然到期")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for now.Before(*v.LeaseUntil) {
		select {
		case <-g.e.ctx.Done():
			t.Fatal("原租约等待被取消", g.e.ctx.Err())
		case <-ticker.C:
			if err := g.e.db.QueryRowContext(g.e.ctx, "SELECT UTC_TIMESTAMP(3)").Scan(&now); err != nil {
				t.Fatal("读取真实数据库时间失败", err)
			}
		}
	}
	before := g.e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
	for range 2 {
		out, err := w.Recover(g.e.ctx, 1, v.ID)
		if err != nil || out.TaskID != v.ID || out.TaskStatus != task.Cancelled || out.Outcome != "terminal" || out.ReportAvailable {
			t.Fatal("原恢复服务未确认本地取消终态", out, err)
		}
	}
	if _, err := g.e.tasks.Heartbeat(g.e.ctx, 1, v.ID, v.LeaseOwner, v.Fence, time.Minute); !errors.Is(err, task.ErrLease) {
		t.Fatal("已回收的旧worker租约仍可写", err)
	}
	g.e.run(t) // 原L1终态重放仍不能执行或再次结算/释放。
	if g.e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before+1 || g.e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || g.e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 || g.e.submits.Load() != 1 || g.e.server.Count() != 0 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 || g.e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 0 {
		t.Fatal("恢复重放产生重复账务、外部提交或报告")
	}
	requirePointAudit(t, g.e.pointsTestEnv, g.e.ledger)
	g.timeoutRecovered = true
	t.Log("原租约自然到期后恢复确认Cancelled；恰好一次释放、旧租约拒写、恢复及L1终态重放零额外调用；账本审计通过")
}
