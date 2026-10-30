package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type aiDossierReader interface {
	ViewVersions(context.Context, string, int64, int64, []int64) (dto.AIDossierView, error)
	ListVersions(context.Context, string, int64, int64, int64) (dto.AIDossierVersionList, error)
}
type AIDossierHandler struct{ reader aiDossierReader }

func NewAIDossierHandler(reader aiDossierReader) *AIDossierHandler { return &AIDossierHandler{reader} }
func (h *AIDossierHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/themes/{id}/dossiers", auth(http.HandlerFunc(h.read)))
	mux.Handle("GET /api/v1/ai/themes/{id}/dossiers/{version}", auth(http.HandlerFunc(h.read)))
}
func (h *AIDossierHandler) read(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	headers := r.Header.Values("Authorization")
	if uid <= 0 || len(headers) != 1 {
		fail(w, 401, 40100, "会员身份无效，请重新登录")
		return
	}
	scheme, token, okToken := strings.Cut(headers[0], " ")
	if !okToken || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 8192 || strings.ContainsAny(token, " \t\r\n,") {
		fail(w, 401, 40100, "会员身份无效，请重新登录")
		return
	}
	theme, e1 := conversationDecimal(r.PathValue("id"), false)
	if r.PathValue("version") == "" {
		query, err := conversationReadRequest(r, "beforeVersion")
		var before int64
		if raw, exists := query["beforeVersion"]; exists {
			before, err = conversationDecimal(raw, false)
		}
		if r.Method != http.MethodGet || e1 != nil || err != nil {
			fail(w, 400, 40001, "专题读取参数无效")
			return
		}
		if h == nil || h.reader == nil {
			fail(w, 503, 50310, "专题资料暂不可用")
			return
		}
		out, err := h.reader.ListVersions(r.Context(), token, uid, theme, before)
		if err != nil {
			dossierFailure(w, err)
			return
		}
		ok(w, out)
		return
	}
	version, e2 := conversationDecimal(r.PathValue("version"), false)
	query, e3 := conversationReadRequest(r, "compareVersion")
	if r.Method != http.MethodGet || e1 != nil || e2 != nil || e3 != nil {
		fail(w, 400, 40001, "专题读取参数无效")
		return
	}
	versions := []int64{version}
	if raw, exists := query["compareVersion"]; exists {
		other, err := conversationDecimal(raw, false)
		if err != nil || other <= version {
			fail(w, 400, 40001, "专题读取参数无效")
			return
		}
		versions = append(versions, other)
	}
	if h == nil || h.reader == nil {
		fail(w, 503, 50310, "专题资料暂不可用")
		return
	}
	out, err := h.reader.ViewVersions(r.Context(), token, uid, theme, versions)
	if err != nil {
		dossierFailure(w, err)
		return
	}
	ok(w, out)
}
func dossierFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, memberauth.ErrUnauthorized):
		fail(w, 401, 40100, "会员身份无效，请重新登录")
	case errors.Is(err, dossier.ErrNotFound):
		fail(w, 404, 40404, "专题版本不可用")
	default:
		fail(w, 503, 50310, "专题资料暂不可用")
	}
}
