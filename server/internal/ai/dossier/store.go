package dossier

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var (
	ErrNotFound = errors.New("专题版本不可用")
	ErrConflict = errors.New("专题版本身份或内容冲突")
)

type EvidenceRef struct {
	ID, SourceID int64
	Hash         string
}
type StoredVersion struct {
	Identity  DraftIdentity
	Digest    string
	Canonical []byte
	Evidence  []EvidenceRef
}

func (StoredVersion) String() string     { return "专题版本存证（内部引用已隐藏）" }
func (v StoredVersion) GoString() string { return v.String() }

type Approval struct {
	Digest, Reference string
	Until             time.Time
	Enabled           bool
}

type Store interface {
	Within(context.Context, bool, func(Tx) error) error
}

// Tx不提供批准写入；发布与会员许可由独立受控管理流程维护，不由读取创建。
type Tx interface {
	Now(context.Context) (time.Time, error)
	Member(context.Context, int64) error
	Theme(context.Context, int64) error
	Version(context.Context, int64, int64) (StoredVersion, error)
	Latest(context.Context, int64) (DraftIdentity, error)
	Insert(context.Context, StoredVersion) error
	Publication(context.Context, int64, int64) (Approval, error)
	Grant(context.Context, int64, int64, int64) (Approval, error)
	Source(context.Context, int64) (sources.Source, error)
	Snapshot(context.Context, int64) (sources.Snapshot, error)
}

func (d Draft) SnapshotIDs() ([]int64, error) {
	if len(d.Cards) > MaxCards || len(d.Gaps) > MaxCards {
		return nil, ErrInvalidDraft
	}
	ids := []int64{}
	for _, c := range d.Cards {
		ids = append(ids, c.Statement.SnapshotID)
		for _, f := range []Field{c.Subject, c.PublishedDate, c.EffectiveDate, c.Period, c.Scope, c.Stage, c.Basis, c.Unit} {
			if f.State == "known" {
				ids = append(ids, f.Ref.SnapshotID)
			}
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) > MaxDraftSnapshots {
		return nil, ErrInvalidDraft
	}
	for _, id := range ids {
		if !conversation.ID(id) {
			return nil, ErrInvalidDraft
		}
	}
	return ids, nil
}

func (b *BoundDraft) Stored() StoredVersion {
	if b == nil {
		return StoredVersion{}
	}
	return StoredVersion{b.identity, b.digest, b.Canonical(), slices.Clone(b.evidence)}
}
