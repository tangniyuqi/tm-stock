package member

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 短信业务类型（addon_sms_log.biz_type）
const (
	SmsBizLogin      = "login"       // 验证码登录
	SmsBizRegister   = "register"    // 注册
	SmsBizForgot     = "forgot"      // 找回 / 重置密码
	SmsBizBindMobile = "bind_mobile" // 绑定 / 更换手机号
)

// 短信日志状态（addon_sms_log.status）
const (
	SmsStatusPending = 0 // 待验证（已发送）
	SmsStatusUsed    = 1 // 已验证使用
	SmsStatusExpired = 2 // 已过期
	SmsStatusFailed  = 3 // 发送失败
)

// SmsLog 短信日志（对应 addon_sms_log 表）
type SmsLog struct {
	global.GVA_MODEL_ADDON
	Mobile     string     `json:"mobile" form:"mobile" gorm:"index;comment:手机号;column:mobile;size:20;"`                // 手机号
	Type       uint8      `json:"type" form:"type" gorm:"default:1;comment:类型(1验证码/2通知);column:type;"`                // 类型
	Code       string     `json:"code" form:"code" gorm:"comment:验证码;column:code;size:8;"`                              // 验证码
	Content    string     `json:"content" form:"content" gorm:"comment:文本内容;column:content;size:500;"`                  // 文本内容
	Status     uint8      `json:"status" form:"status" gorm:"default:0;comment:状态(0待验证/1已用/2过期/3发送失败);column:status;"` // 状态
	BizType    string     `json:"biz_type" form:"biz_type" gorm:"index;comment:业务模块;column:biz_type;size:32;"`         // 业务模块
	BizId      string     `json:"biz_id" form:"biz_id" gorm:"comment:业务关联ID;column:biz_id;size:64;"`                    // 业务关联ID
	IP         string     `json:"ip" form:"ip" gorm:"index;comment:IP;column:ip;size:45;"`                                // IP
	OutId      string     `json:"out_id" form:"out_id" gorm:"comment:返回消息ID;column:out_id;size:64;"`                    // 返回消息ID
	RespCode   string     `json:"resp_code" form:"resp_code" gorm:"comment:返回状态码;column:resp_code;size:64;"`            // 返回状态码
	RespMsg    string     `json:"resp_msg" form:"resp_msg" gorm:"comment:返回错误描述;column:resp_msg;size:255;"`            // 返回错误描述
	RetryCount uint8      `json:"retry_count" form:"retry_count" gorm:"default:0;comment:重试次数;column:retry_count;"`     // 重试次数
	FailCount  uint8      `json:"fail_count" form:"fail_count" gorm:"default:0;comment:输错次数;column:fail_count;"`        // 输错次数
	SendTime   *time.Time `json:"send_time" form:"send_time" gorm:"comment:发送时间;column:send_time;"`                     // 发送时间
	UseTime    *time.Time `json:"use_time" form:"use_time" gorm:"comment:使用时间;column:use_time;"`                        // 使用时间
}

// TableName 短信日志自定义表名 addon_sms_log
func (SmsLog) TableName() string {
	return "addon_sms_log"
}
