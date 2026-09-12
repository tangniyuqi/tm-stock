package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthApi struct{}

var authService = service.ServiceGroupApp.MemberServiceGroup.AuthService

var smsService = service.ServiceGroupApp.MemberServiceGroup.SmsService

// ============ 短信验证码 ============

// Login 登录（密码 / 手机验证码两种方式）
// @Tags ClientUserAuth
// @Summary 用户登录，支持密码登录与手机验证码登录
// @Description 对应前端 pages/member/login：密码登录 + 手机验证码登录两种 tab。
// @Accept application/json
// @Produce application/json
// @Param data body member.LoginReq true "登录请求"
// @Success 200 {object} response.Response{data=member.LoginResp,msg=string} "登录成功"
// @Router /client/auth/login [post]
func (a *AuthApi) Login(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	var resp member.LoginResp
	var err error
	if req.Password != "" {
		resp, err = authService.LoginByPassword(ctx, req)
	} else {
		resp, err = authService.LoginByCode(ctx, req)
	}
	if err != nil {
		global.GVA_LOG.Error("登录失败!", zap.Error(err))
		response.FailWithMessage("登录失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(resp, "登录成功", c)
}

// Register 注册
// @Tags ClientUserAuth
// @Summary 用户注册（手机号 + 短信验证码 + 密码）
// @Description 对应前端 pages/member/register：手机号/验证码/密码/确认密码
// @Accept application/json
// @Produce application/json
// @Param data body member.RegisterReq true "注册请求"
// @Success 200 {object} response.Response{data=member.LoginResp,msg=string} "注册成功"
// @Router /client/auth/register [post]
func (a *AuthApi) Register(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	resp, err := authService.Register(ctx, req)
	if err != nil {
		global.GVA_LOG.Error("注册失败!", zap.Error(err))
		response.FailWithMessage("注册失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(resp, "注册成功", c)
}

// ============ 找回 / 重置密码 ============

// ForgotPwd 找回密码（发送重置验证码）
// @Tags ClientUserAuth
// @Summary 提交手机号，发送找回密码的短信验证码
// @Description 对应前端 pages/member/forget：输入手机号提交
// @Accept application/json
// @Produce application/json
// @Param data body member.ForgotPwdReq true "找回密码请求"
// @Success 200 {object} response.Response{msg=string} "验证码已发送"
// @Router /client/auth/forgot-pwd [post]
func (a *AuthApi) ForgotPwd(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.ForgotPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := authService.ForgotPwd(ctx, req, c.ClientIP()); err != nil {
		global.GVA_LOG.Error("找回密码失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("验证码已发送", c)
}

// ResetPwd 重置密码（验证码校验后设置新密码）
// @Tags ClientUserAuth
// @Summary 通过手机验证码重置密码
// @Description 对应前端 pages/member/reset-password：输入新密码 + 确认密码提交
// @Accept application/json
// @Produce application/json
// @Param data body member.ResetPwdReq true "重置密码请求"
// @Success 200 {object} response.Response{msg=string} "重置成功"
// @Router /client/auth/reset-pwd [post]
func (a *AuthApi) ResetPwd(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.ResetPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := authService.ResetPwd(ctx, req); err != nil {
		global.GVA_LOG.Error("重置密码失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("重置成功", c)
}

// Logout 登出
// @Tags ClientUserAuth
// @Summary 用户登出，注销 token
// @Description 注销当前登录态（前端退出登录调用）
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "登出成功"
// @Router /client/auth/logout [post]
func (a *AuthApi) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	if err := authService.Logout(ctx); err != nil {
		global.GVA_LOG.Error("登出失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("登出成功", c)
}

// SendCode 发送短信验证码
// @Tags ClientUserAuth
// @Summary 发送短信验证码（登录/注册/找回密码/绑定手机号）
// @Description 通用发送短信验证码，type 区分业务场景（login/register/forgot/bind_mobile），成功写入 addon_sms_log
// @Accept application/json
// @Produce application/json
// @Param data body member.SendCodeReq true "发送验证码请求"
// @Success 200 {object} response.Response{msg=string} "验证码已发送"
// @Router /client/auth/send-code [post]
func (a *AuthApi) SendCode(c *gin.Context) {
	ctx := c.Request.Context()
	var req member.SendCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if err := smsService.SendCode(ctx, req.Mobile, req.Type, c.ClientIP()); err != nil {
		global.GVA_LOG.Error("发送验证码失败!", zap.Error(err))
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("验证码已发送", c)
}
