package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MemberApi struct{}

// CreateMember 创建用户
// @Tags Member
// @Summary 创建用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Member true "创建用户"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /member/createMember [post]
func (memberApi *MemberApi) CreateMember(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var member bi.Member
	err := c.ShouldBindJSON(&member)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if member.Mobile != nil && *member.Mobile != "" {
		exists, err := memberService.CheckMobileExists(ctx, *member.Mobile, 0)

		if err != nil {
			global.GVA_LOG.Error("检查手机号唯一性失败", zap.Error(err))
			response.FailWithMessage("系统错误，请稍后重试", c)
			return
		}

		if exists {
			response.FailWithMessage("该手机号已被占用！", c)
			return
		}
	}

	if member.Wechat != nil && *member.Wechat != "" {
		exists, err := memberService.CheckWechatExists(ctx, *member.Wechat, 0)

		if err != nil {
			global.GVA_LOG.Error("检查微信唯一性失败", zap.Error(err))
			response.FailWithMessage("系统错误，请稍后重试", c)
			return
		}

		if exists {
			response.FailWithMessage("该微信号已被占用！", c)
			return
		}
	}

	member.CreatedBy = utils.GetUserID(c)
	err = memberService.CreateMember(ctx, &member)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteMember 删除用户
// @Tags Member
// @Summary 删除用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Member true "删除用户"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /member/deleteMember [delete]
func (memberApi *MemberApi) DeleteMember(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := memberService.DeleteMember(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteMemberByIds 批量删除用户
// @Tags Member
// @Summary 批量删除用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /member/deleteMemberByIds [delete]
func (memberApi *MemberApi) DeleteMemberByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := memberService.DeleteMemberByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateMember 更新用户
// @Tags Member
// @Summary 更新用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Member true "更新用户"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /member/updateMember [put]
func (memberApi *MemberApi) UpdateMember(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var member bi.Member
	err := c.ShouldBindJSON(&member)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	if member.Mobile != nil && *member.Mobile != "" {
		exists, err := memberService.CheckMobileExists(ctx, *member.Mobile, member.ID)

		if err != nil {
			global.GVA_LOG.Error("检查手机号唯一性失败", zap.Error(err))
			response.FailWithMessage("系统错误，请稍后重试", c)
			return
		}

		if exists {
			response.FailWithMessage("该手机号已被占用！", c)
			return
		}
	}

	if member.Wechat != nil && *member.Wechat != "" {
		exists, err := memberService.CheckWechatExists(ctx, *member.Wechat, member.ID)

		if err != nil {
			global.GVA_LOG.Error("检查微信唯一性失败", zap.Error(err))
			response.FailWithMessage("系统错误，请稍后重试", c)
			return
		}

		if exists {
			response.FailWithMessage("该微信号已被占用！", c)
			return
		}
	}

	member.UpdatedBy = utils.GetUserID(c)
	err = memberService.UpdateMember(ctx, member)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindMember 用id查询用户
// @Tags Member
// @Summary 用id查询用户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询用户"
// @Success 200 {object} response.Response{data=bi.Member,msg=string} "查询成功"
// @Router /member/findMember [get]
func (memberApi *MemberApi) FindMember(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	remember, err := memberService.GetMember(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(remember, c)
}

// GetMemberList 分页获取用户列表
// @Tags Member
// @Summary 分页获取用户列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.MemberSearch true "分页获取用户列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /member/getMemberList [get]
func (memberApi *MemberApi) GetMemberList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo biReq.MemberSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := memberService.GetMemberInfoList(ctx, pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetMemberPublic 不需要鉴权的用户接口
// @Tags Member
// @Summary 不需要鉴权的用户接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /member/getMemberPublic [get]
func (memberApi *MemberApi) GetMemberPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	memberService.GetMemberPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的用户接口信息",
	}, "获取成功", c)
}
