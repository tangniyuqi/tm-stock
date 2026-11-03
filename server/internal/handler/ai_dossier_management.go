package handler

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type aiDossierManagementService interface {
	Inspect(context.Context, int64, dossier.ManagementObject) (dto.AIDossierManagementInspect, error)
	Apply(context.Context, int64, dossier.ManagementRequest) (dto.AIDossierManagementReceipt, error)
	Request(context.Context, int64, string) (dto.AIDossierManagementReceipt, error)
}
type AIDossierManagementHandler struct{ svc aiDossierManagementService }

func NewAIDossierManagementHandler(svc aiDossierManagementService) *AIDossierManagementHandler {
	return &AIDossierManagementHandler{svc}
}
func (h *AIDossierManagementHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/ai-dossier-management", h.inspect)
	mux.HandleFunc("POST /internal/v1/ai-dossier-management", h.apply)
	mux.HandleFunc("GET /internal/v1/ai-dossier-management/requests/{id}", h.request)
}
func (h *AIDossierManagementHandler) actor(w http.ResponseWriter, r *http.Request) (int64, bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	actor := middleware.AIAdminActor(r.Context())
	if actor <= 0 {
		fail(w, 401, codeUnauthorized, "需要内部管理身份")
		return 0, false
	}
	if _, present := r.Header["Origin"]; present {
		fail(w, 403, 40301, "内部管理入口不接受浏览器来源")
		return 0, false
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50310, "专题管理暂不可用")
		return 0, false
	}
	return actor, true
}
func (h *AIDossierManagementHandler) inspect(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.actor(w, r)
	if !allowed {
		return
	}
	if r.Method != http.MethodGet {
		h.failure(w, dossier.ErrInvalidManagement)
		return
	}
	q, err := conversationReadRequest(r, "kind", "userId", "themeId", "version")
	uid, e1 := conversationDecimal(q["userId"], true)
	theme, e2 := conversationDecimal(q["themeId"], false)
	version, e3 := conversationDecimal(q["version"], false)
	key := dossier.ManagementObject{Kind: q["kind"], UserID: uid, ThemeID: theme, Version: version}
	if err != nil || len(q) != 4 || e1 != nil || e2 != nil || e3 != nil || !key.Valid() {
		h.failure(w, dossier.ErrInvalidManagement)
		return
	}
	out, err := h.svc.Inspect(r.Context(), actor, key)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, out)
}
func (h *AIDossierManagementHandler) apply(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.actor(w, r)
	if !allowed {
		return
	}
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || len(r.Header.Values("Content-Type")) != 1 || media != "application/json" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8") || r.URL.RawQuery != "" || len(r.Header.Values("Content-Encoding")) != 0 || len(r.Header.Values("Idempotency-Key")) != 0 {
		h.failure(w, dossier.ErrInvalidManagement)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8193))
	if err != nil || len(raw) > 8192 {
		h.failure(w, dossier.ErrInvalidManagement)
		return
	}
	request, err := dossier.ParseManagementRequest(raw)
	if err != nil {
		h.failure(w, err)
		return
	}
	out, err := h.svc.Apply(r.Context(), actor, request)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, out)
}
func (h *AIDossierManagementHandler) request(w http.ResponseWriter, r *http.Request) {
	actor, allowed := h.actor(w, r)
	if !allowed {
		return
	}
	if r.Method != http.MethodGet {
		h.failure(w, dossier.ErrInvalidManagement)
		return
	}
	_, err := conversationReadRequest(r)
	id := r.PathValue("id")
	if err != nil || !dossier.ValidManagementRequestID(id) {
		h.failure(w, dossier.ErrInvalidManagement)
		return
	}
	out, err := h.svc.Request(r.Context(), actor, id)
	if err != nil {
		h.failure(w, err)
		return
	}
	ok(w, out)
}
func (h *AIDossierManagementHandler) failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, memberauth.ErrUnauthorized):
		fail(w, 403, 40301, "未获专题管理权限")
	case errors.Is(err, dossier.ErrInvalidManagement):
		fail(w, 400, 40001, "专题管理参数无效")
	case errors.Is(err, dossier.ErrNotFound):
		fail(w, 404, 40404, "专题版本、原请求或当前许可不可用")
	case errors.Is(err, dossier.ErrManagementConflict), errors.Is(err, dossier.ErrConflict):
		fail(w, 409, 40901, "专题修订或原请求已变化，请重新核对")
	default:
		fail(w, 503, 50310, "专题管理结果未确认，请保留原请求编号核对")
	}
}
