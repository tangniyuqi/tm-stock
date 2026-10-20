//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const extractionStageFact = `{"claims":[{"predicate":"DISCLOSES_BUSINESS","entity":"测试星河公司","theme":"算力","sentence_ids":[1]}]}`

func extractionStagePlan(t *testing.T, body string) (pipeline.ExtractionStage, sources.Snapshot, guard.Vocabulary) {
	t.Helper()
	// 纯测试资料和授权占位；不声称生产当前授权或模型质量已验收。
	snap, err := sources.BuildSnapshot(sources.Source{ID: 3, Domain: "exchange.example", Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "fake-stage-only", LicenseRef: "fake-internal-evaluation"}, "https://exchange.example/docs/test", sources.MediaPlain, []byte(body), time.Now().UTC(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	snap.ID = 42
	vocab := guard.Vocabulary{Predicates: map[string]bool{"DISCLOSES_BUSINESS": true}, Themes: map[string][]string{"算力": {"智算"}}}
	plan, err := pipeline.NewExtractionStage(snap, vocab)
	if err != nil {
		t.Fatal(err)
	}
	return plan, snap, vocab
}

func TestIntegrationAITaskStageExtractionPolicy(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, repo, _, budget, reservation := runningBudgetTask(t, level)
			plan, snap, vocab := extractionStagePlan(t, "测试星河公司主营算力服务。")
			reservation.InputDigest = plan.InputDigest()
			if _, err := budget.Reserve(e.ctx, reservation); err != nil {
				t.Fatal(err)
			}
			s, err := service.NewAITaskStageService(repo, map[string]task.StagePolicy{"extract": plan.Policy()})
			if err != nil {
				t.Fatal(err)
			}
			r := task.StageRequest{UserID: 1, TaskID: reservation.TaskID, Fence: reservation.Fence, Owner: reservation.Owner, OperationKey: reservation.Key, Stage: "extract", InputDigest: plan.InputDigest(), StepNo: 1}
			if _, err := s.Complete(e.ctx, r); !errors.Is(err, points.ErrNotFound) {
				t.Fatal("无响应不应调用或推进", err)
			}
			if out, err := s.Capture(e.ctx, r, []byte(extractionStageFact)); err != nil || !out.NewlyCaptured {
				t.Fatal(out, err)
			}
			if done, err := s.Complete(e.ctx, r); err != nil || !done.NewlyCompleted || done.Checkpoint.PolicyDigest != plan.Policy().Digest {
				t.Fatal(done, err)
			}
			// 重新装配相同策略，复用旧结果，不伪造一次新的模型执行。
			plan, err = pipeline.NewExtractionStage(snap, vocab)
			if err != nil {
				t.Fatal(err)
			}
			s, err = service.NewAITaskStageService(repo, map[string]task.StagePolicy{"extract": plan.Policy()})
			if err != nil {
				t.Fatal(err)
			}
			if done, err := s.Complete(e.ctx, r); err != nil || done.NewlyCompleted {
				t.Fatal(done, err)
			}
			loaded, err := s.Load(e.ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			public, err := json.Marshal(loaded)
			if err != nil || strings.Contains(string(public), "测试星河公司") || strings.Contains(string(public), "claims") {
				t.Fatal("隔离结果序列化泄露正文", err)
			}
			vocab.Themes["算力"] = []string{"数据中心"}
			changed, err := pipeline.NewExtractionStage(snap, vocab)
			if err != nil || changed.InputDigest() != r.InputDigest || changed.Policy().Digest == plan.Policy().Digest {
				t.Fatal("策略变化测试基线无效", err)
			}
			s, err = service.NewAITaskStageService(repo, map[string]task.StagePolicy{"extract": changed.Policy()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Load(e.ctx, r); !errors.Is(err, task.ErrStagePolicy) {
				t.Fatal("策略变化后仍复用旧检查点", err)
			}
			if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
				t.Fatal("校验/复用重复推进、预留或记账")
			}
		})
	}
}

func TestIntegrationAITaskStageExtractionRejections(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "ai", "pipeline", "testdata", "stage_quote_rejections.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	trimmedBody := ""
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var fixture struct{ Name, Body, Purpose string }
		if err := json.Unmarshal([]byte(line), &fixture); err != nil || fixture.Purpose != "仅用于拒绝测试，非真实公司披露" {
			t.Fatal("拒绝测试夹具无效", err)
		}
		if fixture.Name == "需要节选" {
			trimmedBody = strings.ReplaceAll(fixture.Body, "星河科技", "测试星河公司")
		}
	}
	if trimmedBody == "" {
		t.Fatal("节选拒绝夹具缺失")
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, tc := range []struct{ name, body, payload string }{
				{"空候选", "测试星河公司主营算力服务。", `{"claims":[]}`},
				{"原始报告", "测试星河公司主营算力服务。", `{"reports":{"research":"测试研究原文"},"verified":true}`},
				{"未知字段", "测试星河公司主营算力服务。", strings.Replace(extractionStageFact, `"predicate":`, `"score":1,"predicate":`, 1)},
				{"无出处", "另一测试公司主营算力服务。", extractionStageFact},
				{"编号越界", "测试星河公司主营算力服务。", strings.Replace(extractionStageFact, `[1]`, `[9999]`, 1)},
				{"非事实谓词", "测试星河公司主营算力服务。", strings.Replace(extractionStageFact, "DISCLOSES_BUSINESS", "RECOMMEND", 1)},
				{"类别错位", "测试星河公司主营算力服务。", strings.Replace(extractionStageFact, "DISCLOSES_BUSINESS", "LISTED_IN_CATALOG", 1)},
				{"重复条目", "测试星河公司主营算力服务。", strings.Replace(extractionStageFact, `}]}`, `},{"predicate":"DISCLOSES_BUSINESS","entity":"测试星河公司","theme":"算力","sentence_ids":[1]}]}`, 1)},
				{"部分失败", "测试星河公司主营算力服务。", strings.Replace(extractionStageFact, `}]}`, `},{"predicate":"DISCLOSES_BUSINESS","entity":"未披露测试公司","theme":"算力","sentence_ids":[1]}]}`, 1)},
				{"需要节选", trimmedBody, extractionStageFact},
			} {
				t.Run(tc.name, func(t *testing.T) {
					e, _, repo, _, budget, reservation := runningBudgetTask(t, level)
					plan, _, _ := extractionStagePlan(t, tc.body)
					reservation.InputDigest = plan.InputDigest()
					if _, err := budget.Reserve(e.ctx, reservation); err != nil {
						t.Fatal(err)
					}
					s, err := service.NewAITaskStageService(repo, map[string]task.StagePolicy{"extract": plan.Policy()})
					if err != nil {
						t.Fatal(err)
					}
					r := task.StageRequest{UserID: 1, TaskID: reservation.TaskID, Fence: reservation.Fence, Owner: reservation.Owner, OperationKey: reservation.Key, Stage: "extract", InputDigest: plan.InputDigest(), StepNo: 1}
					if _, err := s.Capture(e.ctx, r, []byte(tc.payload)); err != nil {
						t.Fatal("合法 JSON 的隔离捕获失败", err)
					}
					for i := 0; i < 2; i++ {
						if out, err := s.Complete(e.ctx, r); err != task.ErrStageRejected || out != (task.CompletedStage{}) {
							t.Fatal("真实守卫未固定拒绝", out, err)
						}
					}
					if _, err := s.Load(e.ctx, r); !errors.Is(err, task.ErrNotReady) {
						t.Fatal("拒绝结果被复用", err)
					}
					if e.queryInt("SELECT current_step FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_steps") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='step_completed'") != 0 || e.queryInt("SELECT reserved_calls FROM tm_ai_task_budgets") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 2 {
						t.Fatal("拒绝改变进度/账务或重复预算")
					}
				})
			}
		})
	}
}
