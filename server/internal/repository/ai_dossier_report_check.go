package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 只由显式新编排的启动核验调用；全部探针在原校验器事务中回滚。
func (r *AITaskRepo) VerifyDossierReportSchema(ctx context.Context) error {
	if r == nil {
		return points.ErrInvalid
	}
	return NewAIReportCallAuditSchemaVerifier(r.db).VerifyWithDossier(ctx)
}

func (r *AITaskRepo) VerifyDossierComparisonReportSchema(ctx context.Context) error {
	if r == nil {
		return points.ErrInvalid
	}
	return NewAIReportCallAuditSchemaVerifier(r.db).VerifyWithComparison(ctx)
}

func probeDossierReportCallAudit(ctx context.Context, tx *sql.Tx) error {
	for _, enabled := range []bool{false, true} {
		profile := ""
		if enabled {
			profile = task.DossierReportProfile
		}
		if err := probeReportProfileCallAudit(ctx, tx, profile, 5, enabled); err != nil {
			return err
		}
	}
	return nil
}

func probeComparisonReportCallAudit(ctx context.Context, tx *sql.Tx) error {
	for _, test := range []struct {
		profile string
		step    int
		allow   bool
	}{{"", 6, false}, {task.DossierReportProfile, 6, false}, {task.DossierComparisonReportProfile, 5, true}, {task.DossierComparisonReportProfile, 6, true}} {
		if err := probeReportProfileCallAudit(ctx, tx, test.profile, test.step, test.allow); err != nil {
			return err
		}
	}
	return nil
}

func probeReportProfileCallAudit(ctx context.Context, tx *sql.Tx, profile string, lastStep int, enabled bool) error {
	// 负例也给足预算，证明真正拒绝的是编排权限而非较早的容量检查。
	return probeAdmissionTaskWithReportBudget(ctx, tx, profile, int64(lastStep+1), func(v task.Task, admission task.AdmissionRecord) error {
		a := &aiTaskTx{tx}
		now, err := a.Points().Now(ctx)
		if err != nil {
			return err
		}
		next, until := v, now.Add(time.Minute)
		next.Status, next.Attempt, next.Fence, next.LeaseOwner, next.LeaseUntil, next.EventSeq = task.Running, 1, 1, v.Key, &until, v.EventSeq+1
		if err := a.SetTask(ctx, v, next); err != nil {
			return err
		}
		if err := a.InsertEvent(ctx, task.Event{TaskID: v.ID, Seq: next.EventSeq, Type: "claimed", Status: task.Running}); err != nil {
			return err
		}
		v = next
		if _, err := a.InitBudget(ctx, task.BudgetState{TaskID: v.ID, UserID: v.UserID, Limits: v.Budget.Limits()}); err != nil {
			return err
		}
		for step := 1; step <= lastStep; step++ {
			now, err = a.Points().Now(ctx)
			if err != nil {
				return err
			}
			res := task.Reservation{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Seq: step, Owner: v.LeaseOwner, Key: fmt.Sprintf("dossier_probe_operation_%d", step), InputDigest: strings.Repeat("a", 64), Units: task.Units{Calls: 1, Tokens: 10}, CreatedAt: now}
			if err := a.InsertReservation(ctx, res); err != nil {
				return err
			}
			if step == lastStep {
				start := finalreport.CallAudit{Request: task.StageRequest{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Owner: v.LeaseOwner, StepNo: lastStep, Stage: "judge", OperationKey: res.Key, InputDigest: res.InputDigest}, Phase: "start", TaskNo: v.No, AdmissionDigest: admission.Digest, ProfileDigest: admission.Plan.ProfileDigest, RequestDigest: strings.Repeat("c", 64), Outcome: "started", CreatedAt: now}
				err := a.insertCallAudit(ctx, start)
				if !enabled && !pointsProbeError(err, 1644) {
					return fmt.Errorf("旧编排新增步位未按原准入拒绝: %w", errOrMissing(err))
				}
				if enabled && err != nil {
					return fmt.Errorf("显式新编排新增步位正例失败: %w", err)
				}
				return nil
			}
			stage := "judge"
			if step == 1 {
				stage = "agent_call"
			} else if step == 2 {
				stage = "extract"
			}
			body := []byte(`{}`)
			result := task.StageResult{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, ReservationSeq: step, StepNo: step, Stage: stage, Owner: v.LeaseOwner, InputDigest: res.InputDigest, OutputDigest: task.StageDigest(body), Payload: body, CreatedAt: now}
			if err := a.insertStageResult(ctx, result); err != nil {
				return err
			}
			cp := task.StageCheckpoint{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, StepNo: step, ReservationSeq: step, Stage: stage, Owner: v.LeaseOwner, InputDigest: res.InputDigest, OutputDigest: result.OutputDigest, PolicyDigest: strings.Repeat("b", 64), EventSeq: v.EventSeq + 1, CreatedAt: now}
			if err := a.insertStageCheckpoint(ctx, cp); err != nil {
				return err
			}
			v, err = a.LockTask(ctx, v.UserID, v.ID)
			if err != nil {
				return err
			}
		}
		return points.ErrInvariant
	})
}
