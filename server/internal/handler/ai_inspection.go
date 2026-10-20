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
)

type aiInspectionService interface {
	Create(context.Context, int64, candidate.InspectionBatchRequest) (candidate.InspectionBatch, error)
	Batch(context.Context, int64, string) (candidate.InspectionBatch, error)
	Item(context.Context, int64, int64) (candidate.InspectionItem, error)
	Decide(context.Context, int64, candidate.InspectionRequest) (candidate.InspectionEvent, error)
	Issues(context.Context, int64, int) ([]candidate.InspectionEvent, error)
}
type AIInspectionHandler struct{ svc aiInspectionService }

func NewAIInspectionHandler(svc aiInspectionService) *AIInspectionHandler {
	return &AIInspectionHandler{svc: svc}
}
func (h *AIInspectionHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/v1/ai-inspection-batches", h.create)
	mux.HandleFunc("GET /internal/v1/ai-inspection-batches/{request_id}", h.batch)
	mux.HandleFunc("GET /internal/v1/ai-inspection-items/{id}", h.item)
	mux.HandleFunc("POST /internal/v1/ai-inspection-decisions", h.decide)
	mux.HandleFunc("GET /internal/v1/ai-inspection-issues", h.issues)
}
func (h *AIInspectionHandler) authorized(w http.ResponseWriter, r *http.Request) bool {
	if middleware.AIAdminActor(r.Context()) <= 0 {
		fail(w, 401, codeUnauthorized, "需要内部复核身份")
		return false
	}
	return true
}
func (h *AIInspectionHandler) decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if !h.authorized(w, r) {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		h.failure(w, candidate.ErrInvalidInspection)
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		h.failure(w, candidate.ErrInvalidInspection)
		return false
	}
	return true
}
func (h *AIInspectionHandler) create(w http.ResponseWriter, r *http.Request) {
	var request candidate.InspectionBatchRequest
	if !h.decode(w, r, &request) {
		return
	}
	b, err := h.svc.Create(r.Context(), middleware.AIAdminActor(r.Context()), request)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAIInspectionBatch(b))
}
func (h *AIInspectionHandler) batch(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	b, err := h.svc.Batch(r.Context(), middleware.AIAdminActor(r.Context()), r.PathValue("request_id"))
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAIInspectionBatch(b))
}
func (h *AIInspectionHandler) item(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.failure(w, candidate.ErrInvalidInspection)
		return
	}
	item, err := h.svc.Item(r.Context(), middleware.AIAdminActor(r.Context()), id)
	if err != nil {
		h.failure(w, err)
		return
	}
	d, err := dto.NewAIInspectionItem(item)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, d)
}
func (h *AIInspectionHandler) decide(w http.ResponseWriter, r *http.Request) {
	var request candidate.InspectionRequest
	if !h.decode(w, r, &request) {
		return
	}
	e, err := h.svc.Decide(r.Context(), middleware.AIAdminActor(r.Context()), request)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, dto.NewAIInspectionEvent(e))
}
func (h *AIInspectionHandler) issues(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	after, limit := int64(0), 100
	for key, values := range r.URL.Query() {
		if len(values) != 1 || (key != "after_id" && key != "limit") {
			h.failure(w, candidate.ErrInvalidInspection)
			return
		}
	}
	var err error
	if raw := r.URL.Query().Get("after_id"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.failure(w, candidate.ErrInvalidInspection)
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			h.failure(w, candidate.ErrInvalidInspection)
			return
		}
	}
	events, err := h.svc.Issues(r.Context(), after, limit)
	if err != nil {
		h.failure(w, err)
		return
	}
	out := dto.AIInspectionIssues{Items: []dto.AIInspectionEvent{}, NextAfterID: after}
	for _, e := range events {
		out.Items = append(out.Items, dto.NewAIInspectionEvent(e))
		out.NextAfterID = e.ID
	}
	ok(w, out)
}
func (h *AIInspectionHandler) failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, candidate.ErrInvalidInspection):
		fail(w, 400, codeBadRequest, "复核参数无效")
	case errors.Is(err, candidate.ErrInspectionNotFound):
		fail(w, 404, 40405, "复核记录不存在")
	case errors.Is(err, candidate.ErrInspectionActor):
		fail(w, 403, 40301, "仅允许分派的独立复核人操作")
	default:
		(&AIReviewHandler{}).reviewError(w, err)
	}
}
