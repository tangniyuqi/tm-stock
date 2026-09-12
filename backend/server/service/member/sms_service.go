package member

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	memberReq "github.com/flipped-aurora/gin-vue-admin/server/model/member/request"
	"gorm.io/gorm"
)

// 验证码有效期 / 发送间隔 / 最大输错次数
const (
	SmsCodeExpire   = 10 * time.Minute
	SmsCodeCooldown = 60 * time.Second
	SmsMaxFailCount = 5
)

type SmsService struct{}

// package 内共享实例（供 AuthService / MemberService 调用）
var smsService = SmsService{}

// SendCode 生成并下发短信验证码（写入 addon_sms_log）。
// 实际发送已接入短信服务商，当前为占位实现：仅生成验证码并记日志，便于联调与后台查看。
func (s *SmsService) SendCode(ctx context.Context, mobile, kind, ip string) error {
	if !mobileRegexp.MatchString(mobile) {
		return errors.New("手机号格式不正确")
	}

	now := time.Now()
	var latest member.SmsLog
	err := global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).
		Where("mobile = ? AND biz_type = ?", mobile, kind).
		Order("send_time DESC").First(&latest).Error
	if err == nil && latest.SendTime != nil && now.Sub(*latest.SendTime) < SmsCodeCooldown {
		return errors.New("发送过于频繁，请稍后再试")
	}

	// 将同一手机号 + 业务类型下旧的待验证验证码置为过期
	global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).
		Where("mobile = ? AND biz_type = ? AND status = ?", mobile, kind, member.SmsStatusPending).
		Updates(map[string]interface{}{"status": member.SmsStatusExpired, "use_time": now})

	code := genSmsCode()
	rec := member.SmsLog{
		Mobile:   mobile,
		Type:     1, // 1=验证码
		Code:     code,
		Content:  "您的验证码是 " + code + "，10 分钟内有效。",
		Status:   member.SmsStatusPending,
		BizType:  kind,
		IP:       ip,
		SendTime: &now,
	}
	if err := global.GVA_DB.WithContext(ctx).Create(&rec).Error; err != nil {
		return err
	}
	// TODO: 接入短信服务商（阿里云 / 腾讯云）真实下发，成功后回写 out_id / resp_code / resp_msg。
	return nil
}

// VerifyCode 校验短信验证码：成功则标记已使用，失败累加输错次数。
func (s *SmsService) VerifyCode(ctx context.Context, mobile, kind, code string) error {
	now := time.Now()
	var rec member.SmsLog
	if err := global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).
		Where("mobile = ? AND biz_type = ? AND status = ?", mobile, kind, member.SmsStatusPending).
		Order("send_time DESC").First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("请先获取验证码")
		}
		return err
	}

	if rec.SendTime != nil && now.Sub(*rec.SendTime) > SmsCodeExpire {
		global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).Where("id = ?", rec.ID).
			Updates(map[string]interface{}{"status": member.SmsStatusExpired})
		return errors.New("验证码已过期，请重新获取")
	}
	if rec.FailCount >= SmsMaxFailCount {
		global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).Where("id = ?", rec.ID).
			Updates(map[string]interface{}{"status": member.SmsStatusExpired})
		return errors.New("验证码错误次数过多，请重新获取")
	}
	if rec.Code != code {
		global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).Where("id = ?", rec.ID).
			Update("fail_count", rec.FailCount+1)
		return errors.New("验证码不正确")
	}

	return global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{}).Where("id = ?", rec.ID).
		Updates(map[string]interface{}{"status": member.SmsStatusUsed, "use_time": now}).Error
}

// GetSmsLogList 后台管理 - 分页获取短信日志列表
func (s *SmsService) GetSmsLogList(ctx context.Context, info memberReq.SmsLogSearch) (list []member.SmsLog, total int64, err error) {
	limit, offset := info.LimitOffset()

	db := global.GVA_DB.WithContext(ctx).Model(&member.SmsLog{})
	if info.Mobile != nil && *info.Mobile != "" {
		db = db.Where("`mobile` LIKE ?", "%"+*info.Mobile+"%")
	}
	if info.Type != nil {
		db = db.Where("`type` = ?", *info.Type)
	}
	if info.Status != nil {
		db = db.Where("`status` = ?", *info.Status)
	}
	if info.BizType != nil && *info.BizType != "" {
		db = db.Where("`biz_type` = ?", *info.BizType)
	}
	if len(info.SendTimeRange) == 2 {
		db = db.Where("`send_time` BETWEEN ? AND ?", info.SendTimeRange[0], info.SendTimeRange[1])
	}
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("`created_at` BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if err = db.Count(&total).Error; err != nil {
		return
	}
	if limit > 0 {
		db = db.Limit(limit).Offset(offset)
	}
	err = db.Order("send_time DESC, id DESC").Find(&list).Error
	return
}

// DeleteSmsLog 后台管理 - 删除单条短信日志
func (s *SmsService) DeleteSmsLog(ctx context.Context, id uint) error {
	return global.GVA_DB.WithContext(ctx).Delete(&member.SmsLog{}, "id = ?", id).Error
}

// DeleteSmsLogByIds 后台管理 - 批量删除短信日志
func (s *SmsService) DeleteSmsLogByIds(ctx context.Context, ids []uint) error {
	return global.GVA_DB.WithContext(ctx).Delete(&[]member.SmsLog{}, "id in ?", ids).Error
}

// genSmsCode 生成 6 位数字验证码
func genSmsCode() string {
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		code[i] = '0' + byte(n.Int64())
	}
	return string(code)
}
