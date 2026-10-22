//go:build integration

package points_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func TestIntegrationDeepIntentPrepareConfirm(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := intentSQLEnvironment(t, level, "pass")
			e.ctl.p.Scope.AnalysisType, e.ctl.p.Price.Points = task.DeepAnalysisType, task.DeepPricePoints
			e.ctl.allowDeep.Store(true)
			e.ctl.allowed.Store(true)
			limiter, err := service.NewSQLDeepLimiter(e.db)
			if err != nil {
				t.Fatal(err)
			}
			e.creator.AITaskCreateService.SetDeepLimiter(limiter)
			now := time.Now().UTC().Truncate(time.Millisecond)
			grant := points.GrantRequest{UserID: 1, Key: "test_deep_intent_grant", BizID: "deep-intent-grant", Amount: 30, Source: points.Gift, EffectiveAt: now.Add(-time.Minute)}
			if _, err := e.ledger.Grant(e.ctx, grant); err != nil {
				t.Fatal(err)
			}
			intent, err := e.svc.Prepare(e.ctx, 1, e.cid, "test_deep_intent_key_001", conversation.IntentInput{AnalysisType: "deep", SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"})
			if err != nil || intent.State != conversation.Prepared || intent.PricePoints != task.DeepPricePoints || intent.AnalysisType != "deep" {
				t.Fatal("深度意图准备失败", intent, err)
			}
			if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("准备阶段不应冻结")
			}
			linked, err := e.svc.Confirm(e.ctx, 1, e.cid, intent.ID)
			if err != nil || linked.State != conversation.Linked || linked.TaskID <= 0 {
				t.Fatal("深度意图确认失败", linked, err)
			}
			var analysisType int
			var price int64
			if err := e.db.QueryRowContext(e.ctx, "SELECT analysis_type,price_points FROM tm_ai_tasks WHERE id=?", linked.TaskID).Scan(&analysisType, &price); err != nil {
				t.Fatal(err)
			}
			if analysisType != task.DeepAnalysisType || price != task.DeepPricePoints || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != task.DeepPricePoints {
				t.Fatal("深度任务未按30积分冻结", analysisType, price)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks WHERE analysis_type=2 AND status IN (1,2)") != 1 {
				t.Fatal("第一个深度任务未计入并发")
			}
			if release, err := limiter.Acquire(e.ctx, 1); err == nil {
				release()
				t.Fatal("直接并发检查未拒绝第二个深度任务")
			}
			// 同一用户第二个深度意图在并发限流处拒绝，不冻结。
			second, err := e.svc.Prepare(e.ctx, 1, e.cid, "test_deep_intent_key_002", conversation.IntentInput{AnalysisType: "deep", SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"})
			if err != nil || second.State != conversation.Prepared {
				t.Fatal("第二个深度意图准备失败", second, err)
			}
			secondLinked, err := e.svc.Confirm(e.ctx, 1, e.cid, second.ID)
			if err != nil || secondLinked.State != conversation.NotCreated || secondLinked.RejectionCode != "admission_unavailable" || secondLinked.TaskID != 0 {
				t.Fatal("同用户第二个深度任务未被并发限制拒绝", err, secondLinked)
			}
			frozen := e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1")
			if frozen != task.DeepPricePoints {
				t.Fatal("拒绝后冻结被额外占用", frozen)
			}
		})
	}
}
