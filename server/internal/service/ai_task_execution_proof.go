package service

import (
	"context"
	"errors"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// checkTaskCallsKnown 在账户/任务锁内核查所有预留的原始隔离响应。
// 这里只证明调用结果已持久确认，不证明内容通过或报告可交付。
func checkTaskCallsKnown(ctx context.Context, tx task.Tx, v task.Task) error {
	reader, ok := tx.(task.ExecutionReadTx)
	if !ok || v.CurrentStep < 0 || v.CurrentStep > task.MaxSteps {
		return points.ErrInvariant
	}
	state, budgetErr := reader.LockBudget(ctx, v.UserID, v.ID)
	if budgetErr != nil && !errors.Is(budgetErr, points.ErrNotFound) {
		return budgetErr
	}
	rows, err := reader.Reservations(ctx, v.UserID, v.ID)
	if err != nil {
		return err
	}
	if budgetErr != nil {
		if v.CurrentStep != 0 || len(rows) != 0 {
			return points.ErrInvariant
		}
		return nil
	}
	if err := task.CheckBudget(v, state, rows); err != nil {
		return err
	}
	if len(rows) < v.CurrentStep {
		return points.ErrInvariant
	}
	for _, row := range rows {
		result, err := reader.StageResult(ctx, v.UserID, v.ID, row.Seq)
		if errors.Is(err, points.ErrNotFound) {
			return task.ErrUnconfirmed
		}
		if err != nil {
			return err
		}
		request := task.StageRequest{UserID: v.UserID, TaskID: v.ID, StepNo: row.Seq, Stage: result.Stage, InputDigest: row.InputDigest}
		if !task.ValidStage(result.Stage) || !task.StageUnitsMatch(result.Stage, row.Units) || task.CheckStageResult(request, row, result) != nil {
			return points.ErrInvariant
		}
	}
	return nil
}
