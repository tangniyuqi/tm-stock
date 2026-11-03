package handler

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type aiDossierCorrectionService interface {
	Inspect(context.Context, int64, dossier.CorrectionPair) (dto.AIDossierCorrectionInspect, error)
	Apply(context.Context, int64, dossier.CorrectionRequest) (dto.AIDossierCorrectionReceipt, error)
	Request(context.Context, int64, string) (dto.AIDossierCorrectionReceipt, error)
	Read(context.Context, string, int64, dossier.CorrectionPair) (dto.AIDossierCorrections, error)
}
type AIDossierCorrectionHandler struct{ svc aiDossierCorrectionService }

func NewAIDossierCorrectionHandler(svc aiDossierCorrectionService) *AIDossierCorrectionHandler {
	return &AIDossierCorrectionHandler{svc}
}
func (h *AIDossierCorrectionHandler) RegisterInternal(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/ai-dossier-corrections", h.internal)
	mux.HandleFunc("POST /internal/v1/ai-dossier-corrections", h.internal)
	mux.HandleFunc("GET /internal/v1/ai-dossier-corrections/requests/{id}", h.internal)
}
func (h *AIDossierCorrectionHandler) RegisterPublic(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ai/themes/{id}/dossier-corrections", auth(http.HandlerFunc(h.read)))
}
func (h *AIDossierCorrectionHandler) internal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	failure := func(err error) { (&AIDossierManagementHandler{}).failure(w, err) }
	actor := middleware.AIAdminActor(r.Context())
	if actor <= 0 {
		fail(w, 401, 40100, "需要内部管理身份")
		return
	}
	if _, present := r.Header["Origin"]; present {
		fail(w, 403, 40301, "内部管理入口不接受浏览器来源")
		return
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50310, "更正声明管理暂不可用")
		return
	}
	if r.Method == http.MethodPost {
		media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || len(r.Header.Values("Content-Type")) != 1 || media != "application/json" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8") || r.URL.RawQuery != "" || len(r.Header.Values("Content-Encoding")) != 0 || len(r.Header.Values("Idempotency-Key")) != 0 {
			failure(dossier.ErrInvalidManagement)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, dossier.MaxCorrectionBytes+1))
		if err != nil {
			failure(dossier.ErrInvalidManagement)
			return
		}
		request, err := dossier.ParseCorrectionRequest(raw)
		if err != nil {
			failure(err)
			return
		}
		out, err := h.svc.Apply(r.Context(), actor, request)
		if err != nil {
			failure(err)
			return
		}
		ok(w, out)
		return
	}
	if r.Method != http.MethodGet {
		failure(dossier.ErrInvalidManagement)
		return
	}
	if id := r.PathValue("id"); id != "" {
		if _, err := conversationReadRequest(r); err != nil || !dossier.ValidManagementRequestID(id) {
			failure(dossier.ErrInvalidManagement)
			return
		}
		out, err := h.svc.Request(r.Context(), actor, id)
		if err != nil {
			failure(err)
			return
		}
		ok(w, out)
		return
	}
	q, err := conversationReadRequest(r, "themeId", "beforeVersion", "afterVersion")
	p, e := correctionPair(q["themeId"], q["beforeVersion"], q["afterVersion"])
	if err != nil || e != nil || len(q) != 3 {
		failure(dossier.ErrInvalidManagement)
		return
	}
	out, err := h.svc.Inspect(r.Context(), actor, p)
	if err != nil {
		failure(err)
		return
	}
	ok(w, out)
}
func correctionPair(theme, before, after string) (dossier.CorrectionPair, error) {
	t, e1 := conversationDecimal(theme, false)
	b, e2 := conversationDecimal(before, false)
	a, e3 := conversationDecimal(after, false)
	p := dossier.CorrectionPair{ThemeID: t, BeforeVersion: b, AfterVersion: a}
	if e1 != nil || e2 != nil || e3 != nil || !p.Valid() {
		return dossier.CorrectionPair{}, dossier.ErrInvalidManagement
	}
	return p, nil
}
func (h *AIDossierCorrectionHandler) read(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	headers := r.Header.Values("Authorization")
	if uid <= 0 || len(headers) != 1 {
		fail(w, 401, 40100, "会员身份无效，请重新登录")
		return
	}
	scheme, token, valid := strings.Cut(headers[0], " ")
	if !valid || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 8192 || strings.ContainsAny(token, " \t\r\n,") {
		fail(w, 401, 40100, "会员身份无效，请重新登录")
		return
	}
	q, err := conversationReadRequest(r, "beforeVersion", "afterVersion")
	p, e := correctionPair(r.PathValue("id"), q["beforeVersion"], q["afterVersion"])
	if r.Method != http.MethodGet || err != nil || e != nil || len(q) != 2 {
		fail(w, 400, 40001, "更正声明查询参数无效")
		return
	}
	if h == nil || h.svc == nil {
		fail(w, 503, 50310, "更正声明暂不可用")
		return
	}
	out, err := h.svc.Read(r.Context(), token, uid, p)
	if err != nil {
		dossierFailure(w, err)
		return
	}
	ok(w, out)
}
