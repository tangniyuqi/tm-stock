package service

import (
	"bytes"
	"context"
	"errors"
	"math"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AIReportService 只提供内部终检核；没有生产默认策略、公开入口或真实调用装配。
type AIReportService struct {
	store   finalreport.Store
	policy  *finalreport.Policy
	core    *AITaskService
	enabled bool
}

func NewAIReportService(store finalreport.Store, ledger *PointsService, p *finalreport.Policy, enabled bool) (*AIReportService, error) {
	if store == nil || ledger == nil || enabled && !p.Valid() {
		return nil, points.ErrInvalid
	}
	return &AIReportService{store: store, policy: p, core: &AITaskService{ledger: ledger}, enabled: enabled}, nil
}

type reportView struct {
	v   task.Task
	raw task.StageResult
	old *finalreport.Commit
}

// readReportView 在账户/任务锁后核对全部预算与检查点，不把未完成预留或元数据回执当报告。
func (s *AIReportService) readReportView(ctx context.Context, tx finalreport.Tx, r finalreport.Request) (reportView, error) {
	v, err := lockOwnedTask(ctx, tx, r.Stage.UserID, r.Stage.TaskID)
	if err != nil {
		return reportView{}, err
	}
	wantType := 1
	if s.policy != nil && s.policy.AnalysisType() == task.DeepAnalysisType {
		wantType = task.DeepAnalysisType
	}
	if v.No != r.TaskNo || v.AnalysisType != wantType || v.SubjectType != task.Theme {
		return reportView{}, points.ErrInvalid
	}
	// 与报告/账务处于同一当前读事务，不能只依赖规划器较早的事务外校验。
	if err := checkOriginalAdmission(ctx, tx, v); err != nil {
		return reportView{}, err
	}
	if v.CurrentStep != r.Stage.StepNo || v.CurrentStep < 1 || v.CurrentStep > task.MaxSteps {
		return reportView{}, task.ErrNotReady
	}
	old, err := tx.ReportCommit(ctx, v.UserID, v.ID)
	if err != nil && !errors.Is(err, points.ErrNotFound) {
		return reportView{}, err
	}
	hasOld := err == nil
	if v.Terminal() && !hasOld {
		return reportView{}, points.ErrClosed
	}
	if !v.Terminal() && hasOld {
		return reportView{}, points.ErrInvariant
	}
	if !v.Terminal() {
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return reportView{}, err
		}
		if err := ownedLease(v, r.Stage.Owner, r.Stage.Fence, now); err != nil {
			return reportView{}, err
		}
		if !v.Deadline.After(now) {
			return reportView{}, task.ErrLease
		}
	}
	state, err := tx.LockBudget(ctx, v.UserID, v.ID)
	if err != nil {
		return reportView{}, err
	}
	rows, err := tx.Reservations(ctx, v.UserID, v.ID)
	if err != nil {
		return reportView{}, err
	}
	if err := task.CheckBudget(v, state, rows); err != nil {
		return reportView{}, err
	}
	if len(rows) != v.CurrentStep {
		return reportView{}, task.ErrNotReady
	}
	var last int64
	out := reportView{v: v}
	for i, res := range rows {
		c, err := tx.StageCheckpoint(ctx, v.UserID, v.ID, i+1)
		if err != nil {
			return reportView{}, err
		}
		result, err := tx.StageResult(ctx, v.UserID, v.ID, res.Seq)
		if err != nil {
			return reportView{}, err
		}
		request := task.StageRequest{UserID: v.UserID, TaskID: v.ID, StepNo: i + 1, Stage: c.Stage, InputDigest: res.InputDigest}
		if task.CheckStageResult(request, res, result) != nil || c.TaskID != v.ID || c.UserID != v.UserID || c.StepNo != i+1 || c.ReservationSeq != res.Seq || c.InputDigest != res.InputDigest || c.OutputDigest != result.OutputDigest || !finalreport.ValidDigest(c.PolicyDigest) || !task.ValidStage(c.Stage) || !task.StageUnitsMatch(c.Stage, res.Units) || c.Fence < result.Fence || c.Fence > v.Fence || !task.ValidOwner(c.Owner) || c.CreatedAt.IsZero() || c.EventSeq <= last || c.EventSeq > v.EventSeq {
			return reportView{}, points.ErrInvariant
		}
		e, err := tx.StageEvent(ctx, v.ID, c.EventSeq)
		if err != nil {
			return reportView{}, err
		}
		if e != (task.Event{TaskID: v.ID, Seq: c.EventSeq, Type: "step_completed", Status: task.Running, Step: i + 1}) {
			return reportView{}, points.ErrInvariant
		}
		last = c.EventSeq
		if i == len(rows)-1 {
			if c.Stage != "judge" || res.Key != r.Stage.OperationKey || c.InputDigest != r.Stage.InputDigest {
				return reportView{}, points.ErrIdempotency
			}
			if c.PolicyDigest != s.policy.Digest() {
				return reportView{}, task.ErrStagePolicy
			}
			out.raw = result
		}
	}
	h, err := tx.Points().LockHold(ctx, v.UserID, v.HoldID)
	if err != nil {
		return reportView{}, err
	}
	if h.UserID != v.UserID || h.ID != v.HoldID || h.Amount != v.PricePoints || h.PriceVersion != v.PriceVersion || h.BizType != "ai_task" || h.BizID != "task:"+v.No || !h.ExpiresAt.Equal(v.Deadline) {
		return reportView{}, points.ErrInvariant
	}
	if hasOld {
		if err := old.Validate(); err != nil {
			return reportView{}, err
		}
		if old.TaskID != v.ID || old.UserID != v.UserID || old.TaskNo != v.No || old.Status != v.Status || old.StepNo != v.CurrentStep || old.Fence != v.Fence || old.EventSeq != v.EventSeq || old.OperationKey != r.Stage.OperationKey || old.InputDigest != r.Stage.InputDigest || old.PolicyDigest != s.policy.Digest() || old.ContentDigest != out.raw.OutputDigest {
			return reportView{}, points.ErrInvariant
		}
		wantState, kind, code := points.Settled, "settle", ""
		if old.Status == task.Failed {
			wantState, kind, code = points.Released, "release", "guard_rejected"
		}
		if h.State != wantState || h.TerminalTxnID != old.TerminalTxnID || v.ErrorCode != code || old.Status == task.Succeeded && !bytes.Equal(old.Payload, out.raw.Payload) {
			return reportView{}, points.ErrInvariant
		}
		if old.Status == task.Succeeded && h.Captured != h.Amount || old.Status == task.Failed && h.Captured != 0 {
			return reportView{}, points.ErrInvariant
		}
		txn, err := tx.Points().FindTransactionByID(ctx, v.UserID, h.TerminalTxnID)
		if err != nil {
			return reportView{}, err
		}
		key := taskPointsKey("terminal", v.UserID, v.No)
		hash, err := points.RequestHash("finalize", points.FinalizeRequest{UserID: v.UserID, Key: key, HoldID: v.HoldID, State: wantState})
		if err != nil {
			return reportView{}, err
		}
		if txn.ID != h.TerminalTxnID || txn.UserID != v.UserID || txn.Kind != kind || txn.Key != key || txn.Hash != hash || txn.BizType != h.BizType || txn.BizID != h.BizID {
			return reportView{}, points.ErrInvariant
		}
		e, err := tx.StageEvent(ctx, v.ID, old.EventSeq)
		if err != nil {
			return reportView{}, err
		}
		if e != (task.Event{TaskID: v.ID, Seq: old.EventSeq, Type: map[int]string{task.Succeeded: "succeeded", task.Failed: "failed"}[old.Status], Status: old.Status, Step: v.CurrentStep, ErrorCode: code}) {
			return reportView{}, points.ErrInvariant
		}
		if old.Status == task.Succeeded {
			if _, err := s.policy.Decode(string(old.Payload)); err != nil {
				return reportView{}, points.ErrInvariant
			}
		}
		out.old = &old
	} else if h.State != points.Holding {
		return reportView{}, points.ErrInvariant
	}
	if err := checkReportAdmissionScope(ctx, tx, v, s.policy, out.raw.Payload); err != nil {
		return reportView{}, err
	}
	return out, nil
}

func knownEvidenceRejection(err error) bool {
	return errors.Is(err, sources.ErrUnauthorized) || errors.Is(err, sources.ErrSourceUnavailable) || errors.Is(err, sources.ErrUnsafeURL) || errors.Is(err, finalreport.ErrRejected) || errors.Is(err, points.ErrNotFound)
}

// Finalize 的纯校验不持事务；最终重新当前读后，报告/任务/积分三者原子提交。
func (s *AIReportService) Finalize(ctx context.Context, r finalreport.Request) (finalreport.Receipt, error) {
	if s == nil {
		return finalreport.Receipt{}, points.ErrInvalid
	}
	if !s.enabled {
		return finalreport.Receipt{}, finalreport.ErrDisabled
	}
	if r.Stage.Validate() != nil || r.Stage.Stage != "judge" || r.TaskNo == "" {
		return finalreport.Receipt{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var before reportView
	err := s.store.WithinReportTx(ctx, func(tx finalreport.Tx) error { var err error; before, err = s.readReportView(ctx, tx, r); return err })
	if err != nil {
		return finalreport.Receipt{}, err
	}
	if before.old != nil {
		return before.old.Receipt(), nil
	}
	p, shapeErr := s.policy.Decode(string(before.raw.Payload))
	e, sem, reason := finalreport.Uncertain, finalreport.Uncertain, "structure"
	if shapeErr == nil && p.TaskNo == before.v.No && p.ThemeID == before.v.SubjectID && !before.v.CancelRequested {
		var snaps []sources.Snapshot
		var theme string
		err = s.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
			view, err := s.readReportView(ctx, tx, r)
			if err != nil {
				return err
			}
			if view.old != nil {
				before = view
				return nil
			}
			if view.raw.OutputDigest != before.raw.OutputDigest {
				return points.ErrInvariant
			}
			if view.v.CancelRequested {
				return task.ErrCancelled
			}
			snaps, theme, err = s.reportEvidence(ctx, tx, view.v, s.policy.EvidenceIDs(p))
			return err
		})
		if before.old != nil && err == nil {
			return before.old.Receipt(), nil
		}
		if err != nil && !knownEvidenceRejection(err) && !errors.Is(err, task.ErrCancelled) {
			return finalreport.Receipt{}, err
		}
		reason = "evidence"
		if err == nil && s.policy.Bind(p, snaps, theme) == nil {
			e, sem, reason, err = s.policy.Review(ctx, string(before.raw.Payload), p, snaps)
			if err != nil {
				return finalreport.Receipt{}, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return finalreport.Receipt{}, err
	}
	var out finalreport.Receipt
	err = s.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		view, err := s.readReportView(ctx, tx, r)
		if err != nil {
			return err
		}
		if view.old != nil {
			out = view.old.Receipt()
			return nil
		}
		if view.raw.OutputDigest != before.raw.OutputDigest {
			return points.ErrInvariant
		}
		if view.v.CancelRequested {
			next, err := s.core.finishTask(ctx, tx, view.v, task.Cancelled, "cancelled")
			if err != nil {
				return err
			}
			h, err := tx.Points().LockHold(ctx, next.UserID, next.HoldID)
			if err != nil {
				return err
			}
			out = finalreport.Receipt{TaskID: next.ID, Status: next.Status, TerminalTxnID: h.TerminalTxnID}
			return nil
		}
		if reason == "" {
			snaps, theme, err := s.reportEvidence(ctx, tx, view.v, s.policy.EvidenceIDs(p))
			if err != nil && !knownEvidenceRejection(err) {
				return err
			}
			if err != nil || s.policy.Bind(p, snaps, theme) != nil {
				reason = "evidence"
			}
		}
		if view.v.EventSeq == math.MaxInt64 {
			return points.ErrOverflow
		}
		next := view.v
		var txnID int64
		if reason == "" {
			receipt, err := s.core.ledger.FinalizeInTx(ctx, tx.Points(), points.FinalizeRequest{UserID: next.UserID, Key: taskPointsKey("terminal", next.UserID, next.No), HoldID: next.HoldID, State: points.Settled})
			if err != nil {
				return err
			}
			txnID = receipt.TxnID
			next.Status = task.Succeeded
			next.LeaseOwner = ""
			next.LeaseUntil = nil
			if err := taskEvent(tx, ctx, view.v, next, "succeeded"); err != nil {
				return err
			}
			next.EventSeq++
		} else {
			next, err = s.core.finishTask(ctx, tx, view.v, task.Failed, "guard_rejected")
			if err != nil {
				return err
			}
			h, err := tx.Points().LockHold(ctx, next.UserID, next.HoldID)
			if err != nil {
				return err
			}
			txnID = h.TerminalTxnID
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		c := finalreport.Commit{TaskID: next.ID, UserID: next.UserID, TaskNo: next.No, Fence: next.Fence, EventSeq: next.EventSeq, StepNo: next.CurrentStep, Status: next.Status, TerminalTxnID: txnID, OperationKey: r.Stage.OperationKey, InputDigest: r.Stage.InputDigest, ContentDigest: view.raw.OutputDigest, PolicyDigest: s.policy.Digest(), Entailment: e, Semantic: sem, Reason: reason, CreatedAt: now}
		if next.Status == task.Succeeded {
			c.Payload = append([]byte(nil), view.raw.Payload...)
		}
		if err := c.Validate(); err != nil {
			return err
		}
		if err := tx.InsertReportCommit(ctx, c); err != nil {
			return err
		}
		confirmed, err := s.readReportView(ctx, tx, r)
		if err != nil {
			return err
		}
		if confirmed.old == nil {
			return points.ErrInvariant
		}
		out = confirmed.old.Receipt()
		return nil
	})
	if err != nil {
		return finalreport.Receipt{}, err
	}
	return out, nil
}
