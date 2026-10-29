package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// AIReportCallAuditService 不授权模型或报告发布；仅在原预算协议内持久存证。
type AIReportCallAuditService struct{ store finalreport.CallAuditStore }

func NewAIReportCallAuditService(store finalreport.CallAuditStore) (*AIReportCallAuditService, error) {
	if store == nil {
		return nil, points.ErrInvalid
	}
	return &AIReportCallAuditService{store}, nil
}

func (s *AIReportCallAuditService) Start(parent context.Context, r task.StageRequest, req provider.Request) error {
	if s == nil || parent == nil || r.Validate() != nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	return s.store.WithinCallAuditTx(ctx, func(tx finalreport.CallAuditTx) error {
		v, err := lockOwnedTask(ctx, tx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		_, err = tx.ReadCallAudit(ctx, r.UserID, r.TaskID, r.StepNo, "start")
		if err == nil {
			return task.ErrUnconfirmed
		}
		if !errors.Is(err, points.ErrNotFound) {
			return err
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if err := ownedLease(v, r.Owner, r.Fence, now); err != nil {
			return err
		}
		if v.CancelRequested || !v.Deadline.After(now) || v.CurrentStep != r.StepNo-1 {
			return task.ErrLease
		}
		a, found, err := reportAdmission(ctx, tx, v)
		if err != nil {
			return err
		}
		if !found {
			return task.ErrAdmissionUnavailable
		}
		state, err := tx.LockBudget(ctx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		rows, err := tx.Reservations(ctx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		if err := task.CheckBudget(v, state, rows); err != nil {
			return err
		}
		if len(rows) != r.StepNo {
			return points.ErrIdempotency
		}
		row := rows[r.StepNo-1]
		if row.Owner != r.Owner || row.Fence != r.Fence || row.Key != r.OperationKey || row.InputDigest != r.InputDigest || row.Units.Calls != 1 || row.Units.Pages != 0 || int64(req.MaxTokens) > row.Units.Tokens {
			return points.ErrIdempotency
		}
		if _, err := tx.StageResult(ctx, r.UserID, r.TaskID, r.StepNo); !errors.Is(err, points.ErrNotFound) {
			if err != nil {
				return err
			}
			return task.ErrUnconfirmed
		}
		digest, err := finalreport.ReportRequestDigest(r, v.No, a.Plan.ModelName, req)
		if err != nil {
			return err
		}
		out := finalreport.CallAudit{Request: r, Phase: "start", TaskNo: v.No, AdmissionDigest: a.Digest, ProfileDigest: a.Plan.ProfileDigest, RequestDigest: digest, Outcome: "started", CreatedAt: now}
		if err := out.Validate(); err != nil {
			return err
		}
		return tx.InsertCallAudit(ctx, out)
	})
}

func (s *AIReportCallAuditService) Finish(parent context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	if s == nil || parent == nil || r.Validate() != nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	digest, err := finalreport.ReportResultDigest(out, callErr != nil)
	if err != nil {
		return err
	}
	return s.store.WithinCallAuditTx(ctx, func(tx finalreport.CallAuditTx) error {
		v, err := lockOwnedTask(ctx, tx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		start, err := tx.ReadCallAudit(ctx, r.UserID, r.TaskID, r.StepNo, "start")
		if err != nil {
			return err
		}
		if start.Validate() != nil || start.Request != r || start.TaskNo != v.No {
			return points.ErrIdempotency
		}
		result := start
		result.Phase, result.ResultDigest, result.Outcome = "finish", digest, "returned"
		if callErr != nil {
			result.Outcome = "errored"
		}
		old, err := tx.ReadCallAudit(ctx, r.UserID, r.TaskID, r.StepNo, "finish")
		if err == nil {
			result.CreatedAt = old.CreatedAt
			if old.Validate() != nil || old.CreatedAt.Before(start.CreatedAt) || result != old {
				return points.ErrIdempotency
			}
			return nil
		}
		if !errors.Is(err, points.ErrNotFound) {
			return err
		}
		result.CreatedAt, err = tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if result.CreatedAt.Before(start.CreatedAt) || result.Validate() != nil {
			return points.ErrInvariant
		}
		// 不要求旧租约仍存活或来源仍开放：原调用返回后必须先留证，不据此推进步骤或账务。
		return tx.InsertCallAudit(ctx, result)
	})
}
