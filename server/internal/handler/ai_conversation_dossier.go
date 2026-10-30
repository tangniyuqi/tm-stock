package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type aiConversationDossierService interface {
	Save(context.Context, string, int64, int64, int64, dossier.Selection) (dto.AIConversationDossier, error)
	Receipt(context.Context, string, int64, int64, dossier.Selection) (dto.AIConversationDossier, error)
	List(context.Context, string, int64, int64, int64) (dto.AIConversationDossierPage, error)
	Read(context.Context, string, int64, int64, int64) (dto.AIConversationDossierView, error)
}
type AIConversationDossierHandler struct{ svc aiConversationDossierService }

func NewAIConversationDossierHandler(svc aiConversationDossierService) *AIConversationDossierHandler {
	return &AIConversationDossierHandler{svc}
}
func (h *AIConversationDossierHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	for _, p := range []string{"POST /api/v1/ai/conversations/{id}/dossiers", "GET /api/v1/ai/conversations/{id}/dossiers", "GET /api/v1/ai/conversations/{id}/dossier-receipt", "GET /api/v1/ai/conversations/{id}/dossiers/{referenceId}"} {
		mux.Handle(p, auth(http.HandlerFunc(h.serve)))
	}
}
func conversationDossierFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, memberauth.ErrUnauthorized):
		fail(w, 401, 40100, "会员身份无效，请重新登录")
	case errors.Is(err, conversation.ErrNotFound), errors.Is(err, dossier.ErrNotFound):
		fail(w, 404, 40406, "会话或档案引用当前不可用")
	case errors.Is(err, dossier.ErrReferenceLimit):
		fail(w, 409, 40907, "本会话最多保存100条档案引用")
	case errors.Is(err, conversation.ErrConflict), errors.Is(err, dossier.ErrConflict):
		fail(w, 409, 40903, "会话已变化，请核对当前状态或原组合收据")
	default:
		fail(w, 503, 50311, "档案引用操作未确认，请核查原组合收据或重新读取")
	}
}
func (h *AIConversationDossierHandler) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")
	uid := middleware.MemberID(r.Context())
	headers := r.Header.Values("Authorization")
	if uid <= 0 || len(headers) != 1 {
		fail(w, 401, 40100, "需要会员身份")
		return
	}
	scheme, token, okToken := strings.Cut(headers[0], " ")
	if !okToken || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 8192 || strings.ContainsAny(token, " \t\r\n,") {
		fail(w, 401, 40100, "需要会员身份")
		return
	}
	cid, err := conversationDecimal(r.PathValue("id"), false)
	if err != nil || r.Method == http.MethodHead {
		fail(w, 400, 40001, "档案引用参数无效")
		return
	}
	if h == nil || h.svc == nil {
		conversationDossierFailure(w, dossier.ErrInvalidDraft)
		return
	}
	var out any
	switch {
	case r.Method == http.MethodPost:
		var body struct {
			ThemeID         int64 `json:"themeId"`
			Version         int64 `json:"version"`
			CompareVersion  int64 `json:"compareVersion"`
			ExpectedVersion int64 `json:"expectedVersion"`
		}
		err = taskSubmissionBody(w, r, &body, "themeId", "version", "compareVersion", "expectedVersion")
		selected := dossier.Selection{ThemeID: body.ThemeID, Version: body.Version, CompareVersion: body.CompareVersion}
		if err != nil || !selected.Valid() || !conversation.ID(body.ExpectedVersion) {
			fail(w, 400, 40001, "档案引用参数无效")
			return
		}
		out, err = h.svc.Save(r.Context(), token, uid, cid, body.ExpectedVersion, selected)
	case r.PathValue("referenceId") != "":
		id, e := conversationDecimal(r.PathValue("referenceId"), false)
		_, queryErr := conversationReadRequest(r)
		if e != nil || queryErr != nil {
			fail(w, 400, 40001, "档案引用参数无效")
			return
		}
		out, err = h.svc.Read(r.Context(), token, uid, cid, id)
	case strings.HasSuffix(r.URL.Path, "/dossier-receipt"):
		q, e := conversationReadRequest(r, "themeId", "version", "compareVersion")
		if e != nil || len(q) != 3 {
			fail(w, 400, 40001, "档案引用参数无效")
			return
		}
		theme, e1 := conversationDecimal(q["themeId"], false)
		version, e2 := conversationDecimal(q["version"], false)
		compare, e3 := conversationDecimal(q["compareVersion"], true)
		selected := dossier.Selection{ThemeID: theme, Version: version, CompareVersion: compare}
		if e1 != nil || e2 != nil || e3 != nil || !selected.Valid() {
			fail(w, 400, 40001, "档案引用参数无效")
			return
		}
		out, err = h.svc.Receipt(r.Context(), token, uid, cid, selected)
	default:
		q, e := conversationReadRequest(r, "beforeId")
		var before int64
		if raw, present := q["beforeId"]; present && e == nil {
			before, e = conversationDecimal(raw, false)
		}
		if e != nil {
			fail(w, 400, 40001, "档案引用参数无效")
			return
		}
		out, err = h.svc.List(r.Context(), token, uid, cid, before)
	}
	if err != nil {
		conversationDossierFailure(w, err)
		return
	}
	ok(w, out)
}
