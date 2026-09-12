package member

import (
	"context"
	"errors"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	sysReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"gorm.io/gorm"
)

type AuthService struct{}

// Register 用户注册（手机号 + 短信验证码 + 密码）。
// 短信验证码真实校验需与短信服务联调，当前仅要求非空。
func (s *AuthService) Register(ctx context.Context, req member.RegisterReq) (member.LoginResp, error) {
	if !mobileRegexp.MatchString(req.Mobile) {
		return member.LoginResp{}, errors.New("手机号格式不正确")
	}
	if err := smsService.VerifyCode(ctx, req.Mobile, member.SmsBizRegister, req.Code); err != nil {
		return member.LoginResp{}, err
	}
	if len(req.Password) < 6 {
		return member.LoginResp{}, errors.New("密码不能少于 6 位")
	}

	// 检查手机号是否已注册
	var existCount int64
	if err := global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("mobile = ?", req.Mobile).Count(&existCount).Error; err != nil {
		return member.LoginResp{}, err
	}
	if existCount > 0 {
		return member.LoginResp{}, errors.New("该手机号已注册")
	}

	m := member.Member{
		Mobile:   req.Mobile,
		Password: utils.BcryptHash(req.Password),
		Nickname: "题材助手",
		Status:   1,
	}
	if err := global.GVA_DB.WithContext(ctx).Create(&m).Error; err != nil {
		return member.LoginResp{}, err
	}

	// 注册时初始化一条资产记录（积分等资产改由 addon_member_asset 承载）
	asset := member.MemberAsset{MemberId: m.ID, Status: 1}
	if err := global.GVA_DB.WithContext(ctx).Create(&asset).Error; err != nil {
		return member.LoginResp{}, err
	}

	// 注册后直接签发 token
	return s.issueToken(m.ID)
}

// LoginByPassword 密码登录（支持手机号或用户名）
func (s *AuthService) LoginByPassword(ctx context.Context, req member.LoginReq) (member.LoginResp, error) {
	if req.Mobile == "" {
		return member.LoginResp{}, errors.New("账号不能为空")
	}
	if req.Password == "" {
		return member.LoginResp{}, errors.New("密码不能为空")
	}

	var m member.Member
	// 账号可为手机号（mobile）或用户名（name）
	if err := global.GVA_DB.WithContext(ctx).Where("mobile = ? OR name = ?", req.Mobile, req.Mobile).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return member.LoginResp{}, errors.New("账号未注册")
		}
		return member.LoginResp{}, err
	}
	if m.Status != 1 {
		return member.LoginResp{}, errors.New("账号已被禁用")
	}
	if !utils.BcryptCheck(req.Password, m.Password) {
		return member.LoginResp{}, errors.New("密码错误")
	}

	// 更新最后登录时间
	now := time.Now()
	global.GVA_DB.WithContext(ctx).Model(&m).Updates(map[string]interface{}{
		"last_login_at": now,
	})

	return s.issueToken(m.ID)
}

// LoginByCode 短信验证码登录。
// 真实短信校验需与短信服务联调，当前仅要求验证码非空。
func (s *AuthService) LoginByCode(ctx context.Context, req member.LoginReq) (member.LoginResp, error) {
	if !mobileRegexp.MatchString(req.Mobile) {
		return member.LoginResp{}, errors.New("手机号格式不正确")
	}
	if err := smsService.VerifyCode(ctx, req.Mobile, member.SmsBizLogin, req.Code); err != nil {
		return member.LoginResp{}, err
	}

	var m member.Member
	if err := global.GVA_DB.WithContext(ctx).Where("mobile = ?", req.Mobile).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return member.LoginResp{}, errors.New("手机号未注册，请先注册")
		}
		return member.LoginResp{}, err
	}
	if m.Status != 1 {
		return member.LoginResp{}, errors.New("账号已被禁用")
	}

	now := time.Now()
	global.GVA_DB.WithContext(ctx).Model(&m).Updates(map[string]interface{}{
		"last_login_at": now,
	})

	return s.issueToken(m.ID)
}

// ForgotPwd 找回密码：校验手机号已注册后下发短信验证码（记入 addon_sms_log）。
func (s *AuthService) ForgotPwd(ctx context.Context, req member.ForgotPwdReq, ip string) error {
	if !mobileRegexp.MatchString(req.Mobile) {
		return errors.New("手机号格式不正确")
	}

	var m member.Member
	if err := global.GVA_DB.WithContext(ctx).Where("mobile = ?", req.Mobile).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("该手机号未注册")
		}
		return err
	}
	if m.Status != 1 {
		return errors.New("账号已被禁用")
	}

	return smsService.SendCode(ctx, req.Mobile, member.SmsBizForgot, ip)
}

// ResetPwd 重置密码（短信验证码校验后设置新密码）。
// 真实短信校验需与短信服务联调，当前仅要求验证码非空且密码合法。
func (s *AuthService) ResetPwd(ctx context.Context, req member.ResetPwdReq) error {
	if !mobileRegexp.MatchString(req.Mobile) {
		return errors.New("手机号格式不正确")
	}
	if err := smsService.VerifyCode(ctx, req.Mobile, member.SmsBizForgot, req.Code); err != nil {
		return err
	}
	if len(req.Password) < 6 {
		return errors.New("密码不能少于 6 位")
	}

	var m member.Member
	if err := global.GVA_DB.WithContext(ctx).Where("mobile = ?", req.Mobile).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("该手机号未注册")
		}
		return err
	}

	return global.GVA_DB.WithContext(ctx).Model(&m).
		Update("password", utils.BcryptHash(req.Password)).Error
}

// Logout 登出：当前为无状态 JWT 模式，前端清除 token 即可，
// 后续如需服务端主动失效可接入 Redis 黑名单。
func (s *AuthService) Logout(ctx context.Context) error {
	return nil
}

// issueToken 签发 member JWT token
func (s *AuthService) issueToken(memberID uint) (member.LoginResp, error) {
	j := utils.NewJWT()
	claims := j.CreateClaims(sysReq.BaseClaims{
		ID: memberID,
	})
	token, err := j.CreateToken(claims)
	if err != nil {
		return member.LoginResp{}, err
	}
	return member.LoginResp{Token: token}, nil
}
