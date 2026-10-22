package quant

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/service/feedbackconsole"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

type FeedbackConsoleAPI struct{ client *feedbackconsole.Client }

func NewFeedbackConsoleAPI(c *feedbackconsole.Client) *FeedbackConsoleAPI {
	return &FeedbackConsoleAPI{c}
}
func (a *FeedbackConsoleAPI) Register(private *gin.RouterGroup) {
	private.GET("/quant/ai-feedback", a.serve)
	private.GET("/quant/ai-feedback/:id", a.serve)
	private.POST("/quant/ai-feedback/:id/decisions", a.serve)
}
func feedbackResponse(c *gin.Context, status, code int, data any) {
	msg := ""
	switch code {
	case 40100:
		msg = "需要后台处理身份"
	case 40301:
		msg = "当前账号或内部处理凭据未获准"
	case 40001:
		msg = "反馈参数无效"
	case 40403:
		msg = "反馈不存在"
	case 40904:
		msg = "反馈已变化，请重新查询"
	case 50306:
		msg = "操作未确认，请查询原受理单"
	}
	c.JSON(status, gin.H{"code": code, "msg": msg, "data": data})
}
func feedbackFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, feedbackconsole.ErrInvalid):
		feedbackResponse(c, 400, 40001, nil)
	case errors.Is(err, feedbackconsole.ErrForbidden):
		feedbackResponse(c, 403, 40301, nil)
	case errors.Is(err, feedbackconsole.ErrNotFound):
		feedbackResponse(c, 404, 40403, nil)
	case errors.Is(err, feedbackconsole.ErrConflict):
		feedbackResponse(c, 409, 40904, nil)
	default:
		feedbackResponse(c, 503, 50306, nil)
	}
}
func feedbackID(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || !feedbackconsole.ID(n) || strconv.FormatInt(n, 10) != s {
		return 0, feedbackconsole.ErrInvalid
	}
	return n, nil
}

// 反馈接口在原JWT/Casbin之后再核对当前进程策略，不复用具体URL的判定缓存。
// 参数化/筛选删策略可能未清理该缓存；只加固本功能，不调整全站缓存设置。
func feedbackCurrentPolicy(c *gin.Context, authority uint) bool {
	enforcer := utils.GetCasbin()
	if enforcer == nil || enforcer.SyncedEnforcer == nil {
		return false
	}
	path := strings.TrimPrefix(c.Request.URL.Path, global.GVA_CONFIG.System.RouterPrefix)
	allowed, err := enforcer.SyncedEnforcer.Enforce(strconv.FormatUint(uint64(authority), 10), path, c.Request.Method)
	return err == nil && allowed
}

func (a *FeedbackConsoleAPI) serve(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Vary", "x-token")
	c.Header("X-Content-Type-Options", "nosniff")
	claims, exists := c.Get("claims")
	identity, ok := claims.(*systemReq.CustomClaims)
	if !exists || !ok || identity == nil || identity.UserType != "admin" || identity.BaseClaims.ID == 0 {
		feedbackResponse(c, 401, 40100, nil)
		return
	}
	if a == nil || a.client == nil {
		feedbackFailure(c, feedbackconsole.ErrUnconfirmed)
		return
	}
	if !feedbackCurrentPolicy(c, identity.AuthorityId) {
		// 保持原GVA拒绝契约，页面局部归一化；拒绝发生在任何上游调用前。
		response.FailWithDetailed(gin.H{}, "权限不足", c)
		c.Abort()
		return
	}
	if len(c.Request.Header.Values("Origin")) > 1 || c.GetHeader("Origin") != "" && c.GetHeader("Origin") != a.client.WebOrigin() {
		feedbackFailure(c, feedbackconsole.ErrForbidden)
		return
	}
	if len(c.Request.Header.Values("Content-Encoding")) != 0 || c.Request.URL.ForceQuery {
		feedbackFailure(c, feedbackconsole.ErrInvalid)
		return
	}
	uid := int64(identity.BaseClaims.ID)
	rawID := c.Param("id")
	var id int64
	var err error
	if rawID != "" {
		id, err = feedbackID(rawID)
		if err != nil {
			feedbackFailure(c, err)
			return
		}
	}
	if c.Request.Method == "POST" {
		typ, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || typ != "application/json" || len(params) > 1 || len(params) == 1 && params["charset"] != "utf-8" || c.Request.URL.RawQuery != "" {
			feedbackFailure(c, feedbackconsole.ErrInvalid)
			return
		}
		b, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 2048))
		if err != nil {
			feedbackFailure(c, feedbackconsole.ErrInvalid)
			return
		}
		d, err := feedbackconsole.DecodeDecision(b)
		if err != nil {
			feedbackFailure(c, err)
			return
		}
		out, err := a.client.Decide(c.Request.Context(), uid, id, d)
		if err != nil {
			feedbackFailure(c, err)
			return
		}
		feedbackResponse(c, 200, 0, out)
		return
	}
	b, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	if err != nil || len(b) != 0 {
		feedbackFailure(c, feedbackconsole.ErrInvalid)
		return
	}
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		feedbackFailure(c, feedbackconsole.ErrInvalid)
		return
	}
	if rawID != "" {
		if len(q) != 0 {
			feedbackFailure(c, feedbackconsole.ErrInvalid)
			return
		}
		out, err := a.client.Detail(c.Request.Context(), uid, id)
		if err != nil {
			feedbackFailure(c, err)
			return
		}
		feedbackResponse(c, 200, 0, out)
		return
	}
	after, limit := int64(0), int64(20)
	for key, values := range q {
		if len(values) != 1 || values[0] == "" || key != "after_id" && key != "limit" {
			feedbackFailure(c, feedbackconsole.ErrInvalid)
			return
		}
		v, err := feedbackID(values[0])
		if err != nil {
			feedbackFailure(c, err)
			return
		}
		if key == "after_id" {
			after = v
		} else {
			limit = v
		}
	}
	if limit > 100 {
		feedbackFailure(c, feedbackconsole.ErrInvalid)
		return
	}
	out, err := a.client.List(c.Request.Context(), uid, after, int(limit))
	if err != nil {
		feedbackFailure(c, err)
		return
	}
	feedbackResponse(c, 200, 0, out)
}
