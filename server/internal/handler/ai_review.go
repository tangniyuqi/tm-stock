package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type aiReviewService interface {
	ListPending(context.Context, int64, int) ([]candidate.ReviewRecord, error)
	Review(context.Context, int64, candidate.ReviewRequest) (candidate.AuditEvent, error)
}

type AIReviewHandler struct{ svc aiReviewService }

func NewAIReviewHandler(svc aiReviewService) *AIReviewHandler { return &AIReviewHandler{svc: svc} }

// Register 只供 cmd/ai-admin 独立监听使用，不在公众 cmd/api 中注册。
func (h *AIReviewHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/ai-review-candidates", h.list)
	mux.HandleFunc("POST /internal/v1/ai-review-decisions", h.review)
}

func (h *AIReviewHandler) list(w http.ResponseWriter, r *http.Request) {
	if middleware.AIAdminActor(r.Context()) <= 0 {
		fail(w, http.StatusUnauthorized, codeUnauthorized, "需要内部审核身份")
		return
	}
	afterID, limit := int64(0), 100
	var err error
	if raw := r.URL.Query().Get("after_id"); raw != "" {
		afterID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.reviewError(w, candidate.ErrInvalidReview)
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			h.reviewError(w, candidate.ErrInvalidReview)
			return
		}
	}
	records, err := h.svc.ListPending(r.Context(), afterID, limit)
	if err != nil {
		h.reviewError(w, err)
		return
	}
	queue := dto.AIReviewQueue{Items: make([]dto.AIReviewItem, 0, len(records)), NextAfterID: afterID}
	for _, record := range records {
		item, err := dto.NewAIReviewItem(record.Candidate)
		if err != nil {
			h.reviewError(w, err)
			return
		}
		queue.Items = append(queue.Items, item)
		queue.NextAfterID = item.ID
	}
	ok(w, queue)
}

func (h *AIReviewHandler) review(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.AIAdminActor(r.Context())
	if actorID <= 0 {
		fail(w, http.StatusUnauthorized, codeUnauthorized, "需要内部审核身份")
		return
	}
	var request candidate.ReviewRequest
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		h.reviewError(w, candidate.ErrInvalidReview)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.reviewError(w, candidate.ErrInvalidReview)
		return
	}
	event, err := h.svc.Review(r.Context(), actorID, request)
	if err != nil {
		h.reviewError(w, err)
		return
	}
	ok(w, dto.AIReviewEvent{ID: event.ID, RequestID: event.Request.RequestID, CandidateID: event.Request.CandidateID,
		ActorID: event.ActorID, Decision: event.Request.Decision, Reason: event.Request.Reason, MappingID: event.MappingID, CreatedAt: event.CreatedAt.UnixMilli()})
}

func (h *AIReviewHandler) reviewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, candidate.ErrInvalidReview):
		fail(w, http.StatusBadRequest, codeBadRequest, "审核参数无效")
	case errors.Is(err, candidate.ErrNotFound):
		fail(w, http.StatusNotFound, 40403, "候选不存在")
	case errors.Is(err, candidate.ErrConflict), errors.Is(err, candidate.ErrEvidenceChanged):
		fail(w, http.StatusConflict, 40901, "审核状态或证据已变化，请重新导出核对")
	case errors.Is(err, sources.ErrUnauthorized):
		fail(w, http.StatusUnprocessableEntity, 42202, "来源授权无效，不能通过审核")
	default:
		fail(w, http.StatusInternalServerError, codeInternal, "审核服务暂不可用")
	}
}
