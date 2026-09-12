package member

import "time"

// MemberProfile 会员资料（对应前端 pages/member/my 与 setting/profile）
type MemberProfile struct {
	Nickname string `json:"nickname"` // 昵称
	Avatar   string `json:"avatar"`   // 头像 URL
	Mobile   string `json:"mobile"`   // 手机号（脱敏展示）
	Level    int    `json:"level"`    // VIP 等级
	Credit   int    `json:"credit"`   // 积分
	Gender   int8   `json:"gender"`   // 性别（0保密 /1男 /2女）
	Email    string `json:"email"`    // 电子邮箱
	Wechat   string `json:"wechat"`   // 微信
	Bio      string `json:"bio"`      // 个人简介
	Verified bool   `json:"verified"` // 是否已完成实名认证
}

// UpdateProfileReq 更新个人资料请求（对应前端 setting/profile）
type UpdateProfileReq struct {
	Avatar   string     `json:"avatar"`   // 头像 URL
	Nickname string     `json:"nickname"` // 昵称
	Gender   *int8      `json:"gender"`   // 性别（nil 表示不修改）
	Bio      string     `json:"bio"`      // 个人简介
	Email    string     `json:"email"`    // 电子邮箱
	Wechat   string     `json:"wechat"`   // 微信
	Birthday *time.Time `json:"birthday"` // 生日（nil 表示不修改）
}

// BindMobileReq 绑定 / 更换手机号请求（对应前端 setting/mobile）
type BindMobileReq struct {
	Mobile string `json:"mobile"` // 新手机号
	Code   string `json:"code"`   // 短信验证码
}

// ChangePwdReq 修改密码请求（对应前端 setting/password）
type ChangePwdReq struct {
	OldPassword string `json:"oldPassword"` // 原密码
	NewPassword string `json:"newPassword"` // 新密码
}

// RealNameReq 实名认证请求（对应前端 setting/verify）
type RealNameReq struct {
	Name   string `json:"name"`   // 姓名
	IdCard string `json:"idCard"` // 证件号
}

// AccountSetting 账号设置（对应前端 setting/account）
type AccountSetting struct {
	Name      string    `json:"name"`      // 用户名（登录凭证，可自定义）
	CreatedAt time.Time `json:"createdAt"` // 账号创建时间
}

// AssetOverview 资产概况（对应前端 api/member/asset.uts，当前为积分资产 type=2）
type AssetOverview struct {
	AssetType        int `json:"assetType"`        // 资产类型（2=积分）
	Credit           int `json:"credit"`           // 当前可用
	FrozenCredit     int `json:"frozenCredit"`     // 冻结
	AccumulateCredit int `json:"accumulateCredit"` // 累计获得
	ConsumeCredit    int `json:"consumeCredit"`    // 累计消费
	GiftCredit       int `json:"giftCredit"`       // 累计赠送
}

// AssetLogItem 资产流水单条（对应前端 api/member/asset_log.uts 明细列表）
// 数据源 addon_member_asset_log 的 type=2（积分）流水。
type AssetLogItem struct {
	ID        uint      `json:"id"`        // 流水ID
	Change    float64   `json:"change"`    // 积分变动数量（正=获得，负=扣减）
	AfterNum  float64   `json:"afterNum"`  // 变动后积分
	FlowType  uint8     `json:"flowType"`  // 流向（1增加 2减少）
	BizType   string    `json:"bizType"`   // 关联业务类型
	Remark    string    `json:"remark"`    // 备注
	CreatedAt time.Time `json:"createdAt"` // 流水发生时间
}

// UpdateAccountReq 更新用户名请求（对应前端 setting/account，用户名可用于登录）
type UpdateAccountReq struct {
	Name string `json:"name"` // 新用户名
}

// ============ C 端鉴权 DTO ============

// LoginReq 登录请求（密码 / 手机验证码二选一）
type LoginReq struct {
	Mobile   string `json:"mobile"`   // 手机号
	Password string `json:"password"` // 密码（密码登录）
	Code     string `json:"code"`     // 短信验证码（验证码登录）
}

// LoginResp 登录响应
type LoginResp struct {
	Token string `json:"token"` // 访问令牌
}

// SendCodeReq 发送短信验证码请求
type SendCodeReq struct {
	Mobile string `json:"mobile"` // 手机号
	Type   string `json:"type"`   // 业务类型（login/register/forgot/bind_mobile）
}

// RegisterReq 注册请求
type RegisterReq struct {
	Mobile   string `json:"mobile"`   // 手机号
	Code     string `json:"code"`     // 短信验证码
	Password string `json:"password"` // 密码
}

// ForgotPwdReq 找回密码请求
type ForgotPwdReq struct {
	Mobile string `json:"mobile"` // 手机号
}

// ResetPwdReq 重置密码请求
type ResetPwdReq struct {
	Mobile   string `json:"mobile"`   // 手机号
	Code     string `json:"code"`     // 短信验证码
	Password string `json:"password"` // 新密码
}

// ============ 后台管理 DTO ============

// ResetMemberPwdReq 后台管理 - 重置会员密码请求
type ResetMemberPwdReq struct {
	ID       uint   `json:"id"`       // 会员ID
	Password string `json:"password"` // 新密码
}
