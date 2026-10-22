package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiConversationService interface {
	Create(context.Context, int64, string, string) (dto.AIConversationReceipt, error)
	Receipt(context.Context, int64, string) (dto.AIConversationReceipt, error)
	Detail(context.Context, int64, int64) (dto.AIConversation, error)
	List(context.Context, int64, int64, bool, int) (dto.AIConversationPage, error)
	Rename(context.Context, int64, int64, int64, string) (dto.AIConversation, error)
	Archive(context.Context, int64, int64, int64) (dto.AIConversation, error)
	Messages(context.Context, int64, int64, int64, int) (dto.AIConversationMessages, error)
}
type AIConversationHandler struct{ svc aiConversationService }

func NewAIConversationHandler(svc aiConversationService) *AIConversationHandler {
	return &AIConversationHandler{svc}
}
func (h *AIConversationHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	for _, pattern := range []string{"POST /api/v1/ai/conversations", "GET /api/v1/ai/conversations", "GET /api/v1/ai/conversation-receipt", "GET /api/v1/ai/conversations/{id}", "PATCH /api/v1/ai/conversations/{id}", "POST /api/v1/ai/conversations/{id}/archive", "GET /api/v1/ai/conversations/{id}/messages"} {
		mux.Handle(pattern, auth(http.HandlerFunc(h.serve)))
	}
}
func conversationFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, conversation.ErrInvalid), errors.Is(err, points.ErrInvalid):
		fail(w, 400, 40001, "会话参数无效")
	case errors.Is(err, conversation.ErrNotFound):
		fail(w, 404, 40403, "会话不存在")
	case errors.Is(err, conversation.ErrConflict):
		fail(w, 409, 40903, "会话或原键已变化，请查询当前状态")
	default:
		fail(w, 503, 50307, "会话操作未确认，请查询原键或当前会话")
	}
}
func conversationDecimal(raw string, zero bool) (int64, error) {
	if zero && raw == "0" {
		return 0, nil
	}
	n, err := positiveDecimal(raw)
	if err != nil || !conversation.ID(n) {
		return 0, conversation.ErrInvalid
	}
	return n, nil
}

func conversationReadRequest(r *http.Request, allowed ...string) (map[string]string, error) {
	q, err := feedbackReadRequest(r, allowed...)
	if err != nil {
		return nil, conversation.ErrInvalid
	}
	return q, nil
}
func (h *AIConversationHandler) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, 40100, "需要会员身份")
		return
	}
	if r.Method == http.MethodHead {
		conversationFailure(w, conversation.ErrInvalid)
		return
	}
	if h == nil || h.svc == nil {
		conversationFailure(w, conversation.ErrInvariant)
		return
	}
	ctx := r.Context()
	var out any
	var err error
	if r.URL.Path == "/api/v1/ai/conversation-receipt" {
		var key string
		if _, err = conversationReadRequest(r); err == nil {
			key, err = taskCreationKey(r)
		}
		if err == nil {
			out, err = h.svc.Receipt(ctx, uid, key)
		}
	} else if r.PathValue("id") == "" {
		if r.Method == "POST" {
			var key string
			var body struct {
				Title string `json:"title"`
			}
			key, err = taskCreationKey(r)
			if err == nil {
				err = taskSubmissionBody(w, r, &body, "title")
			}
			if err == nil {
				out, err = h.svc.Create(ctx, uid, key, body.Title)
			}
		} else {
			var q map[string]string
			q, err = conversationReadRequest(r, "before_id", "limit", "archived")
			before, limit, archived := int64(0), int64(20), false
			if err == nil {
				if raw, ok := q["before_id"]; ok {
					before, err = conversationDecimal(raw, false)
				}
			}
			if err == nil {
				if raw, ok := q["limit"]; ok {
					limit, err = conversationDecimal(raw, false)
					if limit > 100 {
						err = conversation.ErrInvalid
					}
				}
			}
			if err == nil {
				if raw, ok := q["archived"]; ok {
					if raw != "0" && raw != "1" {
						err = conversation.ErrInvalid
					} else {
						archived = raw == "1"
					}
				}
			}
			if err == nil {
				out, err = h.svc.List(ctx, uid, before, archived, int(limit))
			}
		}
	} else {
		var id int64
		id, err = conversationDecimal(r.PathValue("id"), false)
		if err == nil {
			switch {
			case r.URL.Path == "/api/v1/ai/conversations/"+r.PathValue("id")+"/messages":
				var q map[string]string
				q, err = conversationReadRequest(r, "after_seq", "limit")
				after, limit := int64(0), int64(20)
				if err == nil {
					if raw, ok := q["after_seq"]; ok {
						after, err = conversationDecimal(raw, true)
					}
				}
				if err == nil {
					if raw, ok := q["limit"]; ok {
						limit, err = conversationDecimal(raw, false)
						if limit > 100 {
							err = conversation.ErrInvalid
						}
					}
				}
				if err == nil {
					out, err = h.svc.Messages(ctx, uid, id, after, int(limit))
				}
			case r.Method == "PATCH":
				var body struct {
					Title   string `json:"title"`
					Version int64  `json:"expectedVersion"`
				}
				err = taskSubmissionBody(w, r, &body, "title", "expectedVersion")
				if err == nil {
					out, err = h.svc.Rename(ctx, uid, id, body.Version, body.Title)
				}
			case r.Method == "POST":
				var body struct {
					Version int64 `json:"expectedVersion"`
				}
				err = taskSubmissionBody(w, r, &body, "expectedVersion")
				if err == nil {
					out, err = h.svc.Archive(ctx, uid, id, body.Version)
				}
			default:
				if _, err = conversationReadRequest(r); err == nil {
					out, err = h.svc.Detail(ctx, uid, id)
				}
			}
		}
	}
	if err != nil {
		conversationFailure(w, err)
		return
	}
	ok(w, out)
}
