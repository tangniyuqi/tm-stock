//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

const deepIntentBody = "{\"analysis_type\":\"deep\",\"subject_type\":\"theme\",\"subject_id\":1,\"price_version\":\"v1\"}"

func TestIntegrationDeepIntentHTTPPrepareConfirm(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := intentSQLEnvironment(t, level, "pass")
			e.ctl.p.Scope.AnalysisType, e.ctl.p.Price.Points, e.ctl.p.Budget.MaxCalls = task.DeepAnalysisType, task.DeepPricePoints, 10
			e.ctl.allowDeep.Store(true)
			e.ctl.allowed.Store(true)
			limiter, err := service.NewSQLDeepLimiter(e.db)
			if err != nil {
				t.Fatal(err)
			}
			e.creator.AITaskCreateService.SetDeepLimiter(limiter)
			now := time.Now().UTC().Truncate(time.Millisecond)
			suffix := fmt.Sprintf("%d", now.UnixNano())
			if _, err := e.ledger.Grant(e.ctx, points.GrantRequest{UserID: 1, Key: "test_deep_http_" + suffix, BizID: "deep-http-" + suffix, Amount: 30, Source: points.Gift, EffectiveAt: now.Add(-time.Minute)}); err != nil {
				t.Fatal(err)
			}
			var prepared struct {
				Data dto.AIConversationTask `json:"data"`
			}
			if json.Unmarshal(e.mustRequest(t, "POST", e.path+"/task-intents", "deep_http_intent_key", deepIntentBody, e.token, 200), &prepared) != nil || prepared.Data.State != "prepared" || prepared.Data.PricePoints != task.DeepPricePoints || prepared.Data.AnalysisType != "deep" {
				t.Fatal("深度意图HTTP准备失败", prepared)
			}
			var linked struct {
				Data dto.AIConversationTask `json:"data"`
			}
			confirmPath := fmt.Sprintf("%s/task-intents/%d/confirm", e.path, prepared.Data.ID)
			if json.Unmarshal(e.mustRequest(t, "POST", confirmPath, "", "{\"confirmed\":true}", e.token, 200), &linked) != nil || linked.Data.State != "linked" || linked.Data.TaskID <= 0 {
				t.Fatal("深度意图HTTP确认失败", linked)
			}
			v, err := e.tasks.Inspect(e.ctx, 1, linked.Data.TaskID)
			if err != nil || v.AnalysisType != task.DeepAnalysisType || v.PricePoints != task.DeepPricePoints || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != task.DeepPricePoints {
				t.Fatal("深度任务未按30冻结", v, err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE task_id="+fmt.Sprintf("%d", v.ID)) != 0 {
				t.Fatal("深度意图确认后不应已有报告")
			}
		})
	}
}
