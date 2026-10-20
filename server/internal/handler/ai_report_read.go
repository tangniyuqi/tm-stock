package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type aiReportReader interface {
	Read(context.Context, int64, int64) (dto.AIReport, error)
}
type AIReportReadHandler struct{ svc aiReportReader }

func NewAIReportReadHandler(svc aiReportReader) *AIReportReadHandler {
	return &AIReportReadHandler{svc}
}
func (h *AIReportReadHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/tasks/{id}/report", auth(http.HandlerFunc(h.read)))
}
func (h *AIReportReadHandler) read(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	id, err := positiveDecimal(r.PathValue("id"))
	if err != nil || r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		fail(w, 400, codeBadRequest, "报告查询参数无效")
		return
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50304, "报告查询暂不可用")
		return
	}
	out, err := h.svc.Read(r.Context(), uid, id)
	if err != nil {
		switch {
		case errors.Is(err, points.ErrNotFound):
			fail(w, 404, 40403, "任务不存在")
		case errors.Is(err, finalreport.ErrUnavailable):
			fail(w, 422, 42202, "报告暂不可读取")
		default:
			fail(w, 503, 50304, "报告查询暂不可用")
		}
		return
	}
	ok(w, out)
}
