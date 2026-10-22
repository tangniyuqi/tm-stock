package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AITaskAdmissionControl 必须查证真实定价/模型/质量/授权来源/远端硬预算，不能调用模型。
// Ready 用于启动检查；Plan/Recheck 均在数据库事务外。生产没有默认通过实现。
// AITaskDeepAdmission 是可选的深度准入证明；启用深度时控制面必须显式实现并通过启动检查。
type AITaskDeepAdmission interface {
	ReadyDeep(context.Context) error
}

type AITaskAdmissionControl interface {
	Ready(context.Context) error
	Plan(context.Context, task.AdmissionScope) (task.AdmissionPlan, error)
	Recheck(context.Context, task.AdmissionPlan) error
}

type AITaskCreateService struct {
	tasks   *AITaskService
	control AITaskAdmissionControl
	enabled bool
	deep    AITaskDeepLimiter
}

func NewAITaskCreateService(tasks *AITaskService, control AITaskAdmissionControl, enabled bool) (*AITaskCreateService, error) {
	if tasks == nil || tasks.store == nil || tasks.ledger == nil || enabled && control == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskCreateService{tasks: tasks, control: control, enabled: enabled}, nil
}

// SetDeepLimiter 由启动装配注入；未注入时深度创建必须失败关闭，不能默认放行。
func (s *AITaskCreateService) SetDeepLimiter(l AITaskDeepLimiter) {
	if s != nil {
		s.deep = l
	}
}

func (s *AITaskCreateService) acquireDeep(ctx context.Context, scope task.AdmissionScope) (func(), error) {
	if scope.AnalysisType != task.DeepAnalysisType {
		return func() {}, nil
	}
	if s.deep == nil {
		return nil, task.ErrAdmissionUnavailable
	}
	return s.deep.Acquire(ctx, scope.UserID)
}

func admissionScope(uid int64, kind, subject string, id int64) (task.AdmissionScope, error) {
	var level int
	switch kind {
	case "light":
		level = 1
	case "deep":
		level = 2
	default:
		return task.AdmissionScope{}, points.ErrInvalid
	}
	s := task.AdmissionScope{UserID: uid, AnalysisType: level, SubjectType: subject, SubjectID: id}
	return s, s.Validate()
}

func (s *AITaskCreateService) plan(ctx context.Context, scope task.AdmissionScope) (task.AdmissionPlan, error) {
	if (scope.AnalysisType != 1 && scope.AnalysisType != 2) || scope.SubjectType != task.Theme {
		return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
	}
	p, err := s.control.Plan(ctx, scope)
	if err != nil {
		return task.AdmissionPlan{}, err
	}
	p = p.Clone()
	if p.Validate(scope) != nil {
		return task.AdmissionPlan{}, points.ErrInvariant
	}
	if err := ctx.Err(); err != nil {
		return task.AdmissionPlan{}, err
	}
	return p, nil
}

func (s *AITaskCreateService) recheck(ctx context.Context, p task.AdmissionPlan) error {
	if err := s.control.Recheck(ctx, p.Clone()); err != nil {
		return err
	}
	return ctx.Err()
}

// eligiblePoints 不创建账户、不清扫、不写流水，按数据库时点复核实际可消费批次。
func eligiblePoints(ctx context.Context, tx points.Tx, uid int64) (int64, time.Time, error) {
	a, err := tx.LockAccount(ctx, uid)
	if err != nil && !errors.Is(err, points.ErrNotFound) {
		return 0, time.Time{}, err
	}
	now, clockErr := tx.Now(ctx)
	if clockErr != nil {
		return 0, time.Time{}, clockErr
	}
	if now.IsZero() || now.UnixMilli() <= 0 {
		return 0, time.Time{}, points.ErrInvariant
	}
	if errors.Is(err, points.ErrNotFound) {
		return 0, now, nil
	}
	if a.UserID != uid || a.Avail < 0 || a.Frozen < 0 || a.Debt < 0 {
		return 0, time.Time{}, points.ErrInvariant
	}
	lots, err := tx.LockUsableLots(ctx, uid, now)
	if err != nil {
		return 0, time.Time{}, err
	}
	seen := make(map[int64]bool, len(lots))
	var available int64
	for _, lot := range lots {
		if lot.ID <= 0 || seen[lot.ID] || lot.UserID != uid || lot.Frozen < 0 || lot.Remain <= lot.Frozen || lot.Total < lot.Remain || lot.EffectiveAt.IsZero() || lot.EffectiveAt.After(now) || lot.ExpiresAt != nil && !lot.ExpiresAt.After(now) {
			return 0, time.Time{}, points.ErrInvariant
		}
		seen[lot.ID] = true
		available, err = points.AddNonnegative(available, lot.Remain-lot.Frozen)
		if err != nil {
			return 0, time.Time{}, err
		}
	}
	if available > a.Avail {
		return 0, time.Time{}, points.ErrInvariant
	}
	return available, now, nil
}

func (s *AITaskCreateService) Quote(parent context.Context, uid int64, input dto.AITaskQuoteInput) (dto.AITaskQuote, error) {
	if s == nil {
		return dto.AITaskQuote{}, points.ErrInvalid
	}
	if !s.enabled {
		return dto.AITaskQuote{}, task.ErrCreationDisabled
	}
	scope, err := admissionScope(uid, input.AnalysisType, input.SubjectType, input.SubjectID)
	if err != nil {
		return dto.AITaskQuote{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	p, err := s.plan(ctx, scope)
	if err != nil {
		return dto.AITaskQuote{}, err
	}
	if err := s.recheck(ctx, p); err != nil {
		return dto.AITaskQuote{}, err
	}
	var available int64
	var at time.Time
	err = s.tasks.within(ctx, func(ctx context.Context, tx task.Tx) error {
		var err error
		available, at, err = eligiblePoints(ctx, tx.Points(), uid)
		return err
	})
	if err != nil {
		return dto.AITaskQuote{}, err
	}
	if err := s.recheck(ctx, p); err != nil {
		return dto.AITaskQuote{}, err
	}
	out := dto.AITaskQuote{AnalysisType: input.AnalysisType, SubjectType: scope.SubjectType, SubjectID: scope.SubjectID, SubjectName: p.SubjectName, PricePoints: p.Price.Points, PriceVersion: p.Price.Version, AvailablePoints: available, QuotedAt: at.UnixMilli(), HoldExpireSec: (p.Budget.MaxDurationMS + 999) / 1000, MaterialScope: make([]dto.AITaskMaterialScope, 0, len(p.Materials))}
	for _, m := range p.Materials {
		if m.CollectedAt.After(at) {
			return dto.AITaskQuote{}, points.ErrInvariant
		}
		out.MaterialScope = append(out.MaterialScope, dto.AITaskMaterialScope{SourceType: m.Category, SourceURL: m.URL, CollectedAt: m.CollectedAt.UnixMilli()})
	}
	return out, nil
}

func creationProjection(v task.Created, uid int64, version string) (dto.AITaskCreation, error) {
	if v.TaskID <= 0 || v.UserID != uid || v.HoldID <= 0 || (points.Price{Version: v.PriceVersion, Points: v.PricePoints}).Validate() != nil || version != "" && v.PriceVersion != version {
		return dto.AITaskCreation{}, points.ErrInvariant
	}
	return dto.AITaskCreation{TaskID: v.TaskID, PricePoints: v.PricePoints, PriceVersion: v.PriceVersion}, nil
}

// lookup 只复核原创建证据，不能为缺失请求补建任务或冻结。
func (s *AITaskCreateService) lookup(ctx context.Context, uid int64, key string, expected *task.CreateRequest) (dto.AITaskCreation, bool, error) {
	var out dto.AITaskCreation
	var found bool
	err := s.tasks.within(ctx, func(ctx context.Context, tx task.Tx) error {
		if _, err := tx.Points().LockAccount(ctx, uid); errors.Is(err, points.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		v, err := tx.FindByKey(ctx, uid, key)
		if errors.Is(err, points.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := taskProgress(v, uid); err != nil {
			return err
		}
		r := task.CreateRequest{UserID: uid, Key: key, AnalysisType: v.AnalysisType, SubjectType: v.SubjectType, SubjectID: v.SubjectID, PriceVersion: v.PriceVersion}
		if r.Validate() != nil || v.Key != key || len(v.No) != 32 || v.Budget.Validate() != nil {
			return points.ErrInvariant
		}
		for _, c := range v.No {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return points.ErrInvariant
			}
		}
		hash, err := points.RequestHash("ai_task_create", r)
		if err != nil {
			return err
		}
		if hash != v.Hash {
			return points.ErrInvariant
		}
		if expected != nil && *expected != r {
			return points.ErrIdempotency
		}
		if err := checkOriginalAdmission(ctx, tx, v); err != nil {
			return err
		}
		h, err := tx.Points().LockHold(ctx, uid, v.HoldID)
		if errors.Is(err, points.ErrNotFound) {
			return points.ErrInvariant
		}
		if err != nil {
			return err
		}
		state := points.Holding
		switch v.Status {
		case task.Succeeded:
			state = points.Settled
		case task.Failed, task.Cancelled:
			state = points.Released
		case task.Expired:
			state = points.Expired
		}
		captured := int64(0)
		if state == points.Settled {
			captured = v.PricePoints
		}
		if h.ID != v.HoldID || h.UserID != uid || h.BizType != "ai_task" || h.BizID != "task:"+v.No || h.PriceVersion != v.PriceVersion || h.Amount != v.PricePoints || h.State != state || h.Captured != captured || !h.ExpiresAt.Equal(v.Deadline) || h.FreezeTxnID <= 0 || (state == points.Holding && h.TerminalTxnID != 0) || (state != points.Holding && h.TerminalTxnID <= 0) {
			return points.ErrInvariant
		}
		f := points.FreezeRequest{UserID: uid, Key: taskPointsKey("freeze", uid, v.No), BizType: h.BizType, BizID: h.BizID, PriceVersion: v.PriceVersion, ExpiresAt: v.Deadline.UTC()}
		freezeHash, err := points.RequestHash("freeze", f)
		if err != nil {
			return err
		}
		txn, err := tx.Points().FindTransactionByID(ctx, uid, h.FreezeTxnID)
		if errors.Is(err, points.ErrNotFound) {
			return points.ErrInvariant
		}
		if err != nil {
			return err
		}
		if txn.ID != h.FreezeTxnID || txn.UserID != uid || txn.Kind != "freeze" || txn.BizType != h.BizType || txn.BizID != h.BizID || txn.Key != f.Key || txn.Hash != freezeHash {
			return points.ErrInvariant
		}
		out, err = creationProjection(v.Created(), uid, v.PriceVersion)
		found = err == nil
		return err
	})
	if err != nil {
		return dto.AITaskCreation{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return dto.AITaskCreation{}, false, err
	}
	return out, found, nil
}

func (s *AITaskCreateService) Receipt(parent context.Context, uid int64, key string) (dto.AITaskCreation, error) {
	if s == nil || uid <= 0 || !points.ValidKey(key) {
		return dto.AITaskCreation{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	out, found, err := s.lookup(ctx, uid, key, nil)
	if err != nil {
		return dto.AITaskCreation{}, err
	}
	if !found {
		return dto.AITaskCreation{}, points.ErrNotFound
	}
	return out, nil
}

func (s *AITaskCreateService) Create(parent context.Context, uid int64, key string, input dto.AITaskCreateInput) (dto.AITaskCreation, error) {
	return s.createQuoted(parent, uid, key, input, 0)
}

// CreateQuoted 供会话意图复用原创建核；额外核对已确认金额，不改变原任务请求哈希。
// 不能在冻结之后才比较报价，否则会先按错误金额扣住积分。
func (s *AITaskCreateService) CreateQuoted(parent context.Context, uid int64, key string, input dto.AITaskCreateInput, expectedPoints int64) (dto.AITaskCreation, error) {
	if expectedPoints <= 0 {
		return dto.AITaskCreation{}, points.ErrInvalid
	}
	return s.createQuoted(parent, uid, key, input, expectedPoints)
}

func (s *AITaskCreateService) createQuoted(parent context.Context, uid int64, key string, input dto.AITaskCreateInput, expectedPoints int64) (dto.AITaskCreation, error) {
	if s == nil {
		return dto.AITaskCreation{}, points.ErrInvalid
	}
	if !s.enabled {
		return dto.AITaskCreation{}, task.ErrCreationDisabled
	}
	scope, err := admissionScope(uid, input.AnalysisType, input.SubjectType, input.SubjectID)
	if err != nil {
		return dto.AITaskCreation{}, err
	}
	r := task.CreateRequest{UserID: uid, Key: key, AnalysisType: scope.AnalysisType, SubjectType: scope.SubjectType, SubjectID: scope.SubjectID, PriceVersion: input.PriceVersion}
	if r.Validate() != nil {
		return dto.AITaskCreation{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if old, found, err := s.lookup(ctx, uid, key, &r); found || err != nil {
		if err == nil && found && expectedPoints > 0 && old.PricePoints != expectedPoints {
			return dto.AITaskCreation{}, points.ErrIdempotency
		}
		return old, err
	}
	p, err := s.plan(ctx, scope)
	if err != nil {
		return dto.AITaskCreation{}, err
	}
	if err := s.recheck(ctx, p); err != nil {
		return dto.AITaskCreation{}, err
	}
	if p.Price.Version != input.PriceVersion || expectedPoints > 0 && p.Price.Points != expectedPoints {
		return dto.AITaskCreation{}, points.ErrPriceChanged
	}
	release, err := s.acquireDeep(ctx, scope)
	if err != nil {
		return dto.AITaskCreation{}, err
	}
	defer release()
	created, err := s.tasks.createAdmitted(ctx, r, p)
	if err != nil {
		return dto.AITaskCreation{}, fmt.Errorf("会员任务创建未确认: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return dto.AITaskCreation{}, err
	}
	return creationProjection(created, uid, input.PriceVersion)
}
