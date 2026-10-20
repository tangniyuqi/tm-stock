package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type AITaskBudgetService struct{ store task.BudgetStore }

func NewAITaskBudgetService(store task.BudgetStore) (*AITaskBudgetService, error) {
	if store == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskBudgetService{store}, nil
}

// Reserve 不执行网络、不碰积分。预留未知结果永不退款；后续调用层须同时强制上界和有效租约。
func (s *AITaskBudgetService) Reserve(ctx context.Context, r task.ReserveRequest) (task.ReservationReceipt, error) {
	return s.reserve(ctx, r, false, 0)
}

// ReserveSingleOperation 限定单阶段执行任务只有这一项预留，拒绝混入既有抽取/采集任务。
// 检查与插预留在同一当前锁事务内，不靠事务外的“先查再写”。
func (s *AITaskBudgetService) ReserveSingleOperation(ctx context.Context, r task.ReserveRequest) (task.ReservationReceipt, error) {
	return s.reserve(ctx, r, true, 0)
}

// ReserveStep 在同一预算事务内绑定步骤序号；新调用只能紧接完整前缀，同一步不能换键。
func (s *AITaskBudgetService) ReserveStep(ctx context.Context, r task.ReserveRequest, step int) (task.ReservationReceipt, error) {
	if step < 1 || step > task.MaxSteps {
		return task.ReservationReceipt{}, points.ErrInvalid
	}
	return s.reserve(ctx, r, false, step)
}

func (s *AITaskBudgetService) reserve(ctx context.Context, r task.ReserveRequest, single bool, step int) (task.ReservationReceipt, error) {
	if s == nil {
		return task.ReservationReceipt{}, points.ErrInvalid
	}
	if err := r.Validate(); err != nil {
		return task.ReservationReceipt{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var out task.ReservationReceipt
	err := s.store.WithinBudgetTx(ctx, func(tx task.BudgetTx) error {
		// 只锁任务→预算→预留，不反向取得账户/冻结单锁。
		v, err := tx.LockTask(ctx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		if v.ID != r.TaskID || v.UserID != r.UserID {
			return points.ErrInvariant
		}
		now, err := tx.Points().Now(ctx)
		if err != nil {
			return err
		}
		if err := ownedLease(v, r.Owner, r.Fence, now); err != nil {
			return err
		}
		if !v.Deadline.After(now) {
			return task.ErrLease
		}
		if v.CancelRequested {
			return task.ErrCancelled
		}
		if v.Budget.Validate() != nil {
			return points.ErrInvariant
		}
		initialized, err := tx.InitBudget(ctx, task.BudgetState{TaskID: v.ID, UserID: v.UserID, Limits: v.Budget.Limits()})
		if err != nil {
			return err
		}
		state, err := tx.LockBudget(ctx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		var rows []task.Reservation
		// 刚插入的预算由 FK 保证之前没有明细；不锁空索引区间，避免 RR 下不同新任务互相阻塞插入。
		// 已存在的计数器即使为零也必须当前读，不能据此跳过历史一致性核对。
		if !initialized {
			rows, err = tx.Reservations(ctx, r.UserID, r.TaskID)
			if err != nil {
				return err
			}
		}
		if err := task.CheckBudget(v, state, rows); err != nil {
			return err
		}
		if single && (len(rows) > 1 || len(rows) == 1 && rows[0].Key != r.Key || v.CurrentStep > 0 && len(rows) != 1) {
			return points.ErrIdempotency
		}
		for _, row := range rows {
			if row.Key == r.Key {
				if row.InputDigest != r.InputDigest || row.Units != r.Units || step > 0 && row.Seq != step {
					return points.ErrIdempotency
				}
				out = task.ReservationReceipt{Reservation: row}
				return nil
			}
		}
		if step > 0 && (len(rows) != step-1 || v.CurrentStep != step-1) {
			return points.ErrIdempotency
		}
		used, err := state.Reserved.Add(r.Units)
		if err != nil {
			return err
		}
		if state.Count >= task.MaxReservations || !used.Fits(state.Limits) {
			return task.ErrBudget
		}
		row := task.Reservation{TaskID: v.ID, UserID: v.UserID, Fence: v.Fence, Seq: state.Count + 1, Owner: r.Owner, Key: r.Key, InputDigest: r.InputDigest, Units: r.Units, CreatedAt: now}
		if err := tx.InsertReservation(ctx, row); err != nil {
			return err
		}
		expected := state
		expected.Reserved = used
		expected.Count++
		confirmed, err := tx.LockBudget(ctx, r.UserID, r.TaskID)
		if err != nil {
			return err
		}
		if confirmed != expected {
			return points.ErrInvariant
		}
		out = task.ReservationReceipt{Reservation: row, NewlyReserved: true}
		return nil
	})
	if err != nil {
		return task.ReservationReceipt{}, err
	}
	return out, nil
}
