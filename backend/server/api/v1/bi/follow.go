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

type FollowApi struct{}

// CreateFollow 创建跟进
// @Tags Follow
// @Summary 创建跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Follow true "创建跟进"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /follow/createFollow [post]
func (followApi *FollowApi) CreateFollow(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var follow bi.Follow
	err := c.ShouldBindJSON(&follow)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	follow.CreatedBy = utils.GetUserID(c)
	err = followService.CreateFollow(ctx, &follow)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteFollow 删除跟进
// @Tags Follow
// @Summary 删除跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Follow true "删除跟进"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /follow/deleteFollow [delete]
func (followApi *FollowApi) DeleteFollow(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := followService.DeleteFollow(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteFollowByIds 批量删除跟进
// @Tags Follow
// @Summary 批量删除跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /follow/deleteFollowByIds [delete]
func (followApi *FollowApi) DeleteFollowByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := followService.DeleteFollowByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateFollow 更新跟进
// @Tags Follow
// @Summary 更新跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Follow true "更新跟进"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /follow/updateFollow [put]
func (followApi *FollowApi) UpdateFollow(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var follow bi.Follow
	err := c.ShouldBindJSON(&follow)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	follow.UpdatedBy = utils.GetUserID(c)
	err = followService.UpdateFollow(ctx, follow)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindFollow 用id查询跟进
// @Tags Follow
// @Summary 用id查询跟进
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询跟进"
// @Success 200 {object} response.Response{data=bi.Follow,msg=string} "查询成功"
// @Router /follow/findFollow [get]
func (followApi *FollowApi) FindFollow(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	refollow, err := followService.GetFollow(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(refollow, c)
}

// GetFollowList 分页获取跟进列表
// @Tags Follow
// @Summary 分页获取跟进列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.FollowSearch true "分页获取跟进列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /follow/getFollowList [get]
func (followApi *FollowApi) GetFollowList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo biReq.FollowSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := followService.GetFollowInfoList(ctx, pageInfo)
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

// GetFollowPublic 不需要鉴权的跟进接口
// @Tags Follow
// @Summary 不需要鉴权的跟进接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /follow/getFollowPublic [get]
func (followApi *FollowApi) GetFollowPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	followService.GetFollowPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的跟进接口信息",
	}, "获取成功", c)
}
