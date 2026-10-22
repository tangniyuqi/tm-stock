package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiConversationTaskService interface {
	Prepare(context.Context, int64, int64, string, conversation.IntentInput) (dto.AIConversationTask, error)
	Confirm(context.Context, int64, int64, int64) (dto.AIConversationTask, error)
	Reconcile(context.Context, int64, int64, int64) (dto.AIConversationTask, error)
	Detail(context.Context, int64, int64, int64) (dto.AIConversationTask, error)
	Receipt(context.Context, int64, int64, string) (dto.AIConversationTask, error)
	List(context.Context, int64, int64, int64, int) (dto.AIConversationTasks, error)
}
type AIConversationTaskHandler struct{ svc aiConversationTaskService }

func NewAIConversationTaskHandler(svc aiConversationTaskService) *AIConversationTaskHandler {
	return &AIConversationTaskHandler{svc}
}
func (h *AIConversationTaskHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler, writes bool) {
	patterns := []string{"GET /api/v1/ai/conversations/{id}/task-intents", "GET /api/v1/ai/conversations/{id}/task-intent-receipt", "GET /api/v1/ai/conversations/{id}/task-intents/{intent}", "POST /api/v1/ai/conversations/{id}/task-intents/{intent}/reconcile"}
	if writes {
		patterns = append(patterns, "POST /api/v1/ai/conversations/{id}/task-intents", "POST /api/v1/ai/conversations/{id}/task-intents/{intent}/confirm")
	}
	for _, pattern := range patterns {
		next := auth(http.HandlerFunc(h.serve))
		mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Vary", "Authorization")
			next.ServeHTTP(w, r)
		}))
	}
}
func intentFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, conversation.ErrUnconfirmed):
		fail(w, 503, 50308, "任务创建未确认，请核查原意图，勿重新提交")
	case errors.Is(err, task.ErrCreationDisabled), errors.Is(err, task.ErrAdmissionUnavailable), errors.Is(err, points.ErrPriceChanged), errors.Is(err, points.ErrInsufficient):
		taskSubmissionError(w, err)
	default:
		conversationFailure(w, err)
	}
}
func (h *AIConversationTaskHandler) serve(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if !conversation.ID(uid) {
		fail(w, 401, 40100, "需要会员身份")
		return
	}
	if h == nil || h.svc == nil {
		intentFailure(w, conversation.ErrInvariant)
		return
	}
	if r.Method == http.MethodHead {
		intentFailure(w, conversation.ErrInvalid)
		return
	}
	cid, err := conversationDecimal(r.PathValue("id"), false)
	if err != nil {
		intentFailure(w, err)
		return
	}
	var out any
	ctx := r.Context()
	rawID := r.PathValue("intent")
	switch {
	case strings.HasSuffix(r.URL.Path, "/task-intent-receipt"):
		var key string
		if _, err = conversationReadRequest(r); err == nil {
			key, err = taskCreationKey(r)
		}
		if err == nil {
			out, err = h.svc.Receipt(ctx, uid, cid, key)
		}
	case rawID == "" && r.Method == "POST":
		var in conversation.IntentInput
		var key string
		key, err = taskCreationKey(r)
		if err == nil {
			err = taskSubmissionBody(w, r, &in, "analysis_type", "subject_type", "subject_id", "price_version")
		}
		if err == nil {
			out, err = h.svc.Prepare(ctx, uid, cid, key, in)
		}
	case rawID == "":
		var q map[string]string
		q, err = conversationReadRequest(r, "before_id", "limit")
		before, limit := int64(0), int64(20)
		if err == nil {
			if v, ok := q["before_id"]; ok {
				before, err = conversationDecimal(v, false)
			}
		}
		if err == nil {
			if v, ok := q["limit"]; ok {
				limit, err = conversationDecimal(v, false)
				if limit > 100 {
					err = conversation.ErrInvalid
				}
			}
		}
		if err == nil {
			out, err = h.svc.List(ctx, uid, cid, before, int(limit))
		}
	default:
		var id int64
		id, err = conversationDecimal(rawID, false)
		if err == nil {
			switch {
			case strings.HasSuffix(r.URL.Path, "/confirm"):
				var body struct {
					Confirmed bool `json:"confirmed"`
				}
				err = taskSubmissionBody(w, r, &body, "confirmed")
				if err == nil && !body.Confirmed {
					err = conversation.ErrInvalid
				}
				if err == nil {
					out, err = h.svc.Confirm(ctx, uid, cid, id)
				}
			case strings.HasSuffix(r.URL.Path, "/reconcile"):
				var body struct{}
				err = taskSubmissionBody(w, r, &body)
				if err == nil {
					out, err = h.svc.Reconcile(ctx, uid, cid, id)
				}
			default:
				if _, err = conversationReadRequest(r); err == nil {
					out, err = h.svc.Detail(ctx, uid, cid, id)
				}
			}
		}
	}
	if err != nil {
		intentFailure(w, err)
		return
	}
	ok(w, out)
}
