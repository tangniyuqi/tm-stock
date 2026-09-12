package member

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
)

var (
	mobileRegexp   = regexp.MustCompile(`^1[3-9]\d{9}$`)
	idCardRegexp   = regexp.MustCompile(`(^\d{15}$)|(^\d{17}[\dXx]$)`)
	usernameRegexp = regexp.MustCompile(`^[\p{L}\p{N}_]{2,20}$`)
)

// MemberService C 端会员（题材宝典）业务服务
type MemberService struct{}

// GetMember 根据 ID 获取会员记录
func (s *MemberService) GetMember(ctx context.Context, id uint) (m member.Member, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&m).Error
	return
}

// GetProfile 获取当前会员资料（含脱敏手机号）
func (s *MemberService) GetProfile(ctx context.Context, id uint) (member.MemberProfile, error) {
	m, err := s.GetMember(ctx, id)
	if err != nil {
		return member.MemberProfile{}, err
	}
	profile := member.MemberProfile{
		Nickname: m.Nickname,
		Avatar:   m.Avatar,
		Mobile:   maskMobile(m.Mobile),
		Level:    int(m.Level),
		Credit:   0,
		Gender:   m.Gender,
		Email:    m.Email,
		Wechat:   m.Wechat,
		Bio:      m.Bio,
		Verified: m.Realname != "",
	}
	// 积分改由资产表（addon_member_asset）提供，存量会员无资产记录时按 0 处理
	var asset member.MemberAsset
	if err := global.GVA_DB.WithContext(ctx).Where("member_id = ?", id).First(&asset).Error; err == nil {
		profile.Credit = asset.Credit
	}
	return profile, nil
}

// UpdateProfile 更新昵称 / 头像等个人资料，仅更新传入的字段
func (s *MemberService) UpdateProfile(ctx context.Context, id uint, req member.UpdateProfileReq) error {
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		return errors.New("昵称不能为空")
	}
	updates := make(map[string]interface{})
	updates["nickname"] = nickname
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	// 简介 / 邮箱 / 微信允许清空，直接以表单当前值覆盖
	updates["bio"] = req.Bio
	updates["email"] = req.Email
	updates["wechat"] = req.Wechat
	if req.Birthday != nil {
		updates["birthday"] = req.Birthday
	}
	if len(updates) == 0 {
		return nil
	}
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("id = ?", id).Updates(updates).Error
}

// BindMobile 绑定 / 更换手机号。短信验证码的真实校验需与短信服务联调，
// 当前仅要求非空并校验新手机号格式与唯一性。
func (s *MemberService) BindMobile(ctx context.Context, id uint, req member.BindMobileReq) error {
	mobile := strings.TrimSpace(req.Mobile)
	if !mobileRegexp.MatchString(mobile) {
		return errors.New("手机号格式不正确")
	}
	if err := smsService.VerifyCode(ctx, mobile, member.SmsBizBindMobile, strings.TrimSpace(req.Code)); err != nil {
		return err
	}
	var count int64
	if err := global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("mobile = ? AND id != ?", mobile, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该手机号已被使用")
	}
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("id = ?", id).Update("mobile", mobile).Error
}

// ChangePwd 修改登录密码（校验原密码后写入 bcrypt 哈希）
func (s *MemberService) ChangePwd(ctx context.Context, id uint, req member.ChangePwdReq) error {
	newPwd := strings.TrimSpace(req.NewPassword)
	if len(newPwd) < 6 {
		return errors.New("新密码不能少于 6 位")
	}
	m, err := s.GetMember(ctx, id)
	if err != nil {
		return err
	}
	if m.Password == "" {
		return errors.New("当前账号未设置密码，请通过短信验证码设置")
	}
	if !utils.BcryptCheck(req.OldPassword, m.Password) {
		return errors.New("原密码错误")
	}
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("id = ?", id).Update("password", utils.BcryptHash(newPwd)).Error
}

// RealName 提交实名认证信息（姓名 + 证件号）
func (s *MemberService) RealName(ctx context.Context, id uint, req member.RealNameReq) error {
	name := strings.TrimSpace(req.Name)
	idCard := strings.ToUpper(strings.TrimSpace(req.IdCard))
	if name == "" {
		return errors.New("姓名不能为空")
	}
	if !idCardRegexp.MatchString(idCard) {
		return errors.New("证件号格式不正确")
	}
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"realname": name, "id_card": idCard}).Error
}

// AccountSetting 获取账号设置聚合信息（当前含账号创建时间，具体项按业务扩展）
func (s *MemberService) AccountSetting(ctx context.Context, id uint) (member.AccountSetting, error) {
	m, err := s.GetMember(ctx, id)
	if err != nil {
		return member.AccountSetting{}, err
	}
	return member.AccountSetting{Name: m.Name, CreatedAt: m.CreatedAt}, nil
}

// UpdateAccount 自定义用户名（可用于登录），校验格式与全局唯一
func (s *MemberService) UpdateAccount(ctx context.Context, id uint, req member.UpdateAccountReq) error {
	username := strings.TrimSpace(req.Name)
	if username == "" {
		return errors.New("用户名不能为空")
	}
	if !usernameRegexp.MatchString(username) {
		return errors.New("用户名仅支持 2-20 位中文、字母、数字或下划线")
	}
	var count int64
	if err := global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("name = ? AND id != ?", username, id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该用户名已被使用")
	}
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).
		Where("id = ?", id).Update("name", username).Error
}

// maskMobile 手机号脱敏：138****5678
func maskMobile(mobile string) string {
	if len(mobile) != 11 {
		return mobile
	}
	return mobile[:3] + "****" + mobile[7:]
}
