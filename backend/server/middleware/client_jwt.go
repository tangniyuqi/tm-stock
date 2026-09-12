package middleware

import (
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// clientToken C 端取令牌：优先标准 Authorization: Bearer <token>，再兜底 x-token/cookie。
// 仅在 C 端中间件局部实现，不改动主系统共用的 utils.GetToken。
func clientToken(c *gin.Context) string {
	if auth := c.Request.Header.Get("Authorization"); auth != "" && strings.HasPrefix(auth, "Bearer ") {
		if t := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")); t != "" {
			return t
		}
	}
	return utils.GetToken(c)
}

// ClientJWTAuth C 端（member）登录鉴权中间件。
//
// 登录令牌为 member 维度 JWT（与主系统共用签名与 claims 结构），签发时
// 需将 claims.BaseClaims.ID 置为 CommonMember.ID（由 /client/auth 登录功能负责签发）。
// 校验通过后把 claims 写入上下文，业务层可用 utils.GetUserID(c) 取得当前会员 ID。
func ClientJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := clientToken(c)
		if token == "" {
			response.NoAuth("未登录或非法访问，请登录", c)
			c.Abort()
			return
		}
		claims, err := utils.NewJWT().ParseToken(token)
		if err != nil {
			response.NoAuth("登录已过期或令牌无效，请重新登录", c)
			c.Abort()
			return
		}
		// 校验对应当前会员是否存在且状态正常
		var m member.Member
		if err := global.GVA_DB.Where("id = ?", claims.BaseClaims.ID).First(&m).Error; err != nil {
			// 临时诊断日志: 确认 token 中会员 ID 与查询失败原因
			global.GVA_LOG.Error("C端鉴权: 按ID查询会员失败",
				zap.Uint("member_id", claims.BaseClaims.ID),
				zap.Error(err))
			response.NoAuth("账号不存在或已被删除", c)
			c.Abort()
			return
		}
		if m.Status != 1 {
			response.NoAuth("账号已被禁用", c)
			c.Abort()
			return
		}
		c.Set("claims", claims)
		c.Set("member_id", m.ID)
		c.Next()
	}
}
