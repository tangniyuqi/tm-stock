package task

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// ExpiryRange 只固定扫描上界和数据库 UTC 截止时点，不承诺跨页数据库快照。
type ExpiryRange struct {
	UpperID int64
	AsOf    time.Time
}

// ExpiryCandidate 不读取响应、提示词、价格或租约；它不是终结授权。
type ExpiryCandidate struct {
	ID, UserID int64
	Status     int
	Deadline   time.Time
}

type ExpiryScanner interface {
	ExpiryRange(context.Context) (ExpiryRange, error)
	ExpiryPage(context.Context, int64, int64, int) ([]ExpiryCandidate, error)
}

type Expirer interface {
	Expire(context.Context, int64, int64) (Task, error)
}

type ExpiryOptions struct {
	AfterID  int64
	PageSize int
	MaxTasks int
}

func (o ExpiryOptions) Validate() error {
	if o.AfterID < 0 || o.PageSize < 1 || o.PageSize > 1000 || o.MaxTasks < 1 || o.MaxTasks > 100000 {
		return points.ErrInvalid
	}
	return nil
}

// ExpiredObserved 是成功复核到过期终态的数量，包含竞争者先完成，不能当成本进程写入次数。
// LastID 只越过已确认的任务；未知提交不推进该项。Complete 仅表示本轮可见上界已读完。
type ExpirySweep struct {
	UpperID         int64 `json:"upper_id"`
	AsOf            int64 `json:"as_of"`
	LastID          int64 `json:"last_id"`
	Visited         int   `json:"visited"`
	ExpiredObserved int   `json:"expired_observed"`
	Skipped         int   `json:"skipped"`
	Complete        bool  `json:"complete"`
}
