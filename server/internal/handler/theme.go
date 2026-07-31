package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// ThemeQueryService 是 handler 对 service 的依赖抽象。
// 按 Go 惯例定义在【消费方】，这样 handler 可以用假实现做单测，不需要数据库。
type ThemeQueryService interface {
	GetDetail(ctx context.Context, themeID int64, access service.Access) (*dto.ThemeDetailResp, error)
	GetEvidence(ctx context.Context, themeID int64, tsCode string, access service.Access) (*dto.EvidenceResp, error)
}

// AccessResolver 判定当前请求能否看付费内容。
//
// 🔴 会员与试吃能力尚未实现（member-center 未开发）。
// 在它就位之前，唯一可接受的默认行为是【拒绝】——
// 付费内容在会员系统上线前就裸奔，比接口不可用严重得多。
type AccessResolver interface {
	Resolve(r *http.Request) service.Access
}

// DenyAllAccess 默认实现：一律不给付费内容（fail-closed）。
type DenyAllAccess struct{}

func (DenyAllAccess) Resolve(*http.Request) service.Access {
	return service.Access{CanViewPaid: false, TrialLeft: 0}
}

// DevGrantAllAccess 仅供本地开发，无条件放行。
// 构造时会打一条醒目日志——这种开关最怕的就是"忘了关"。
type DevGrantAllAccess struct{}

func NewDevGrantAllAccess() DevGrantAllAccess {
	log.Println("⚠️  [安全] TM_DEV_GRANT_PAID=true：付费内容对所有请求放行。仅限本地开发，禁止用于任何对外环境。")
	return DevGrantAllAccess{}
}

func (DevGrantAllAccess) Resolve(*http.Request) service.Access {
	return service.Access{CanViewPaid: true, TrialLeft: 0}
}

// ThemeHandler 题材查询相关端点。
type ThemeHandler struct {
	svc    ThemeQueryService
	access AccessResolver
}

// NewThemeHandler 构造。
func NewThemeHandler(svc ThemeQueryService, access AccessResolver) *ThemeHandler {
	return &ThemeHandler{svc: svc, access: access}
}

// Register 注册路由。用 Go 1.22+ 的方法+通配模式。
func (h *ThemeHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/theme/{id}", h.Detail)
	mux.HandleFunc("GET /api/v1/theme/{id}/stock/{tsCode}/evidence", h.Evidence)
}

// Detail GET /api/v1/theme/{id}
func (h *ThemeHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, ok2 := parseID(r.PathValue("id"))
	if !ok2 {
		fail(w, http.StatusBadRequest, codeBadRequest, "题材 id 非法")
		return
	}

	resp, err := h.svc.GetDetail(r.Context(), id, h.access.Resolve(r))
	if err != nil {
		h.mapError(w, "ThemeDetail", err)
		return
	}
	ok(w, resp)
}

// Evidence GET /api/v1/theme/{id}/stock/{tsCode}/evidence
//
// 归属依据是本产品的差异化核心，也是付费内容——
// 权限判定与详情页完全一致，否则可以绕过付费墙直接拿最值钱的部分。
func (h *ThemeHandler) Evidence(w http.ResponseWriter, r *http.Request) {
	id, ok2 := parseID(r.PathValue("id"))
	if !ok2 {
		fail(w, http.StatusBadRequest, codeBadRequest, "题材 id 非法")
		return
	}
	tsCode := strings.TrimSpace(r.PathValue("tsCode"))
	if !validTsCode(tsCode) {
		fail(w, http.StatusBadRequest, codeBadRequest, "股票代码非法")
		return
	}

	resp, err := h.svc.GetEvidence(r.Context(), id, tsCode, h.access.Resolve(r))
	if err != nil {
		h.mapError(w, "ThemeEvidence", err)
		return
	}
	ok(w, resp)
}

// mapError 把领域错误翻译成 HTTP 状态与业务码。
//
// ErrAccessDenied 与 ErrEvidenceNotFound 必须分开：
// 用户点个股想看依据时应引导订阅（403），而不是被告知"没有依据"——
// 后者会让人以为数据缺失，是错误的信息。
func (h *ThemeHandler) mapError(w http.ResponseWriter, where string, err error) {
	switch {
	case errors.Is(err, service.ErrThemeNotFound):
		fail(w, http.StatusNotFound, codeThemeNotFound, "题材不存在")
	case errors.Is(err, service.ErrEvidenceNotFound):
		fail(w, http.StatusNotFound, codeEvidenceNoFound, "该归属暂无可查依据")
	case errors.Is(err, service.ErrAccessDenied):
		fail(w, http.StatusForbidden, codeAccessDenied, "订阅后可查看完整内容")
	default:
		failInternal(w, where, err)
	}
}

func parseID(s string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// validTsCode 校验 000001.SZ 这种形式。
//
// 值最终是走参数绑定进 SQL 的，不存在注入；这里拦的是明显畸形的输入，
// 让它在入口就被挡掉，而不是穿到数据层再返回一个语义模糊的 404。
func validTsCode(s string) bool {
	if len(s) < 6 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isUpper := c >= 'A' && c <= 'Z'
		if !isDigit && !isUpper && c != '.' {
			return false
		}
	}
	return strings.Contains(s, ".")
}
