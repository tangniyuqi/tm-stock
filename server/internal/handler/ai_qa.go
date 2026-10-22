package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiQAService interface {
	Receipt(context.Context, string, int64, int64, string) (dto.AIQAStatus, error)
	Status(context.Context, string, int64, int64, int64) (dto.AIQAStatus, error)
	Answer(context.Context, string, int64, int64, int64) (dto.AIQAAnswer, error)
	Submit(context.Context, string, int64, int64, string, dto.AIQAInput) (dto.AIQAStatus, error)
}
type AIQAHandler struct{ svc aiQAService }

func NewAIQAHandler(svc aiQAService) *AIQAHandler { return &AIQAHandler{svc} }
func (h *AIQAHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler, writes bool) {
	for _, p := range []string{"GET /api/v1/ai/conversations/{id}/question-receipt", "GET /api/v1/ai/conversations/{id}/questions/{seq}", "GET /api/v1/ai/conversations/{id}/questions/{seq}/answer"} {
		mux.Handle(p, auth(http.HandlerFunc(h.serve)))
	}
	if writes {
		mux.Handle("POST /api/v1/ai/conversations/{id}/questions", auth(http.HandlerFunc(h.serve)))
	}
}
func qaFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, memberauth.ErrUnauthorized):
		fail(w, 401, 40100, "会员身份无效，请重新登录")
	case errors.Is(err, qa.ErrInvalid), errors.Is(err, conversation.ErrInvalid), errors.Is(err, points.ErrInvalid):
		fail(w, 400, 40001, "问答参数无效")
	case errors.Is(err, qa.ErrNotFound), errors.Is(err, conversation.ErrNotFound):
		fail(w, 404, 40403, "问题不存在")
	case errors.Is(err, qa.ErrConflict), errors.Is(err, conversation.ErrConflict):
		fail(w, 409, 40903, "原问题或会话已变化，请查询当前状态")
	case errors.Is(err, qa.ErrQuota):
		fail(w, 429, 42902, "问答安全配额不足，请查询原问题状态")
	default:
		fail(w, 503, 50309, "问答暂不可用或结果未确认，请查询原问题")
	}
}
func (h *AIQAHandler) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		qaFailure(w, memberauth.ErrUnauthorized)
		return
	}
	if h == nil || h.svc == nil {
		qaFailure(w, qa.ErrUnavailable)
		return
	}
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		qaFailure(w, memberauth.ErrUnauthorized)
		return
	}
	scheme, token, found := strings.Cut(values[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 8192 || strings.ContainsAny(token, " \t\r\n,") {
		qaFailure(w, memberauth.ErrUnauthorized)
		return
	}
	cid, err := conversationDecimal(r.PathValue("id"), false)
	if err != nil {
		qaFailure(w, err)
		return
	}
	var out any
	if r.Method == http.MethodPost {
		var in dto.AIQAInput
		var key string
		key, err = taskCreationKey(r)
		if err == nil {
			err = boundedSubmissionBodyOptional(w, r, &in, 8192, []string{"version", "subjectType", "subjectId", "skill", "question"}, []string{"taskId"})
		}
		if err == nil {
			out, err = h.svc.Submit(r.Context(), token, uid, cid, key, in)
		}
	} else if r.Method == http.MethodGet {
		_, err = conversationReadRequest(r)
		if err == nil && strings.HasSuffix(r.URL.Path, "/question-receipt") {
			var key string
			key, err = taskCreationKey(r)
			if err == nil {
				out, err = h.svc.Receipt(r.Context(), token, uid, cid, key)
			}
		} else if err == nil {
			var seq int64
			seq, err = conversationDecimal(r.PathValue("seq"), false)
			if err == nil {
				if strings.HasSuffix(r.URL.Path, "/answer") {
					out, err = h.svc.Answer(r.Context(), token, uid, cid, seq)
				} else {
					out, err = h.svc.Status(r.Context(), token, uid, cid, seq)
				}
			}
		}
	} else {
		err = qa.ErrInvalid
	}
	if err != nil {
		qaFailure(w, err)
		return
	}
	ok(w, out)
}
