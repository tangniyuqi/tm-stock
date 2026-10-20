package candidate

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// ReviewCatalog 是同一审核事务内锁定读取的主数据，不接受客户端回传值。
type ReviewCatalog struct{ ThemeName, StockName, TsCode string }

// ReviewTx 只提供数据操作；事务内的业务顺序由 Service 决定。
type ReviewTx interface {
	LockCandidate(context.Context, int64) (ReviewRecord, error)
	AuditByRequest(context.Context, string) (AuditEvent, bool, error)
	ApprovalSnapshot(context.Context, Candidate) (sources.Snapshot, error)
	LockCatalog(context.Context, Candidate) (ReviewCatalog, error)
	InsertMapping(context.Context, Candidate, int64) (int64, error)
	AppendAudit(context.Context, ReviewRequest, int64, int64) (AuditEvent, error)
	RefreshThemeCount(context.Context, int64) error
}

type ReviewStore interface {
	WithinReviewTx(context.Context, func(ReviewTx) error) error
	ListReviewQueue(context.Context, int64, int) ([]ReviewRecord, error)
}
