//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 只同步首次范围读取与测试规划器绑定；分页和后续扫描仍走原仓储及真实时钟。
type loopObservedScanner struct {
	task.MemberQueueScanner
	started chan struct{}
	ready   chan struct{}
	rounds  atomic.Int32
}

func (s *loopObservedScanner) MemberQueueRange(ctx context.Context) (task.MemberQueueRange, error) {
	out, err := s.MemberQueueScanner.MemberQueueRange(ctx)
	if s.rounds.Add(1) == 1 {
		close(s.started)
		select {
		case <-s.ready:
		case <-ctx.Done():
			return out, ctx.Err()
		}
	}
	return out, err
}

func TestIntegrationAIReportQueueLoop(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "success")
			scan := &loopObservedScanner{MemberQueueScanner: e.repo, started: make(chan struct{}), ready: make(chan struct{})}
			q, err := service.NewAIL1ReportQueue(admissionWorker(t, e), scan, true)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(e.ctx, 15*time.Second)
			done := make(chan struct{})
			var runErr error
			go func() { defer close(done); runErr = q.RunLoop(ctx, 10, time.Second) }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("队列循环未退出")
				}
			})
			select {
			case <-scan.started:
			case <-ctx.Done():
				t.Fatal("首次扫描未启动")
			}
			token := e.web.token(t, 1, "client")
			created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, token, 200))
			e.bindNewTask(t, created.TaskID)
			close(scan.ready)
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			for {
				v, err := e.tasks.Inspect(ctx, 1, created.TaskID)
				if err != nil {
					t.Fatal(err)
				}
				if v.Status == task.Succeeded && scan.rounds.Load() >= 4 {
					break
				}
				select {
				case <-ticker.C:
				case <-done:
					t.Fatal("队列提前退出", runErr)
				case <-ctx.Done():
					t.Fatal("循环未处理新任务及后续重扫")
				}
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("取消未结束循环")
			}
			if !errors.Is(runErr, context.Canceled) {
				t.Fatal(runErr)
			}
			if e.submits.Load() != 1 || e.server.Count() != 3 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("循环重扫产生额外执行或账务")
			}
			body := e.mustRequest(t, "GET", "/api/v1/ai/tasks/"+jsonNumber(created.TaskID)+"/report", "", "", token, 200)
			assertPublicReport(t, body, e.reportEnv)
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}
