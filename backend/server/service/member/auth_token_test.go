package member

import (
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
)

// 会员令牌必须带 UserType=client：后台与 C 端共用签名密钥，中间件（ClientJWTAuth、JWTAuth）
// 只靠这个字段区分令牌主体。缺了它，会员令牌既进不了 C 端，也无法与后台令牌区分。
func TestIssueTokenCarriesClientUserType(t *testing.T) {
	old := global.GVA_CONFIG.JWT
	global.GVA_CONFIG.JWT = config.JWT{SigningKey: "test-signing-key", ExpiresTime: "7d", BufferTime: "1d", Issuer: "GVA"}
	t.Cleanup(func() { global.GVA_CONFIG.JWT = old })

	resp, err := (&AuthService{}).issueToken(42)
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	claims, err := utils.NewJWT().ParseToken(resp.Token)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if claims.UserType != system.UserTypeClient {
		t.Errorf("会员令牌的 UserType = %q，期望 %q", claims.UserType, system.UserTypeClient)
	}
	if claims.BaseClaims.ID != 42 {
		t.Errorf("claims.BaseClaims.ID = %d，期望会员 ID 42", claims.BaseClaims.ID)
	}
	if claims.AuthorityId != 0 {
		t.Errorf("会员令牌不应带后台角色，AuthorityId = %d", claims.AuthorityId)
	}
}
