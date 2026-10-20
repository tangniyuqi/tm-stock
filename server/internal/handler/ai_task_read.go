package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiTaskReader interface {
	History(context.Context, int64, int64, int) (dto.AITaskHistory, error)
	Progress(context.Context, int64, int64) (dto.AITaskProgress, error)
	Events(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error)
}
type AITaskReadHandler struct{ svc aiTaskReader }

func NewAITaskReadHandler(svc aiTaskReader) *AITaskReadHandler { return &AITaskReadHandler{svc} }

// Register 只注册 GET，不提供创建、取消、扣费、报告或外部 Agent 调用。
func (h *AITaskReadHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/tasks", auth(http.HandlerFunc(h.history)))
	mux.Handle("GET /api/v1/ai/tasks/{id}", auth(http.HandlerFunc(h.progress)))
	mux.Handle("GET /api/v1/ai/tasks/{id}/events", auth(http.HandlerFunc(h.events)))
}

func positiveDecimal(raw string) (int64, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != raw {
		return 0, points.ErrInvalid
	}
	return n, nil
}

func (h *AITaskReadHandler) history(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	q, err := queryTaskRead(r, "before_id", "limit")
	if err != nil {
		h.readError(w, err)
		return
	}
	before, limit := int64(0), int64(20)
	if raw, ok := q["before_id"]; ok {
		before, err = positiveDecimal(raw)
		if err != nil {
			h.readError(w, err)
			return
		}
	}
	if raw, ok := q["limit"]; ok {
		limit, err = positiveDecimal(raw)
		if err != nil || limit > 100 {
			h.readError(w, points.ErrInvalid)
			return
		}
	}
	out, err := h.svc.History(r.Context(), uid, before, int(limit))
	if err != nil {
		h.readError(w, err)
		return
	}
	ok(w, out)
}

func (h *AITaskReadHandler) progress(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	if _, err := queryTaskRead(r); err != nil {
		h.readError(w, err)
		return
	}
	id, err := positiveDecimal(r.PathValue("id"))
	if err != nil {
		h.readError(w, err)
		return
	}
	out, err := h.svc.Progress(r.Context(), uid, id)
	if err != nil {
		h.readError(w, err)
		return
	}
	ok(w, out)
}

func queryTaskRead(r *http.Request, allowed ...string) (map[string]string, error) {
	// URL.Query 会静默忽略畸形转义，必须用 ParseQuery 显式拒绝。
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, points.ErrInvalid
	}
	out := make(map[string]string)
	for k, v := range q {
		found := false
		for _, a := range allowed {
			if k == a {
				found = true
			}
		}
		if !found || len(v) != 1 || v[0] == "" {
			return nil, points.ErrInvalid
		}
		out[k] = v[0]
	}
	return out, nil
}

func (h *AITaskReadHandler) readError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, points.ErrInvalid):
		fail(w, 400, codeBadRequest, "任务查询参数无效")
	case errors.Is(err, points.ErrNotFound):
		fail(w, 404, 40403, "任务不存在")
	default:
		fail(w, 503, 50302, "任务查询暂不可用") // 不记录驱动、URL、令牌或原始 Agent 异常。
	}
}
