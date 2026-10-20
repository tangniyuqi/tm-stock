package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
)

type AIReviewService struct{ store candidate.ReviewStore }

func NewAIReviewService(store candidate.ReviewStore) (*AIReviewService, error) {
	if store == nil {
		return nil, fmt.Errorf("AI 审核仓储不能为空")
	}
	return &AIReviewService{store: store}, nil
}

func (s *AIReviewService) ListPending(ctx context.Context, afterID int64, limit int) ([]candidate.ReviewRecord, error) {
	if afterID < 0 || limit < 1 || limit > 200 {
		return nil, candidate.ErrInvalidReview
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.store.ListReviewQueue(ctx, afterID, limit)
}

// Review 由经过鉴权的内部调用方传入 actorID；不接受请求体伪造身份。
func (s *AIReviewService) Review(ctx context.Context, actorID int64, request candidate.ReviewRequest) (candidate.AuditEvent, error) {
	if actorID <= 0 {
		return candidate.AuditEvent{}, candidate.ErrInvalidReview
	}
	if err := request.Validate(); err != nil {
		return candidate.AuditEvent{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var event candidate.AuditEvent
	err := s.store.WithinReviewTx(ctx, func(tx candidate.ReviewTx) error {
		record, err := tx.LockCandidate(ctx, request.CandidateID)
		if err != nil {
			return err
		}
		previous, found, err := tx.AuditByRequest(ctx, request.RequestID)
		if err != nil {
			return err
		}
		if found {
			if !previous.Matches(request, actorID) {
				return candidate.ErrConflict
			}
			event = previous
			return nil
		}
		if record.Status != candidate.Pending {
			return candidate.ErrConflict
		}
		digest, err := candidate.EvidenceDigest(record.Candidate)
		if err != nil {
			return err
		}
		if digest != request.EvidenceDigest {
			return candidate.ErrEvidenceChanged
		}
		var mappingID int64
		if request.Status() == candidate.Approved {
			snapshot, err := tx.ApprovalSnapshot(ctx, record.Candidate)
			if err != nil {
				return err
			}
			if err := candidate.ValidateAgainstSnapshot(record.Candidate, snapshot); err != nil {
				return err
			}
			catalog, err := tx.LockCatalog(ctx, record.Candidate)
			if err != nil {
				return err
			}
			key := func(value string) string { return strings.ToLower(guard.Normalize(value)) }
			if key(catalog.ThemeName) != key(record.Candidate.ThemeKey) || catalog.TsCode != record.Candidate.TsCode ||
				(key(catalog.StockName) != key(record.Candidate.EntityKey) && key(catalog.TsCode) != key(record.Candidate.EntityKey)) {
				return candidate.ErrEvidenceChanged
			}
			mappingID, err = tx.InsertMapping(ctx, record.Candidate, actorID)
			if err != nil {
				return err
			}
			if mappingID <= 0 {
				return fmt.Errorf("审核通过未返回有效归属编号")
			}
		}
		event, err = tx.AppendAudit(ctx, request, actorID, mappingID)
		if err != nil {
			return err
		}
		if request.Status() == candidate.Approved {
			if err := tx.RefreshThemeCount(ctx, record.Candidate.ThemeID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return candidate.AuditEvent{}, fmt.Errorf("审核候选失败: %w", err)
	}
	return event, nil
}
