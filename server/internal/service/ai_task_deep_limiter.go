package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const deepCreateLock = "tmstock:deep:create"

// AITaskDeepLimiter 在创建冻结前原子检查深度并发；返回的释放函数必须覆盖整个创建事务。
type AITaskDeepLimiter interface {
	Acquire(context.Context, int64) (func(), error)
}

// SQLDeepLimiter 用 MySQL 命名锁串行化“计数+创建”，只统计排队/执行的深度任务。
type SQLDeepLimiter struct {
	db                 *sql.DB
	maxPerUser, maxAll int
}

func NewSQLDeepLimiter(db *sql.DB) (*SQLDeepLimiter, error) {
	if db == nil {
		return nil, points.ErrInvalid
	}
	return &SQLDeepLimiter{db: db, maxPerUser: task.DeepMaxPerUser, maxAll: task.DeepMaxGlobal}, nil
}

func (l *SQLDeepLimiter) Acquire(ctx context.Context, uid int64) (func(), error) {
	if l == nil || l.db == nil || uid <= 0 || l.maxPerUser < 1 || l.maxAll < l.maxPerUser {
		return nil, points.ErrInvalid
	}
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 3)", deepCreateLock).Scan(&got); err != nil || !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return nil, task.ErrAdmissionUnavailable
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", deepCreateLock)
		_ = conn.Close()
	}
	var total, mine int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM tm_ai_tasks WHERE analysis_type=? AND status IN (?,?)", task.DeepAnalysisType, task.Queued, task.Running).Scan(&total); err != nil {
		release()
		return nil, err
	}
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM tm_ai_tasks WHERE analysis_type=? AND user_id=? AND status IN (?,?)", task.DeepAnalysisType, uid, task.Queued, task.Running).Scan(&mine); err != nil {
		release()
		return nil, err
	}
	if mine >= l.maxPerUser || total >= l.maxAll {
		release()
		return nil, task.ErrAdmissionUnavailable
	}
	return release, nil
}
