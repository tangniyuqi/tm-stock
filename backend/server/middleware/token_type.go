package middleware

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

// requireTokenType 校验令牌里的主体类型（admin 或 client），不符就 401 并中止请求。
//
// 后台与 C 端共用同一把签名密钥和同一套 claims 结构，唯一的区分就是 UserType：
//   - 不校验的话，会员令牌（claims.ID 是会员 ID）能拿去撞后台接口里同号的后台账号，
//     后台令牌（claims.ID 是后台账号 ID）也能拿去冒充 C 端里同号的会员；
//   - 采用"只认白名单"的写法：类型为空（旧令牌或手工拼出的令牌）一律拒绝，不做兼容放行，
//     否则这条兼容分支会永远是一个绕过口。代价是部署后已登录的会员需重新登录一次。
//
// 日志只记录令牌类型和路径，不记录令牌本身。
func requireTokenType(c *gin.Context, claims *systemReq.CustomClaims, want system.UserType) bool {
	if claims != nil && claims.UserType == want {
		return true
	}
	got := system.UserType("")
	if claims != nil {
		got = claims.UserType
	}
	logger.WithCtx(c.Request.Context()).Mod("auth").
		Field("want", string(want)).Field("got", string(got)).Field("path", c.FullPath()).
		Warn("令牌类型不匹配，已拒绝")
	response.NoAuth("令牌类型不匹配，请使用对应入口重新登录", c)
	c.Abort()
	return false
}
