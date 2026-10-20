//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func newConversionEnv(t *testing.T, level sql.IsolationLevel, lease time.Duration) (*reportEnv, task.StageRequest, *finalreport.SelectionStage) {
	return newConversionEnvWithPolicy(t, level, lease, nil)
}

func newConversionEnvWithPolicy(t *testing.T, level sql.IsolationLevel, lease time.Duration, cfg *finalreport.Config) (*reportEnv, task.StageRequest, *finalreport.SelectionStage) {
	t.Helper()
	x := newReportEnvWithStage(t, level, lease, false)
	if cfg != nil {
		var err error
		x.policy, err = finalreport.NewPolicy(*cfg, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
			return finalreport.Uncertain, nil
		}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
		if err != nil {
			t.Fatal(err)
		}
		x.svc, err = service.NewAIReportService(x.repo, x.ledger, x.policy, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	sel, err := finalreport.NewSelectionStage(x.policy, x.v.No, x.v.SubjectID, "示例题材", []sources.Snapshot{x.snap})
	if err != nil {
		t.Fatal(err)
	}
	r := task.StageRequest{UserID: x.v.UserID, TaskID: x.v.ID, Owner: x.v.LeaseOwner, Fence: x.v.Fence, OperationKey: "test_only_selection_operation", InputDigest: sel.InputDigest(), StepNo: 1, Stage: "extract"}
	b, err := service.NewAITaskBudgetService(x.repo)
	if err != nil {
		t.Fatal(err)
	}
	// 以下均为本地示例模型响应与预算上界，不是真实调用次数/用量/成本。
	if _, err := b.Reserve(x.ctx, task.ReserveRequest{UserID: r.UserID, TaskID: r.TaskID, Owner: r.Owner, Fence: r.Fence, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
		t.Fatal(err)
	}
	stage, err := service.NewAITaskStageService(x.repo, map[string]task.StagePolicy{"extract": sel.Policy()})
	if err != nil {
		t.Fatal(err)
	}
	in := finalreport.SelectionResponse{}
	for i, key := range []render.SectionKey{render.SecDefinition, render.SecTimeline, render.SecSegment, render.SecCompanies} {
		in.Selections = append(in.Selections, finalreport.Selection{Section: key, SnapshotID: x.snap.ID, SentenceIDs: []int{i + 1}})
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Capture(x.ctx, r, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Complete(x.ctx, r); err != nil {
		t.Fatal(err)
	}
	return x, r, sel
}

func conversionJudge(t *testing.T, x *reportEnv, r task.StageRequest, d service.AIReportDraft) finalreport.Request {
	t.Helper()
	r.StepNo++
	r.Stage = "judge"
	r.OperationKey = "test_only_converted_judge"
	r.InputDigest = d.InputDigest
	b, err := service.NewAITaskBudgetService(x.repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Reserve(x.ctx, task.ReserveRequest{UserID: r.UserID, TaskID: r.TaskID, Owner: r.Owner, Fence: r.Fence, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
		t.Fatal(err)
	}
	stage, err := service.NewAITaskStageService(x.repo, map[string]task.StagePolicy{"judge": x.policy.StagePolicy()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Capture(x.ctx, r, d.Payload); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Complete(x.ctx, r); err != nil {
		t.Fatal(err)
	}
	return finalreport.Request{TaskNo: x.v.No, Stage: r}
}

func TestIntegrationAIReportConversionLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, reject := range []bool{false, true} {
				name := "success"
				if reject {
					name = "reject"
				}
				t.Run(name, func(t *testing.T) {
					x, r, sel := newConversionEnv(t, level, time.Minute)
					d, err := x.svc.Convert(x.ctx, r, sel)
					if err != nil {
						t.Fatal(err)
					}
					again, err := x.svc.Convert(x.ctx, r, sel)
					if err != nil || d.InputDigest != again.InputDigest || string(d.Payload) != string(again.Payload) {
						t.Fatal("转换重放变化", err)
					}
					if x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || x.calls.Load() != 0 {
						t.Fatal("转换冒充已终检")
					}
					judge := conversionJudge(t, x, r, d)
					if reject {
						x.semantic = finalreport.Uncertain
					}
					got, err := x.svc.Finalize(x.ctx, judge)
					if err != nil {
						t.Fatal(err)
					}
					want := task.Succeeded
					kind := "settle"
					balance := 13
					if reject {
						want = task.Failed
						kind = "release"
						balance = 20
					}
					if got.Status != want || x.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != balance || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind=?", kind) != 1 {
						t.Fatal("转换后结算/释放不一致", got)
					}
					if reject && x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE payload IS NOT NULL") != 0 {
						t.Fatal("拒绝报告保存正文")
					}
					calls := x.calls.Load()
					replay, err := x.svc.Finalize(x.ctx, judge)
					if err != nil || replay != got || calls != x.calls.Load() {
						t.Fatal("重放重复终检或结算", err)
					}
					requirePointAudit(t, x.pointsTestEnv, x.ledger)
				})
			}
		})
	}
}

func TestIntegrationAIReportConversionBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"off", "owner", "fence", "withdrawn", "license", "theme", "cancel", "missing_checkpoint"} {
				t.Run(name, func(t *testing.T) {
					x, r, sel := newConversionEnv(t, level, time.Minute)
					switch name {
					case "off":
						var err error
						x.svc, err = service.NewAIReportService(x.repo, x.ledger, nil, false)
						if err != nil {
							t.Fatal(err)
						}
					case "owner":
						r.UserID++
					case "fence":
						r.Fence++
					case "withdrawn":
						if err := repository.NewAISourceRepo(x.db).SetSourceEnabled(x.ctx, x.source.ID, false); err != nil {
							t.Fatal(err)
						}
					case "license":
						x.mustExec(`UPDATE tm_ai_sources SET license_ref='test-only-changed' WHERE id=?`, x.source.ID)
					case "theme":
						x.mustExec(`UPDATE addon_quant_theme SET name='另一示例题材' WHERE id=1`)
					case "cancel":
						if _, err := x.tasks.Cancel(x.ctx, 1, x.v.ID); err != nil {
							t.Fatal(err)
						}
					case "missing_checkpoint":
						r.StepNo++
					}
					got, err := x.svc.Convert(x.ctx, r, sel)
					if err == nil || len(got.Payload) != 0 || got.InputDigest != "" {
						t.Fatal("边界泄露候选", got, err)
					}
					if x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 0 || x.calls.Load() != 0 {
						t.Fatal("失败转换写了报告/扣费")
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportConversionConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x, r, sel := newConversionEnv(t, level, time.Minute)
			baseline, err := x.svc.Convert(x.ctx, r, sel)
			if err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					got, err := x.svc.Convert(x.ctx, r, sel)
					if err != nil || got.InputDigest != baseline.InputDigest || string(got.Payload) != string(baseline.Payload) {
						t.Error("并发候选不稳定", err)
					}
				}()
			}
			wg.Wait()
			if x.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 1 || x.queryInt("SELECT COUNT(*) FROM tm_ai_task_reservations") != 1 || x.calls.Load() != 0 {
				t.Fatal("只读转换追加调用/响应")
			}
		})
	}
}

func TestIntegrationAIReportConversionRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x, r, sel := newConversionEnv(t, level, time.Second)
			d, err := x.svc.Convert(x.ctx, r, sel)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, x.pointsTestEnv, *x.v.LeaseUntil)
			next, err := x.tasks.Claim(x.ctx, 1, x.v.ID, "test_conversion_recovery", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := x.svc.Convert(x.ctx, r, sel); err == nil || len(got.Payload) != 0 {
				t.Fatal("旧围栏仍能构建")
			}
			r.Owner = next.LeaseOwner
			r.Fence = next.Fence
			recovered, err := x.svc.Convert(x.ctx, r, sel)
			if err != nil || d.InputDigest != recovered.InputDigest || string(d.Payload) != string(recovered.Payload) {
				t.Fatal("接管无法复用已捕获选择", err)
			}
			got, err := x.svc.Finalize(x.ctx, conversionJudge(t, x, r, recovered))
			if err != nil || got.Status != task.Succeeded {
				t.Fatal(got, err)
			}
			if x.queryInt("SELECT COUNT(*) FROM tm_ai_task_step_results") != 2 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 {
				t.Fatal("恢复重复响应/扣费")
			}
		})
	}
}
