package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
)

type AIInspectionService struct {
	store  candidate.InspectionStore
	choose func(int) (int, error)
}

func NewAIInspectionService(store candidate.InspectionStore) (*AIInspectionService, error) {
	if store == nil {
		return nil, candidate.ErrInvalidInspection
	}
	return &AIInspectionService{store: store, choose: func(n int) (int, error) {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
		if err != nil {
			return 0, err
		}
		return int(v.Int64()), nil
	}}, nil
}

func (s *AIInspectionService) Batch(ctx context.Context, actorID int64, id string) (candidate.InspectionBatch, error) {
	if actorID <= 0 || (candidate.InspectionBatchRequest{RequestID: id, WindowSize: 1, SampleSize: 1}).Validate() != nil {
		return candidate.InspectionBatch{}, candidate.ErrInvalidInspection
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b, found, err := s.store.BatchByRequest(ctx, id)
	if err != nil {
		return b, err
	}
	if !found {
		return b, candidate.ErrInspectionNotFound
	}
	if b.ActorID != actorID {
		return candidate.InspectionBatch{}, candidate.ErrInspectionActor
	}
	return b, nil
}

func (s *AIInspectionService) Create(ctx context.Context, actorID int64, request candidate.InspectionBatchRequest) (candidate.InspectionBatch, error) {
	if actorID <= 0 {
		return candidate.InspectionBatch{}, candidate.ErrInvalidInspection
	}
	if err := request.Validate(); err != nil {
		return candidate.InspectionBatch{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var batch candidate.InspectionBatch
	err := s.store.WithinInspectionTx(ctx, func(tx candidate.InspectionTx) error {
		previous, found, err := tx.BatchByRequest(ctx, request.RequestID)
		if err != nil {
			return err
		}
		if found {
			if previous.ActorID != actorID || previous.Request != request {
				return candidate.ErrConflict
			}
			batch = previous
			return nil
		}
		window, err := tx.SamplingWindow(ctx, request.AfterReviewID, request.WindowSize)
		if err != nil {
			return err
		}
		eligible := make([]candidate.AuditEvent, 0, len(window))
		next := request.AfterReviewID
		if len(window) > request.WindowSize {
			return candidate.ErrInvalidInspection
		}
		for _, e := range window {
			if e.ID <= next || e.Request.Decision != "approve" || e.MappingID <= 0 || e.ActorID <= 0 || e.Request.Validate() != nil || e.CreatedAt.IsZero() {
				return candidate.ErrInvalidInspection
			}
			next = e.ID
			if e.ActorID != actorID {
				eligible = append(eligible, e)
			}
		}
		ids := make([]int64, 0, len(window))
		for _, e := range window {
			ids = append(ids, e.ID)
		}
		raw, err := json.Marshal(window)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		batch = candidate.InspectionBatch{Request: request, ActorID: actorID, PopulationCount: len(window), EligibleCount: len(eligible), NextReviewID: next, PopulationDigest: hex.EncodeToString(digest[:]), PopulationReviewIDs: ids}
		// 局部 Fisher-Yates：每个窗口内可选记录等概率、不放回，随机失败不落批次。
		n := min(request.SampleSize, len(eligible))
		for i := 0; i < n; i++ {
			j, err := s.choose(len(eligible) - i)
			if err != nil {
				return fmt.Errorf("生成复核随机样本失败: %w", err)
			}
			if j < 0 || j >= len(eligible)-i {
				return candidate.ErrInvalidInspection
			}
			eligible[i], eligible[i+j] = eligible[i+j], eligible[i]
		}
		batch, err = tx.InsertBatch(ctx, batch, eligible[:n])
		return err
	})
	if errors.Is(err, candidate.ErrConflict) {
		// 并发插入同一请求唯一键时，等待胜方提交后复用首次选择，不重新抽样。
		previous, found, readErr := s.store.BatchByRequest(ctx, request.RequestID)
		if readErr == nil && found && previous.ActorID == actorID && previous.Request == request {
			return previous, nil
		}
	}
	if err != nil {
		return candidate.InspectionBatch{}, err
	}
	return batch, nil
}

func (s *AIInspectionService) Item(ctx context.Context, actorID, id int64) (candidate.InspectionItem, error) {
	if actorID <= 0 || id <= 0 {
		return candidate.InspectionItem{}, candidate.ErrInvalidInspection
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	item, err := s.store.ItemByID(ctx, id)
	if err != nil {
		return item, err
	}
	if item.ActorID != actorID || item.Original.ActorID == actorID {
		return candidate.InspectionItem{}, candidate.ErrInspectionActor
	}
	if _, err := candidate.InspectionDigest(item); err != nil {
		return candidate.InspectionItem{}, err
	}
	return item, nil
}

func (s *AIInspectionService) Decide(ctx context.Context, actorID int64, request candidate.InspectionRequest) (candidate.InspectionEvent, error) {
	if actorID <= 0 {
		return candidate.InspectionEvent{}, candidate.ErrInvalidInspection
	}
	if err := request.Validate(); err != nil {
		return candidate.InspectionEvent{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var event candidate.InspectionEvent
	err := s.store.WithinInspectionTx(ctx, func(tx candidate.InspectionTx) error {
		item, err := tx.LockItem(ctx, request.ItemID)
		if err != nil {
			return err
		}
		if actorID != item.ActorID || actorID == item.Original.ActorID {
			return candidate.ErrInspectionActor
		}
		previous, found, err := tx.EventByRequest(ctx, request.RequestID)
		if err != nil {
			return err
		}
		if found {
			if previous.Request != request || previous.ActorID != actorID {
				return candidate.ErrConflict
			}
			event = previous
			return nil
		}
		if _, found, err := tx.EventByItem(ctx, request.ItemID); err != nil {
			return err
		} else if found {
			return candidate.ErrConflict
		}
		digest, err := candidate.InspectionDigest(item)
		if err != nil {
			return err
		}
		if digest != request.EvidenceDigest {
			return candidate.ErrEvidenceChanged
		}
		if request.Decision == "confirm" {
			snapshot, err := tx.ApprovalSnapshot(ctx, item.Candidate)
			if err != nil {
				return err
			}
			if err := candidate.ValidateAgainstSnapshot(item.Candidate, snapshot); err != nil {
				return err
			}
			catalog, err := tx.LockCatalog(ctx, item.Candidate)
			if err != nil {
				return err
			}
			key := func(v string) string { return strings.ToLower(guard.Normalize(v)) }
			if key(catalog.ThemeName) != key(item.Candidate.ThemeKey) || catalog.TsCode != item.Candidate.TsCode || (key(catalog.StockName) != key(item.Candidate.EntityKey) && key(catalog.TsCode) != key(item.Candidate.EntityKey)) {
				return candidate.ErrEvidenceChanged
			}
		}
		mapping, err := tx.LockMapping(ctx, item.Original.MappingID, item.Candidate.ThemeID)
		if err != nil {
			return err
		}
		if mapping.ID != item.Original.MappingID || !mapping.Matches(item.Candidate) || (request.Decision == "confirm" && mapping.Status != 1) {
			return candidate.ErrEvidenceChanged
		}
		if request.Decision == "issue" {
			if err := tx.Quarantine(ctx, mapping.ID, actorID); err != nil {
				return err
			}
		}
		event, err = tx.AppendInspection(ctx, request, actorID, mapping.ID)
		if err != nil {
			return err
		}
		if request.Decision == "issue" {
			return tx.RefreshThemeCount(ctx, item.Candidate.ThemeID)
		}
		return nil
	})
	if err != nil {
		return candidate.InspectionEvent{}, fmt.Errorf("复核决定失败: %w", err)
	}
	return event, nil
}

func (s *AIInspectionService) Issues(ctx context.Context, afterID int64, limit int) ([]candidate.InspectionEvent, error) {
	if afterID < 0 || limit < 1 || limit > 200 {
		return nil, candidate.ErrInvalidInspection
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.store.ListIssues(ctx, afterID, limit)
}
