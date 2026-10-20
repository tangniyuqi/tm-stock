package sources

import (
	"context"
	"errors"
	"reflect"
	"time"
)

var ErrInvalidSourceAudit = errors.New("来源操作审计无效")

// SourceAudit 只记录已提交操作；正文留在不可变快照中，不重复放入操作日志。
type SourceAudit struct {
	ID          int64
	ActorID     int64
	Operation   string
	Before      *Source
	After       Source
	SnapshotID  int64
	ContentHash string
	URL         string
	CreatedAt   time.Time
}

// Validate 检查历史副本，不因授权后来过期而拒绝读取历史。
func (a SourceAudit) Validate() error {
	if a.ActorID <= 0 || a.After.ID <= 0 {
		return ErrInvalidSourceAudit
	}
	for _, source := range []*Source{a.Before, &a.After} {
		if source == nil {
			continue
		}
		copy := *source
		copy.Enabled = false
		normalized, err := copy.NormalizeRegistration()
		if err != nil || !reflect.DeepEqual(copy, normalized) || copy.MinInterval < time.Second || copy.MinInterval > 24*time.Hour {
			return ErrInvalidSourceAudit
		}
	}
	snapshotOperation := false
	switch a.Operation {
	case "register":
		if a.Before != nil || a.After.Enabled {
			return ErrInvalidSourceAudit
		}
	case "enable", "disable":
		if a.Before == nil || a.Before.ID != a.After.ID || a.After.Enabled != (a.Operation == "enable") {
			return ErrInvalidSourceAudit
		}
		before := *a.Before
		before.Enabled = a.After.Enabled
		if !reflect.DeepEqual(before, a.After) {
			return ErrInvalidSourceAudit
		}
	case "import", "fetch":
		snapshotOperation = true
		if a.Before != nil || !a.After.Enabled {
			return ErrInvalidSourceAudit
		}
	default:
		return ErrInvalidSourceAudit
	}
	if snapshotOperation {
		if a.SnapshotID <= 0 || len(a.ContentHash) != 64 {
			return ErrInvalidSourceAudit
		}
		for _, c := range a.ContentHash {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return ErrInvalidSourceAudit
			}
		}
		url, err := ValidateURL(a.After.Domain, a.URL)
		if err != nil || url != a.URL {
			return ErrInvalidSourceAudit
		}
	} else if a.SnapshotID != 0 || a.ContentHash != "" || a.URL != "" {
		return ErrInvalidSourceAudit
	}
	return nil
}

// MutationTx 由业务服务把来源/快照变更和审计组合为同一事务。
type MutationTx interface {
	SourceByID(context.Context, int64) (Source, error) // 写事务内加行锁。
	RegisterSource(context.Context, Source) (Source, error)
	SetSourceEnabled(context.Context, int64, bool) error
	SaveSnapshot(context.Context, Snapshot) (Snapshot, error)
	AppendSourceAudit(context.Context, SourceAudit) error
}

type MutationStore interface {
	WithinSourceTx(context.Context, func(MutationTx) error) error
	ListSourceAudits(context.Context, int64, int64, int) ([]SourceAudit, error)
}
