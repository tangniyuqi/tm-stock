package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiTaskCanceller interface {
	Cancel(context.Context, int64, int64) (dto.AITaskCancellation, error)
}
type AITaskCancelHandler struct{ svc aiTaskCanceller }

func NewAITaskCancelHandler(svc aiTaskCanceller) *AITaskCancelHandler {
	return &AITaskCancelHandler{svc: svc}
}

func (h *AITaskCancelHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/ai/tasks/{id}/cancel", auth(http.HandlerFunc(h.cancel)))
}

func cancellationBody(w http.ResponseWriter, r *http.Request) error {
	if r.URL.RawQuery != "" || r.URL.ForceQuery || len(r.Header.Values("Content-Encoding")) != 0 {
		return points.ErrInvalid
	}
	types := r.Header.Values("Content-Type")
	if len(types) > 1 {
		return points.ErrInvalid
	}
	if len(types) == 1 {
		media, params, err := mime.ParseMediaType(types[0])
		if err != nil || media != "application/json" || len(params) > 1 {
			return points.ErrInvalid
		}
		for k, v := range params {
			if k != "charset" || !strings.EqualFold(v, "utf-8") {
				return points.ErrInvalid
			}
		}
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64))
	if err != nil {
		return points.ErrInvalid
	}
	if len(raw) == 0 {
		return nil
	}
	if len(types) != 1 {
		return points.ErrInvalid
	}
	raw = bytes.Trim(raw, " \t\r\n")
	var fields map[string]json.RawMessage
	if len(raw) < 2 || raw[0] != '{' || json.Unmarshal(raw, &fields) != nil || len(fields) != 0 {
		return points.ErrInvalid
	}
	return nil
}

func (h *AITaskCancelHandler) cancel(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	id, err := positiveDecimal(r.PathValue("id"))
	if err == nil {
		err = cancellationBody(w, r)
	}
	if err != nil {
		fail(w, 400, codeBadRequest, "任务取消参数无效")
		return
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50303, "任务取消暂不可用，请查询原任务状态")
		return
	}
	out, err := h.svc.Cancel(r.Context(), uid, id)
	if err != nil {
		if errors.Is(err, points.ErrNotFound) {
			fail(w, 404, 40403, "任务不存在")
		} else {
			// 不记录驱动详情、请求体或令牌；提交未知不自动重试。
			fail(w, 503, 50303, "任务取消暂不可用，请查询原任务状态")
		}
		return
	}
	ok(w, out)
}
