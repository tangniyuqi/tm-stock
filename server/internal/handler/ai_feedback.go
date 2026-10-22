package handler

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiFeedbackService interface {
	Submit(context.Context, int64, int64, string) (dto.AIFeedback, error)
	ForReport(context.Context, int64, int64) (dto.AIFeedbackList, error)
	List(context.Context, int64, int64, int) (dto.AIFeedbackPage, error)
	Detail(context.Context, int64, int64) (dto.AIInternalFeedback, error)
	Decide(context.Context, int64, int64, feedback.Decision) (dto.AIFeedback, error)
}
type AIFeedbackHandler struct{ svc aiFeedbackService }

func NewAIFeedbackHandler(svc aiFeedbackService) *AIFeedbackHandler { return &AIFeedbackHandler{svc} }
func (h *AIFeedbackHandler) RegisterMember(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/tasks/{id}/feedback", auth(http.HandlerFunc(h.member)))
	mux.Handle("POST /api/v1/ai/tasks/{id}/feedback", auth(http.HandlerFunc(h.member)))
}
func (h *AIFeedbackHandler) RegisterInternal(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/ai-feedback", h.internal)
	mux.HandleFunc("GET /internal/v1/ai-feedback/{id}", h.internal)
	mux.HandleFunc("POST /internal/v1/ai-feedback/{id}/decisions", h.internal)
}
func feedbackReadRequest(r *http.Request, allowed ...string) (map[string]string, error) {
	q, err := queryTaskRead(r, allowed...)
	if err != nil || r.URL.ForceQuery || len(r.Header.Values("Content-Encoding")) != 0 {
		return nil, feedback.ErrInvalid
	}
	if r.Body != nil {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(raw) != 0 {
			return nil, feedback.ErrInvalid
		}
	}
	return q, nil
}
func (h *AIFeedbackHandler) member(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	id, err := positiveDecimal(r.PathValue("id"))
	if err != nil || !feedback.ID(id) {
		h.failure(w, feedback.ErrInvalid)
		return
	}
	if h == nil || h.svc == nil {
		h.failure(w, feedback.ErrInvariant)
		return
	}
	if r.Method == "POST" {
		var body struct {
			Category string `json:"category"`
		}
		if err := taskSubmissionBody(w, r, &body, "category"); err != nil || !feedback.Category(body.Category) {
			h.failure(w, feedback.ErrInvalid)
			return
		}
		out, err := h.svc.Submit(r.Context(), uid, id, body.Category)
		if err != nil {
			h.failure(w, err)
			return
		}
		ok(w, out)
		return
	}
	if _, err := feedbackReadRequest(r); err != nil {
		h.failure(w, err)
		return
	}
	out, err := h.svc.ForReport(r.Context(), uid, id)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, out)
}
func (h *AIFeedbackHandler) internal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor := middleware.AIAdminActor(r.Context())
	if actor <= 0 {
		fail(w, 401, codeUnauthorized, "需要内部处理身份")
		return
	}
	if h == nil || h.svc == nil {
		h.failure(w, feedback.ErrInvariant)
		return
	}
	raw := r.PathValue("id")
	if raw == "" {
		q, err := feedbackReadRequest(r, "after_id", "limit")
		if err != nil {
			h.failure(w, err)
			return
		}
		after, limit := int64(0), int64(20)
		if s, ok := q["after_id"]; ok {
			after, err = positiveDecimal(s)
			if err != nil {
				h.failure(w, feedback.ErrInvalid)
				return
			}
		}
		if s, ok := q["limit"]; ok {
			limit, err = positiveDecimal(s)
			if err != nil || limit > 100 {
				h.failure(w, feedback.ErrInvalid)
				return
			}
		}
		out, err := h.svc.List(r.Context(), actor, after, int(limit))
		if err != nil {
			h.failure(w, err)
			return
		}
		ok(w, out)
		return
	}
	id, err := positiveDecimal(raw)
	if err != nil || !feedback.ID(id) {
		h.failure(w, feedback.ErrInvalid)
		return
	}
	if r.Method == "POST" {
		var d feedback.Decision
		if err := taskSubmissionBody(w, r, &d, "expectedVersion", "status", "resolution"); err != nil || !d.Valid() {
			h.failure(w, feedback.ErrInvalid)
			return
		}
		out, err := h.svc.Decide(r.Context(), actor, id, d)
		if err != nil {
			h.failure(w, err)
			return
		}
		ok(w, out)
		return
	}
	if _, err := feedbackReadRequest(r); err != nil {
		h.failure(w, err)
		return
	}
	out, err := h.svc.Detail(r.Context(), actor, id)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, out)
}
func (h *AIFeedbackHandler) failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, feedback.ErrInvalid), errors.Is(err, points.ErrInvalid):
		fail(w, 400, 40001, "反馈参数无效")
	case errors.Is(err, feedback.ErrNotFound):
		fail(w, 404, 40403, "反馈或任务不存在")
	case errors.Is(err, feedback.ErrNoReport):
		fail(w, 422, 42204, "该任务暂无可反馈的报告")
	case errors.Is(err, feedback.ErrConflict):
		fail(w, 409, 40904, "反馈已变化，请查询当前状态")
	default:
		fail(w, 503, 50306, "反馈操作未确认，请查询该报告反馈")
	}
}
