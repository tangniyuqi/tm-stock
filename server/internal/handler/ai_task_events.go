package handler

import (
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func (h *AITaskReadHandler) events(w http.ResponseWriter, r *http.Request) {
	uid := middleware.MemberID(r.Context())
	if uid <= 0 {
		fail(w, 401, codeUnauthorized, "需要会员身份")
		return
	}
	id, err := positiveDecimal(r.PathValue("id"))
	if err != nil {
		h.readError(w, err)
		return
	}
	q, err := queryTaskRead(r, "after_seq", "limit")
	if err != nil {
		h.readError(w, err)
		return
	}
	after, limit := int64(0), int64(20)
	if raw, ok := q["after_seq"]; ok && raw != "0" {
		after, err = positiveDecimal(raw)
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
	if h == nil || h.svc == nil {
		h.readError(w, points.ErrInvariant)
		return
	}
	out, err := h.svc.Events(r.Context(), uid, id, after, int(limit))
	if err != nil {
		h.readError(w, err)
		return
	}
	ok(w, out)
}
