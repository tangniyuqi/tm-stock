//go:build integration

package points_test

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func TestIntegrationDeepLimiterAtomic(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newWorkflowEnv(t, level, "pass")
			if _, err := e.tasks.Cancel(e.ctx, 1, e.v.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := e.tasks.FailOrRetry(e.ctx, 1, e.v.ID, e.v.LeaseOwner, e.v.Fence, "internal_failure", 0); err != nil {
				t.Fatal(err)
			}
			limiter, err := service.NewSQLDeepLimiter(e.db)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Millisecond)
			for uid := int64(1); uid <= 6; uid++ {
				g := points.GrantRequest{UserID: uid, Key: fmt.Sprintf("test_slot_grant_%02d", uid), BizID: fmt.Sprintf("slot-%d", uid), Amount: 40, Source: points.Gift, EffectiveAt: now.Add(-time.Minute)}
				if _, err := e.ledger.Grant(e.ctx, g); err != nil {
					t.Fatal("授予测试积分失败", uid, err)
				}
			}
			budget := taskBudget()
			budget.MaxCalls, budget.MaxTokens, budget.MaxDurationMS = 10, 12000, 300000
			var wg sync.WaitGroup
			outcomes := make([]error, 6)
			for uid := int64(1); uid <= 6; uid++ {
				wg.Add(1)
				go func(uid int64) {
					defer wg.Done()
					release, err := limiter.Acquire(e.ctx, uid)
					if err != nil {
						outcomes[uid-1] = err
						return
					}
					defer release()
					_, outcomes[uid-1] = e.tasks.Create(e.ctx, task.CreateRequest{UserID: uid, Key: fmt.Sprintf("test_l2_slot_%04d", uid), AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, points.Price{Version: "v1", Points: task.DeepPricePoints}, budget)
				}(uid)
			}
			wg.Wait()
			accepted, rejected := 0, 0
			for _, err := range outcomes {
				if err == nil {
					accepted++
				} else if errors.Is(err, task.ErrAdmissionUnavailable) {
					rejected++
				} else {
					t.Fatal("并发创建返回未预期错误", err)
				}
			}
			if accepted != task.DeepMaxGlobal || rejected != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks WHERE analysis_type=2 AND status IN (1,2)") != task.DeepMaxGlobal {
				t.Fatal("全局深度并发未恰好限制五个", accepted, rejected)
			}
			if release, err := limiter.Acquire(e.ctx, 1); !errors.Is(err, task.ErrAdmissionUnavailable) {
				if release != nil {
					release()
				}
				t.Fatal("同一用户第二个深度任务未被拒绝", err)
			}
		})
	}
}
