package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type AITaskStageService struct {
	store    task.StageStore
	policies map[string]task.StagePolicy
}

func NewAITaskStageService(store task.StageStore, policies map[string]task.StagePolicy) (*AITaskStageService, error) {
	if store == nil || len(policies) < 1 || len(policies) > 4 {
		return nil, points.ErrInvalid
	}
	s := &AITaskStageService{store: store, policies: make(map[string]task.StagePolicy, len(policies))}
	for name, p := range policies {
		if !task.ValidStage(name) || !p.Valid() {
			return nil, points.ErrInvalid
		}
		s.policies[name] = p
	}
	return s, nil
}
func (s *AITaskStageService) prepare(ctx context.Context, r task.StageRequest) (context.Context, context.CancelFunc, task.StagePolicy, error) {
	if s == nil {
		return nil, nil, task.StagePolicy{}, points.ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return nil, nil, task.StagePolicy{}, err
	}
	p, ok := s.policies[r.Stage]
	if !ok {
		return nil, nil, task.StagePolicy{}, task.ErrStagePolicy
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	return ctx, cancel, p, nil
}
func stageTask(ctx context.Context, tx task.StageTx, r task.StageRequest) (task.Task, time.Time, error) {
	v, err := tx.LockTask(ctx, r.UserID, r.TaskID)
	if err != nil {
		return task.Task{}, time.Time{}, err
	}
	if v.ID != r.TaskID || v.UserID != r.UserID || v.CurrentStep < 0 || v.CurrentStep > task.MaxSteps || v.EventSeq < 1 {
		return task.Task{}, time.Time{}, points.ErrInvariant
	}
	now, err := tx.Points().Now(ctx)
	if err != nil {
		return task.Task{}, time.Time{}, err
	}
	if err := ownedLease(v, r.Owner, r.Fence, now); err != nil {
		return task.Task{}, time.Time{}, err
	}
	if !v.Deadline.After(now) {
		return task.Task{}, time.Time{}, task.ErrLease
	}
	if v.CancelRequested {
		return task.Task{}, time.Time{}, task.ErrCancelled
	}
	return v, now, nil
}
func stageReservation(ctx context.Context, tx task.StageTx, v task.Task, r task.StageRequest) (task.Reservation, error) {
	state, err := tx.LockBudget(ctx, r.UserID, r.TaskID)
	if err != nil {
		return task.Reservation{}, err
	}
	rows, err := tx.Reservations(ctx, r.UserID, r.TaskID)
	if err != nil {
		return task.Reservation{}, err
	}
	if err := task.CheckBudget(v, state, rows); err != nil {
		return task.Reservation{}, err
	}
	for _, row := range rows {
		if row.Key == r.OperationKey {
			if row.InputDigest != r.InputDigest || !task.StageUnitsMatch(r.Stage, row.Units) {
				return task.Reservation{}, points.ErrIdempotency
			}
			return row, nil
		}
	}
	return task.Reservation{}, points.ErrNotFound
}

// Capture 只保存隔离响应，不推进阶段，不宣称已校验，也不输出正文。
func (s *AITaskStageService) Capture(ctx context.Context, r task.StageRequest, payload []byte) (task.CaptureReceipt, error) {
	ctx, cancel, _, err := s.prepare(ctx, r)
	if err != nil {
		return task.CaptureReceipt{}, err
	}
	defer cancel()
	// 复制调用方内存，后续摘要/落库始终针对同一份输入；调用方不得并发修改正在传入的切片。
	payload = append([]byte(nil), payload...)
	if err := task.CheckStageJSON(payload); err != nil {
		return task.CaptureReceipt{}, err
	}
	var out task.CaptureReceipt
	err = s.store.WithinStageTx(ctx, func(tx task.StageTx) error {
		v, now, err := stageTask(ctx, tx, r)
		if err != nil {
			return err
		}
		if r.StepNo != v.CurrentStep+1 {
			return task.ErrNotReady
		}
		res, err := stageReservation(ctx, tx, v, r)
		if err != nil {
			return err
		}
		if res.Owner != r.Owner || res.Fence != r.Fence {
			return task.ErrLease
		}
		row := task.StageResult{TaskID: r.TaskID, UserID: r.UserID, Fence: r.Fence, ReservationSeq: res.Seq, StepNo: r.StepNo, Stage: r.Stage, Owner: r.Owner, InputDigest: r.InputDigest, OutputDigest: task.StageDigest(payload), Payload: payload, CreatedAt: now}
		fresh, err := tx.InsertStageResult(ctx, row)
		if err != nil {
			return err
		}
		stored, err := tx.StageResult(ctx, r.UserID, r.TaskID, res.Seq)
		if err != nil {
			return err
		}
		if err := task.CheckStageResult(r, res, stored); err != nil {
			return err
		}
		if stored.OutputDigest != row.OutputDigest {
			return points.ErrIdempotency
		}
		out = stored.Receipt(fresh)
		return nil
	})
	if err != nil {
		return task.CaptureReceipt{}, err
	}
	return out, nil
}

type stageView struct {
	v          task.Task
	result     task.StageResult
	checkpoint task.StageCheckpoint
	complete   bool
}

func readStageView(ctx context.Context, tx task.StageTx, r task.StageRequest, p task.StagePolicy) (stageView, error) {
	v, _, err := stageTask(ctx, tx, r)
	if err != nil {
		return stageView{}, err
	}
	if r.StepNo > v.CurrentStep+1 {
		return stageView{}, task.ErrNotReady
	}
	res, err := stageReservation(ctx, tx, v, r)
	if err != nil {
		return stageView{}, err
	}
	result, err := tx.StageResult(ctx, r.UserID, r.TaskID, res.Seq)
	if err != nil {
		return stageView{}, err
	}
	if err := task.CheckStageResult(r, res, result); err != nil {
		return stageView{}, err
	}
	out := stageView{v: v, result: result}
	if r.StepNo > v.CurrentStep {
		return out, nil
	}
	c, err := tx.StageCheckpoint(ctx, r.UserID, r.TaskID, r.StepNo)
	if err != nil {
		if errors.Is(err, points.ErrNotFound) {
			return stageView{}, points.ErrInvariant
		}
		return stageView{}, err
	}
	if c.TaskID != r.TaskID || c.UserID != r.UserID || c.StepNo != r.StepNo || c.ReservationSeq != res.Seq || c.Stage != r.Stage || c.InputDigest != r.InputDigest || c.OutputDigest != result.OutputDigest || c.Fence < result.Fence || c.Fence > v.Fence || !task.ValidOwner(c.Owner) || c.EventSeq < 1 || c.EventSeq > v.EventSeq || c.CreatedAt.IsZero() {
		return stageView{}, points.ErrInvariant
	}
	if c.PolicyDigest != p.Digest {
		return stageView{}, task.ErrStagePolicy
	}
	e, err := tx.StageEvent(ctx, r.TaskID, c.EventSeq)
	if err != nil {
		if errors.Is(err, points.ErrNotFound) {
			return stageView{}, points.ErrInvariant
		}
		return stageView{}, err
	}
	if e.TaskID != r.TaskID || e.Seq != c.EventSeq || e.Type != "step_completed" || e.Status != task.Running || e.Step != r.StepNo || e.ErrorCode != "" {
		return stageView{}, points.ErrInvariant
	}
	out.checkpoint = c
	out.complete = true
	return out, nil
}

// Complete 的纯校验在事务外；真正推进前重新检查当前租约、取消、内容与策略。
func (s *AITaskStageService) Complete(ctx context.Context, r task.StageRequest) (task.CompletedStage, error) {
	ctx, cancel, p, err := s.prepare(ctx, r)
	if err != nil {
		return task.CompletedStage{}, err
	}
	defer cancel()
	var before stageView
	err = s.store.WithinStageTx(ctx, func(tx task.StageTx) error { var err error; before, err = readStageView(ctx, tx, r, p); return err })
	if err != nil {
		return task.CompletedStage{}, err
	}
	if before.complete {
		return task.CompletedStage{Checkpoint: before.checkpoint}, nil
	}
	if err := p.Check(ctx, task.StageValidationInput{Stage: r.Stage, InputDigest: r.InputDigest, Payload: string(before.result.Payload)}); err != nil {
		if ctx.Err() != nil {
			return task.CompletedStage{}, ctx.Err()
		}
		return task.CompletedStage{}, task.ErrStageRejected
	}
	if err := ctx.Err(); err != nil {
		return task.CompletedStage{}, err
	}
	var out task.CompletedStage
	err = s.store.WithinStageTx(ctx, func(tx task.StageTx) error {
		nowView, err := readStageView(ctx, tx, r, p)
		if err != nil {
			return err
		}
		if nowView.result.OutputDigest != before.result.OutputDigest {
			return points.ErrInvariant
		}
		if nowView.complete {
			out = task.CompletedStage{Checkpoint: nowView.checkpoint}
			return nil
		}
		if nowView.v.EventSeq == math.MaxInt64 {
			return points.ErrOverflow
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		c := task.StageCheckpoint{TaskID: r.TaskID, UserID: r.UserID, Fence: r.Fence, StepNo: r.StepNo, ReservationSeq: nowView.result.ReservationSeq, Stage: r.Stage, Owner: r.Owner, InputDigest: r.InputDigest, OutputDigest: nowView.result.OutputDigest, PolicyDigest: p.Digest, EventSeq: nowView.v.EventSeq + 1, CreatedAt: now}
		if err := tx.InsertStageCheckpoint(ctx, c); err != nil {
			return err
		}
		confirmed, err := readStageView(ctx, tx, r, p)
		if err != nil {
			return err
		}
		if !confirmed.complete || confirmed.checkpoint != c || confirmed.v.CurrentStep != r.StepNo || confirmed.v.EventSeq != c.EventSeq {
			return points.ErrInvariant
		}
		out = task.CompletedStage{Checkpoint: c, NewlyCompleted: true}
		return nil
	})
	if err != nil {
		return task.CompletedStage{}, err
	}
	return out, nil
}

func (s *AITaskStageService) Load(ctx context.Context, r task.StageRequest) (task.LoadedStage, error) {
	ctx, cancel, p, err := s.prepare(ctx, r)
	if err != nil {
		return task.LoadedStage{}, err
	}
	defer cancel()
	var out task.LoadedStage
	err = s.store.WithinStageTx(ctx, func(tx task.StageTx) error {
		view, err := readStageView(ctx, tx, r, p)
		if err != nil {
			return err
		}
		if !view.complete {
			return task.ErrNotReady
		}
		out = task.LoadedStage{Checkpoint: view.checkpoint, Result: view.result}
		return nil
	})
	if err != nil {
		return task.LoadedStage{}, err
	}
	return out, nil
}
