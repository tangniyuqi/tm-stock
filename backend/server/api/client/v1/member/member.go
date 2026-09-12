package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MemberApi struct{}

var memberService = service.ServiceGroupApp.MemberServiceGroup.MemberService

// ============ 我的 / 资料 ============

// Profile 我的 / 个人资料
// @Tags ClientMemberMember
// @Summary 获取当前用户资料（我的 页 + 个人资料设置）
// @Description 对应前端 pages/member/my 与 setting/profile：VIP等级、积分、昵称、头像、脱敏手机号
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=member.MemberProfile,msg=string} "获取成功"
// @Router /client/member/profile [get]
func (u *MemberApi) Profile(c *gin.Context) {
	ctx := c.Request.Context()
	memberID := getMemberID(c)
	profile, err := memberService.GetProfile(ctx, memberID)
	if err != nil {
		global.GVA_LOG.Error("获取当前用户资料失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(profile, "获取成功", c)
}

// UpdateProfile 更新个人资料
// @Tags ClientMemberMember
// @Summary 更新昵称 / 头像等个人资料
// @Description 对应前端 setting/profile：修改昵称、头像、简介等
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.UpdateProfileReq true "更新的资料字段"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /client/member/profile [put]
func (u *MemberApi) UpdateProfile(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := memberService.UpdateProfile(ctx, getMemberID(c), req); err != nil {
		global.GVA_LOG.Error("更新个人资料失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// ============ 账号 / 手机 / 密码 ============

// BindMobile 绑定 / 更换手机号码
// @Tags ClientMemberMember
// @Summary 绑定或更换手机号（需短信验证码）
// @Description 对应前端 setting/mobile：输入新手机号 + 短信验证码
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.BindMobileReq true "绑定手机号请求"
// @Success 200 {object} response.Response{msg=string} "绑定成功"
// @Router /client/member/mobile [put]
func (u *MemberApi) BindMobile(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.BindMobileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := memberService.BindMobile(ctx, getMemberID(c), req); err != nil {
		global.GVA_LOG.Error("绑定手机号失败!", zap.Error(err))
		response.FailWithMessage("绑定失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("绑定成功", c)
}

// ChangePwd 修改密码
// @Tags ClientMemberMember
// @Summary 修改登录密码（需原密码校验）
// @Description 对应前端 setting/password：原密码 + 新密码 + 确认密码
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.ChangePwdReq true "修改密码请求"
// @Success 200 {object} response.Response{msg=string} "修改成功"
// @Router /client/member/password [put]
func (u *MemberApi) ChangePwd(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.ChangePwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := memberService.ChangePwd(ctx, getMemberID(c), req); err != nil {
		global.GVA_LOG.Error("修改密码失败!", zap.Error(err))
		response.FailWithMessage("修改失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("修改成功", c)
}

// ============ 实名认证 ============

// RealName 实名认证
// @Tags ClientMemberMember
// @Summary 提交实名认证信息（姓名 + 证件号）
// @Description 对应前端 setting/verify：实名认证提交
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.RealNameReq true "实名认证请求"
// @Success 200 {object} response.Response{msg=string} "提交成功"
// @Router /client/member/realname [post]
func (u *MemberApi) RealName(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.RealNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := memberService.RealName(ctx, getMemberID(c), req); err != nil {
		global.GVA_LOG.Error("实名认证失败!", zap.Error(err))
		response.FailWithMessage("提交失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("提交成功", c)
}

// ============ 账号设置 / 登出 ============

// AccountSetting 账号设置（展示与开关）
// @Tags ClientMemberMember
// @Summary 获取账号设置项（账号创建时间等，按业务扩展）
// @Description 对应前端 setting/account：账号相关设置聚合（具体项按业务扩展）
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=member.AccountSetting,msg=string} "获取成功"
// @Router /client/member/settings [get]
func (u *MemberApi) AccountSetting(c *gin.Context) {
	ctx := c.Request.Context()
	setting, err := memberService.AccountSetting(ctx, getMemberID(c))
	if err != nil {
		global.GVA_LOG.Error("获取账号设置失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(setting, "获取成功", c)
}

// UpdateAccount 自定义用户名（可用于登录）
// @Tags ClientMemberMember
// @Summary 设置用户名（账号），用户名可用于密码登录
// @Description 对应前端 setting/account：查看并自定义用户名
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.UpdateAccountReq true "用户名请求"
// @Success 200 {object} response.Response{msg=string} "设置成功"
// @Router /client/member/name [put]
func (u *MemberApi) UpdateAccount(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.UpdateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := memberService.UpdateAccount(ctx, getMemberID(c), req); err != nil {
		global.GVA_LOG.Error("设置用户名失败!", zap.Error(err))
		response.FailWithMessage("设置失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("设置成功", c)
}

// getMemberID 从 C 端鉴权中间件写入的 claims 中取出当前会员 ID
func getMemberID(c *gin.Context) uint {
	return utils.GetUserID(c)
}
