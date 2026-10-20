package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var ErrInvalidLibraryRequest = errors.New("资料管理参数无效")

type librarySourceService interface {
	PrepareImport(context.Context, int64, string, string, []byte) (sources.Snapshot, error)
	PrepareFetch(context.Context, int64, string) (sources.Snapshot, error)
}
type libraryReader interface {
	sources.MutationStore
	SourceByID(context.Context, int64) (sources.Source, error)
	SnapshotByID(context.Context, int64) (sources.Snapshot, error)
}
type AILibraryService struct {
	sources librarySourceService
	reader  libraryReader
}

func NewAILibraryService(sourceService librarySourceService, reader libraryReader) (*AILibraryService, error) {
	if sourceService == nil || reader == nil {
		return nil, ErrInvalidLibraryRequest
	}
	return &AILibraryService{sources: sourceService, reader: reader}, nil
}

func (s *AILibraryService) RegisterSource(ctx context.Context, actor int64, source sources.Source) (sources.Source, error) {
	if actor <= 0 || source.ID != 0 || source.Enabled || source.MinInterval < 0 || source.MinInterval > 24*time.Hour {
		return sources.Source{}, ErrInvalidLibraryRequest
	}
	if source.MinInterval < time.Second {
		source.MinInterval = time.Second
	}
	normalized, err := source.NormalizeRegistration()
	if err != nil {
		return sources.Source{}, fmt.Errorf("来源登记参数未通过校验: %w", ErrInvalidLibraryRequest)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var created sources.Source
	err = s.reader.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
		var err error
		created, err = tx.RegisterSource(ctx, normalized)
		if err != nil {
			return err
		}
		return appendLibraryAudit(ctx, tx, sources.SourceAudit{ActorID: actor, Operation: "register", After: created})
	})
	if err != nil {
		return sources.Source{}, err
	}
	return created, nil
}
func (s *AILibraryService) SourceByID(ctx context.Context, id int64) (sources.Source, error) {
	if id <= 0 {
		return sources.Source{}, ErrInvalidLibraryRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.reader.SourceByID(ctx, id)
}
func (s *AILibraryService) EnableSource(ctx context.Context, actor, id int64, enabled bool) error {
	if actor <= 0 || id <= 0 {
		return ErrInvalidLibraryRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.reader.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
		before, err := tx.SourceByID(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.SetSourceEnabled(ctx, id, enabled); err != nil {
			return err
		}
		after, err := tx.SourceByID(ctx, id)
		if err != nil {
			return err
		}
		operation := "disable"
		if enabled {
			operation = "enable"
		}
		return appendLibraryAudit(ctx, tx, sources.SourceAudit{ActorID: actor, Operation: operation, Before: &before, After: after})
	})
}
func (s *AILibraryService) ImportText(ctx context.Context, actor, id int64, url, media string, body []byte) (sources.Snapshot, error) {
	if actor <= 0 || id <= 0 || len(body) == 0 {
		return sources.Snapshot{}, ErrInvalidLibraryRequest
	}
	if len(body) > 2<<20 {
		return sources.Snapshot{}, sources.ErrBodyTooLarge
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	snapshot, err := s.sources.PrepareImport(ctx, id, url, media, body)
	if err != nil {
		return sources.Snapshot{}, err
	}
	return s.saveAuditedSnapshot(ctx, actor, id, "import", snapshot)
}
func (s *AILibraryService) FetchSnapshot(ctx context.Context, actor, id int64, url string) (sources.Snapshot, error) {
	if actor <= 0 || id <= 0 {
		return sources.Snapshot{}, ErrInvalidLibraryRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	snapshot, err := s.sources.PrepareFetch(ctx, id, url)
	if err != nil {
		return sources.Snapshot{}, err
	}
	return s.saveAuditedSnapshot(ctx, actor, id, "fetch", snapshot)
}

func (s *AILibraryService) saveAuditedSnapshot(ctx context.Context, actor, id int64, operation string, snapshot sources.Snapshot) (sources.Snapshot, error) {
	if snapshot.SourceID != id {
		return sources.Snapshot{}, sources.ErrInvalidSourceAudit
	}
	var stored sources.Snapshot
	err := s.reader.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
		current, err := tx.SourceByID(ctx, id)
		if err != nil {
			return err
		}
		// 保存函数在持锁状态下用数据库时钟重新复核授权，不信任采集前的登记副本。
		stored, err = tx.SaveSnapshot(ctx, snapshot)
		if err != nil {
			return err
		}
		return appendLibraryAudit(ctx, tx, sources.SourceAudit{ActorID: actor, Operation: operation, After: current,
			SnapshotID: stored.ID, ContentHash: stored.Hash, URL: stored.URL})
	})
	if err != nil {
		return sources.Snapshot{}, err
	}
	return stored, nil
}

func appendLibraryAudit(ctx context.Context, tx sources.MutationTx, event sources.SourceAudit) error {
	if err := event.Validate(); err != nil {
		return err
	}
	return tx.AppendSourceAudit(ctx, event)
}

func (s *AILibraryService) SourceAudits(ctx context.Context, sourceID, afterID int64, limit int) ([]sources.SourceAudit, error) {
	if sourceID <= 0 || afterID < 0 || limit < 1 || limit > 200 {
		return nil, ErrInvalidLibraryRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := s.reader.SourceByID(ctx, sourceID); err != nil {
		return nil, err
	}
	return s.reader.ListSourceAudits(ctx, sourceID, afterID, limit)
}
func (s *AILibraryService) SnapshotByID(ctx context.Context, id int64) (sources.Snapshot, error) {
	if id <= 0 {
		return sources.Snapshot{}, ErrInvalidLibraryRequest
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.reader.SnapshotByID(ctx, id)
}
