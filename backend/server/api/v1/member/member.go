package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	memberReq "github.com/flipped-aurora/gin-vue-admin/server/model/member/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MemberApi struct{}

// CreateMember 创建C端会员
// @Tags Member
// @Summary 创建C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.Member true "创建会员"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /member/member/createMember [post]
func (a *MemberApi) CreateMember(c *gin.Context) {
	ctx := c.Request.Context()

	var m member.Member
	if err := c.ShouldBindJSON(&m); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	m.CreatedBy = utils.GetUserID(c)

	if err := memberService.CreateMemberBiz(ctx, &m); err != nil {
		global.GVA_LOG.Error("创建会员失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteMember 删除C端会员
// @Tags Member
// @Summary 删除C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query string true "会员ID"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /member/member/deleteMember [delete]
func (a *MemberApi) DeleteMember(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)

	if err := memberService.DeleteMember(ctx, id, userID); err != nil {
		global.GVA_LOG.Error("删除会员失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteMemberByIds 批量删除C端会员
// @Tags Member
// @Summary 批量删除C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /member/member/deleteMemberByIds [delete]
func (a *MemberApi) DeleteMemberByIds(c *gin.Context) {
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)

	if err := memberService.DeleteMemberByIds(ctx, ids, userID); err != nil {
		global.GVA_LOG.Error("批量删除会员失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateMember 更新C端会员
// @Tags Member
// @Summary 更新C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.Member true "更新会员"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /member/member/updateMember [put]
func (a *MemberApi) UpdateMember(c *gin.Context) {
	ctx := c.Request.Context()

	var m member.Member
	if err := c.ShouldBindJSON(&m); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	m.UpdatedBy = utils.GetUserID(c)

	if err := memberService.UpdateMemberBiz(ctx, m); err != nil {
		global.GVA_LOG.Error("更新会员失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindMember 用id查询C端会员
// @Tags Member
// @Summary 用id查询C端会员
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query string true "会员ID"
// @Success 200 {object} response.Response{data=member.Member,msg=string} "查询成功"
// @Router /member/member/findMember [get]
func (a *MemberApi) FindMember(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Query("id")
	m, err := memberService.GetMemberByID(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询会员失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(m, c)
}

// ResetMemberPassword 后台管理 - 重置会员密码
// @Tags Member
// @Summary 后台管理 - 重置会员密码
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body member.ResetMemberPwdReq true "重置密码"
// @Success 200 {object} response.Response{msg=string} "重置成功"
// @Router /member/member/resetMemberPassword [post]
func (a *MemberApi) ResetMemberPassword(c *gin.Context) {
	ctx := c.Request.Context()

	var req member.ResetMemberPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if len(req.Password) < 6 {
		response.FailWithMessage("密码长度不能少于6位", c)
		return
	}

	if err := memberService.ResetMemberPassword(ctx, req.ID, req.Password); err != nil {
		global.GVA_LOG.Error("重置会员密码失败!", zap.Error(err))
		response.FailWithMessage("重置失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("密码重置成功", c)
}

// GetMemberList 分页获取C端会员列表
// @Tags Member
// @Summary 分页获取C端会员列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query memberReq.MemberSearch true "分页获取会员列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /member/member/getMemberList [get]
func (a *MemberApi) GetMemberList(c *gin.Context) {
	ctx := c.Request.Context()

	var pageInfo memberReq.MemberSearch
	if err := c.ShouldBindQuery(&pageInfo); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	list, total, err := memberService.GetMemberInfoList(ctx, pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取会员列表失败!", zap.Error(err))
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
