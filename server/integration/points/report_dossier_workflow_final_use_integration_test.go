//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 完整规划预检成功后才撤回，迫使原终检事务独立验证当前用途。
type dossierFinalUsePlanner struct {
	base  service.AIL1ReportPlanner
	fault *dossierWorkflowFaultStore
}

func (p dossierFinalUsePlanner) Plan(ctx context.Context, v task.Task) (service.AIL1ReportPlan, error) {
	return p.base.Plan(ctx, v)
}
func (p dossierFinalUsePlanner) Recheck(ctx context.Context, v task.Task, plan service.AIL1ReportPlan) error {
	if err := p.base.Recheck(ctx, v, plan); err != nil {
		return err
	}
	if v.CurrentStep == 5 && p.fault.once.CompareAndSwap(false, true) {
		_, err := p.fault.f.env.db.ExecContext(ctx, `UPDATE tm_ai_dossier_support_uses SET enabled=0`)
		return err
	}
	return nil
}

func TestIntegrationAIReportDossierWorkflowFinalUse(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			f := newDossierWorkflowFixture(t, level, 1, "partial")
			e := f.env
			store := &dossierWorkflowFaultStore{AITaskRepo: e.repo, f: f, mode: "use_before_final"}
			worker := dossierFaultWorker(t, f, store, false)
			if _, err := worker.Run(e.ctx, 1, f.id); !errors.Is(err, dossier.ErrNotFound) || !store.once.Load() {
				t.Fatal("规划预检后撤回未在原终检事务被拒绝", err)
			}
			v, err := e.tasks.Inspect(e.ctx, 1, f.id)
			if err != nil {
				t.Fatal(err)
			}
			if v.Status != task.Running || v.CurrentStep != 6 || len(e.server.Requests()) != 4 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_step_results WHERE step_no=6`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_steps WHERE step_no=6`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 {
				t.Fatal("原终检未独立核用途或擅自交付/结算")
			}
			e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
			if _, err = worker.Run(e.ctx, 1, f.id); err != nil {
				t.Fatal("恢复当前用途后未复用原完成记录", err)
			}
			v, err = e.tasks.Inspect(e.ctx, 1, f.id)
			if err != nil || v.Status != task.Succeeded || v.CurrentStep != 6 || len(e.server.Requests()) != 4 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
				t.Fatal("用途恢复重复调用或结算", err)
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}
