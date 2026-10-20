package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AITaskService 仅可信内部使用；没有公开收费路由、报告成功占位或真实调用旁路。
type AITaskService struct {
	store   task.Store
	ledger  *PointsService
	entropy io.Reader
}

func NewAITaskService(store task.Store, ledger *PointsService) (*AITaskService, error) {
	if store == nil || ledger == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskService{store: store, ledger: ledger, entropy: rand.Reader}, nil
}
func (s *AITaskService) within(ctx context.Context, fn func(context.Context, task.Tx) error) error {
	if s == nil || s.store == nil || s.ledger == nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.store.WithinTaskTx(ctx, func(tx task.Tx) error { return fn(ctx, tx) })
}

func taskPointsKey(kind string, uid int64, no string) string {
	// 内部编号均来自固定操作域与服务端任务编号，避免与客户端幂等键冲突。
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", kind, uid, no)))
	return "ai_task_" + hex.EncodeToString(hash[:])
}

func (s *AITaskService) Create(ctx context.Context, r task.CreateRequest, price points.Price, budget task.Budget) (task.Created, error) {
	return s.create(ctx, r, price, budget, nil)
}
func (s *AITaskService) createAdmitted(ctx context.Context, r task.CreateRequest, plan task.AdmissionPlan) (task.Created, error) {
	if plan.Validate(task.AdmissionScope{UserID: r.UserID, AnalysisType: r.AnalysisType, SubjectType: r.SubjectType, SubjectID: r.SubjectID}) != nil {
		return task.Created{}, points.ErrInvalid
	}
	return s.create(ctx, r, plan.Price, plan.Budget, &plan)
}
func (s *AITaskService) create(ctx context.Context, r task.CreateRequest, price points.Price, budget task.Budget, plan *task.AdmissionPlan) (task.Created, error) {
	var out task.Created
	if err := r.Validate(); err != nil {
		return out, err
	}
	hash, err := points.RequestHash("ai_task_create", r)
	if err != nil {
		return out, err
	}
	err = s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		// 先锁账户，使同一用户的幂等创建、取消与账务同序串行。
		if _, err := tx.Points().LockAccount(ctx, r.UserID); err != nil {
			if errors.Is(err, points.ErrNotFound) {
				return points.ErrInsufficient
			}
			return err
		}
		old, err := tx.FindByKey(ctx, r.UserID, r.Key)
		if err == nil {
			if old.Hash != hash {
				return points.ErrIdempotency
			}
			if plan != nil {
				if err := checkOriginalAdmission(ctx, tx, old); err != nil {
					return err
				}
			}
			out = old.Created()
			return nil
		}
		if !errors.Is(err, points.ErrNotFound) {
			return err
		}
		if err := price.Validate(); err != nil {
			return err
		}
		if r.PriceVersion != price.Version {
			return points.ErrPriceChanged
		}
		if err := budget.Validate(); err != nil {
			return err
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		var random [16]byte
		if _, err := io.ReadFull(s.entropy, random[:]); err != nil {
			return fmt.Errorf("生成任务编号失败: %w", err)
		}
		no := hex.EncodeToString(random[:])
		deadline := now.Add(time.Duration(budget.MaxDurationMS) * time.Millisecond)
		frozen, err := s.ledger.FreezeInTx(ctx, tx.Points(), points.FreezeRequest{UserID: r.UserID, Key: taskPointsKey("freeze", r.UserID, no), BizType: "ai_task", BizID: "task:" + no, PriceVersion: r.PriceVersion, ExpiresAt: deadline}, price)
		if err != nil {
			return err
		}
		v := task.Task{UserID: r.UserID, No: no, Key: r.Key, Hash: hash, AnalysisType: r.AnalysisType, SubjectType: r.SubjectType, SubjectID: r.SubjectID, HoldID: frozen.HoldID, PriceVersion: price.Version, PricePoints: price.Points, Budget: budget, Status: task.Queued, NotBefore: now, Deadline: deadline, EventSeq: 1}
		var admitted task.AdmissionWriteTx
		var digest string
		if plan != nil {
			var ok bool
			admitted, ok = tx.(task.AdmissionWriteTx)
			if !ok {
				return points.ErrInvariant
			}
			_, digest, err = plan.Encode()
			if err != nil {
				return err
			}
			v.ID, err = admitted.InsertAdmittedTask(ctx, v, digest)
		} else {
			v.ID, err = tx.InsertTask(ctx, v)
		}
		if err != nil {
			return err
		}
		if err := tx.InsertEvent(ctx, task.Event{TaskID: v.ID, Seq: 1, Type: "created", Status: task.Queued}); err != nil {
			return err
		}
		if plan != nil {
			record := task.AdmissionRecord{TaskID: v.ID, UserID: v.UserID, TaskNo: v.No, RequestHash: v.Hash, Digest: digest, Plan: plan.Clone(), CreatedAt: now}
			if err := record.Bind(v); err != nil {
				return err
			}
			if err := admitted.InsertAdmission(ctx, record); err != nil {
				return err
			}
		}
		out = v.Created()
		return nil
	})
	if err != nil {
		return task.Created{}, fmt.Errorf("创建 AI 任务失败: %w", err)
	}
	return out, nil
}

func taskEvent(tx task.Tx, ctx context.Context, old, next task.Task, kind string) error {
	if old.EventSeq == math.MaxInt64 {
		return points.ErrOverflow
	}
	next.EventSeq = old.EventSeq + 1
	if err := tx.SetTask(ctx, old, next); err != nil {
		return err
	}
	return tx.InsertEvent(ctx, task.Event{TaskID: old.ID, Seq: next.EventSeq, Type: kind, Status: next.Status, Step: next.CurrentStep, ErrorCode: next.ErrorCode})
}

// finishTask 只处理非成功终态；成功必须在后续报告终检/保存路径同事务结算。
func (s *AITaskService) finishTask(ctx context.Context, tx task.Tx, old task.Task, state int, code string) (task.Task, error) {
	if old.Terminal() {
		return old, nil
	}
	// 失败理由、重试/容量上限都不能把未确认调用变成可退款的已知失败。
	// 明确取消和实际到期仍按原政策终结，不由此宣称远端已停止。
	if state == task.Failed {
		if err := checkTaskCallsKnown(ctx, tx, old); err != nil {
			return task.Task{}, err
		}
	}
	pointState := points.Released
	if state == task.Expired {
		pointState = points.Expired
	}
	if _, err := s.ledger.FinalizeInTx(ctx, tx.Points(), points.FinalizeRequest{UserID: old.UserID, Key: taskPointsKey("terminal", old.UserID, old.No), HoldID: old.HoldID, State: pointState}); err != nil {
		return task.Task{}, err
	}
	next := old
	next.Status = state
	next.ErrorCode = code
	next.LeaseOwner = ""
	next.LeaseUntil = nil
	kind := "failed"
	if state == task.Cancelled {
		kind = "cancelled"
	}
	if state == task.Expired {
		kind = "expired"
	}
	if err := taskEvent(tx, ctx, old, next, kind); err != nil {
		return task.Task{}, err
	}
	next.EventSeq++
	return next, nil
}

func lockOwnedTask(ctx context.Context, tx task.Tx, uid, id int64) (task.Task, error) {
	if uid <= 0 || id <= 0 {
		return task.Task{}, points.ErrInvalid
	}
	if _, err := tx.Points().LockAccount(ctx, uid); err != nil {
		return task.Task{}, err
	}
	return tx.LockTask(ctx, uid, id)
}

// Inspect 只给可信内部编排器读取当前任务；不授予租约，不返回给公开 HTTP。
func (s *AITaskService) Inspect(ctx context.Context, uid, id int64) (task.Task, error) {
	if uid <= 0 || id <= 0 {
		return task.Task{}, points.ErrInvalid
	}
	var out task.Task
	err := s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		var err error
		out, err = tx.LockTask(ctx, uid, id)
		return err
	})
	if err != nil {
		return task.Task{}, err
	}
	return out, nil
}

func (s *AITaskService) Cancel(ctx context.Context, uid, id int64) (task.Task, error) {
	var out task.Task
	err := s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		if v.Terminal() || v.CancelRequested {
			out = v
			return nil
		}
		if v.Status == task.Queued {
			out, err = s.finishTask(ctx, tx, v, task.Cancelled, "cancelled")
			return err
		}
		if v.Status != task.Running {
			return points.ErrInvariant
		}
		next := v
		next.CancelRequested = true
		if err := taskEvent(tx, ctx, v, next, "cancel_requested"); err != nil {
			return err
		}
		next.EventSeq++
		out = next
		return nil
	})
	if err != nil {
		return task.Task{}, err
	}
	return out, nil
}

func (s *AITaskService) Claim(ctx context.Context, uid, id int64, owner string, lease time.Duration) (task.Task, error) {
	var out task.Task
	if !task.ValidOwner(owner) || lease < time.Second || lease > 10*time.Minute || lease%time.Millisecond != 0 {
		return out, points.ErrInvalid
	}
	err := s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if v.Terminal() {
			return points.ErrClosed
		}
		if v.Status != task.Queued && v.Status != task.Running {
			return points.ErrInvariant
		}
		if !v.Deadline.After(now) {
			out, err = s.finishTask(ctx, tx, v, task.Expired, "expired")
			return err
		}
		if v.Status == task.Running && (v.LeaseUntil == nil || v.LeaseUntil.After(now)) {
			return task.ErrNotReady
		}
		if v.Status == task.Queued && v.NotBefore.After(now) {
			return task.ErrNotReady
		}
		if v.CancelRequested {
			out, err = s.finishTask(ctx, tx, v, task.Cancelled, "cancelled")
			return err
		}
		if v.Attempt >= v.Budget.MaxAttempts {
			out, err = s.finishTask(ctx, tx, v, task.Failed, "attempts_exhausted")
			return err
		}
		if v.Fence == math.MaxInt64 {
			return points.ErrOverflow
		}
		next := v
		next.Status = task.Running
		next.Attempt++
		next.Fence++
		next.LeaseOwner = owner
		until := now.Add(lease)
		if until.After(v.Deadline) {
			until = v.Deadline
		}
		next.LeaseUntil = &until
		if err := taskEvent(tx, ctx, v, next, "claimed"); err != nil {
			return err
		}
		next.EventSeq++
		out = next
		return nil
	})
	if err != nil {
		return task.Task{}, err
	}
	return out, nil
}

func ownedLease(v task.Task, owner string, fence int64, now time.Time) error {
	if !task.ValidOwner(owner) || fence <= 0 {
		return points.ErrInvalid
	}
	if v.Status != task.Running || v.LeaseOwner != owner || v.Fence != fence || v.LeaseUntil == nil || !v.LeaseUntil.After(now) {
		return task.ErrLease
	}
	return nil
}

func (s *AITaskService) Heartbeat(ctx context.Context, uid, id int64, owner string, fence int64, lease time.Duration) (task.Task, error) {
	var out task.Task
	if lease < time.Second || lease > 10*time.Minute || lease%time.Millisecond != 0 {
		return out, points.ErrInvalid
	}
	err := s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		// 心跳不访问积分，因此仅锁任务，不反向追加账户锁。
		if uid <= 0 || id <= 0 {
			return points.ErrInvalid
		}
		v, err := tx.LockTask(ctx, uid, id)
		if err != nil {
			return err
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if err := ownedLease(v, owner, fence, now); err != nil {
			return err
		}
		if v.CancelRequested {
			return task.ErrCancelled
		}
		if !v.Deadline.After(now) {
			return task.ErrLease
		}
		until := now.Add(lease)
		if until.After(v.Deadline) {
			until = v.Deadline
		}
		// 不能因同毫秒心跳或缩短参数倒退租约，也不能把无变化的 UPDATE 当作 CAS 失败。
		if !until.After(*v.LeaseUntil) {
			out = v
			return nil
		}
		next := v
		next.LeaseUntil = &until
		if err := tx.SetTask(ctx, v, next); err != nil {
			return err
		}
		out = next
		return nil
	})
	if err != nil {
		return task.Task{}, err
	}
	return out, nil
}

// FailOrRetry 的错误码必须来自白名单，禁止将供应商原文写入任务/进度事件。
func (s *AITaskService) FailOrRetry(ctx context.Context, uid, id int64, owner string, fence int64, code string, backoff time.Duration) (task.Task, error) {
	var out task.Task
	if !task.ValidFailure(code) || backoff < 0 || backoff > time.Hour || backoff%time.Millisecond != 0 {
		return out, points.ErrInvalid
	}
	err := s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if err := ownedLease(v, owner, fence, now); err != nil {
			return err
		}
		if !v.Deadline.After(now) {
			out, err = s.finishTask(ctx, tx, v, task.Expired, "expired")
			return err
		}
		if v.CancelRequested {
			out, err = s.finishTask(ctx, tx, v, task.Cancelled, "cancelled")
			return err
		}
		if backoff > 0 && v.Attempt >= v.Budget.MaxAttempts {
			code = "attempts_exhausted"
		} else if backoff > 0 && !now.Add(backoff).Before(v.Deadline) {
			code = "budget_exhausted"
		}
		if backoff == 0 || v.Attempt >= v.Budget.MaxAttempts || !now.Add(backoff).Before(v.Deadline) {
			out, err = s.finishTask(ctx, tx, v, task.Failed, code)
			return err
		}
		next := v
		next.Status = task.Queued
		next.LeaseOwner = ""
		next.LeaseUntil = nil
		next.NotBefore = now.Add(backoff)
		if err := taskEvent(tx, ctx, v, next, "retried"); err != nil {
			return err
		}
		next.EventSeq++
		out = next
		return nil
	})
	if err != nil {
		return task.Task{}, err
	}
	return out, nil
}

func (s *AITaskService) Expire(ctx context.Context, uid, id int64) (task.Task, error) {
	var out task.Task
	err := s.within(ctx, func(ctx context.Context, tx task.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		if v.Terminal() {
			out = v
			return nil
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if v.Deadline.After(now) {
			return task.ErrNotReady
		}
		out, err = s.finishTask(ctx, tx, v, task.Expired, "expired")
		return err
	})
	if err != nil {
		return task.Task{}, err
	}
	return out, nil
}
