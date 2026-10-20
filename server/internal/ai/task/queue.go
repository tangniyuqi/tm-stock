package task

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const MaxMemberQueueTasks = 100

// MemberQueueOptions 只是有限扫描位置，不是永久消费偏移；下一轮须从零复查。
type MemberQueueOptions struct {
	AfterID  int64
	MaxTasks int
}

func (o MemberQueueOptions) Validate() error {
	if o.AfterID < 0 || o.MaxTasks < 1 || o.MaxTasks > MaxMemberQueueTasks {
		return points.ErrInvalid
	}
	return nil
}

type MemberQueueRange struct {
	UpperID int64
	AsOf    time.Time
}

// MemberQueueCandidate 不包含正文、准入摘要、价格或预算；仅为当前读前的提示。
type MemberQueueCandidate struct {
	ID, UserID          int64
	AnalysisType        int
	SubjectType         string
	Status              int
	NotBefore, Deadline time.Time
	LeaseUntil          *time.Time
	HasAdmission        bool
}

type MemberQueueScanner interface {
	MemberQueueRange(context.Context) (MemberQueueRange, error)
	MemberQueuePage(context.Context, int64, int64, int) ([]MemberQueueCandidate, error)
}

type MemberQueueItem struct {
	TaskID  int64  `json:"task_id"`
	Status  int    `json:"status"`
	Outcome string `json:"outcome"`
}

// MemberQueueReceipt 只证明有限访问/观察，不授予会员报告读取或财务结算权限。
type MemberQueueReceipt struct {
	UpperID          int64             `json:"upper_id"`
	AsOf             int64             `json:"as_of"`
	LastID           int64             `json:"last_id"`
	Visited          int               `json:"visited"`
	Skipped          int               `json:"skipped"`
	Deferred         int               `json:"deferred"`
	TerminalObserved int               `json:"terminal_observed"`
	Pending          int               `json:"pending"`
	Complete         bool              `json:"complete"`
	Items            []MemberQueueItem `json:"items"`
}
