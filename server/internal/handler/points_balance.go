package handler

import (
	"context"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type pointsBalanceReader interface {
	Balance(context.Context, int64) (service.PointsBalance, error)
}
type PointsBalanceHandler struct{ svc pointsBalanceReader }

func NewPointsBalanceHandler(svc pointsBalanceReader) *PointsBalanceHandler {
	return &PointsBalanceHandler{svc: svc}
}
func (h *PointsBalanceHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/points/balance", auth(http.HandlerFunc(h.balance)))
}
func (h *PointsBalanceHandler) balance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, 40100, "需要会员身份")
		return
	}
	if r.Method != http.MethodGet || r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) != 0 || len(r.Header.Values("Content-Encoding")) != 0 {
		fail(w, 400, 40001, "余额查询参数无效")
		return
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50302, "积分余额暂不可用")
		return
	}
	value, err := h.svc.Balance(r.Context(), uid)
	if err != nil {
		fail(w, 503, 50302, "积分余额暂不可用")
		return
	}
	ok(w, struct {
		Available int64 `json:"availablePoints"`
		Frozen    int64 `json:"frozenPoints"`
		CheckedAt int64 `json:"checkedAt"`
	}{value.Available, value.Frozen, value.CheckedAt})
}
