package handler

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ThemeDailyQueryService interface {
	List(context.Context, time.Time, string, int, int) (*dto.ThemeDailyPageResp, error)
}
type ThemeDailyHandler struct{ s ThemeDailyQueryService }

func NewThemeDailyHandler(s ThemeDailyQueryService) *ThemeDailyHandler {
	return &ThemeDailyHandler{s: s}
}
func (h *ThemeDailyHandler) Register(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/theme-daily/list", h.List)
}
func (h *ThemeDailyHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sortKey := q.Get("sort")
	if sortKey == "" {
		sortKey = "publishAt"
	}
	if sortKey != "changePct" && sortKey != "publishAt" {
		fail(w, 400, codeBadRequest, "sort 参数非法")
		return
	}
	date := time.Now()
	if v := strings.TrimSpace(q.Get("date")); v != "" {
		d, e := time.Parse("2006-01-02", v)
		if e != nil {
			fail(w, 400, codeBadRequest, "date 参数非法")
			return
		}
		date = d
	}
	page, size := 1, 20
	var e error
	if v := q.Get("page"); v != "" {
		page, e = strconv.Atoi(v)
	}
	if e != nil || page < 1 {
		fail(w, 400, codeBadRequest, "page 参数非法")
		return
	}
	if v := q.Get("size"); v != "" {
		size, e = strconv.Atoi(v)
	}
	if e != nil || size < 1 || size > 100 {
		fail(w, 400, codeBadRequest, "size 参数非法")
		return
	}
	out, e := h.s.List(r.Context(), date, sortKey, page, size)
	if e != nil {
		failInternal(w, "ThemeDaily", e)
		return
	}
	ok(w, out)
}
