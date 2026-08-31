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

type GroupApi struct{}

// CreateGroup 创建群组
// @Tags Group
// @Summary 创建群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Group true "创建群组"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /group/createGroup [post]
func (groupApi *GroupApi) CreateGroup(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var group bi.Group
	err := c.ShouldBindJSON(&group)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	group.CreatedBy = utils.GetUserID(c)
	err = groupService.CreateGroup(ctx, &group)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteGroup 删除群组
// @Tags Group
// @Summary 删除群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Group true "删除群组"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /group/deleteGroup [delete]
func (groupApi *GroupApi) DeleteGroup(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := groupService.DeleteGroup(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteGroupByIds 批量删除群组
// @Tags Group
// @Summary 批量删除群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /group/deleteGroupByIds [delete]
func (groupApi *GroupApi) DeleteGroupByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := groupService.DeleteGroupByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateGroup 更新群组
// @Tags Group
// @Summary 更新群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Group true "更新群组"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /group/updateGroup [put]
func (groupApi *GroupApi) UpdateGroup(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var group bi.Group
	err := c.ShouldBindJSON(&group)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	group.UpdatedBy = utils.GetUserID(c)
	err = groupService.UpdateGroup(ctx, group)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindGroup 用id查询群组
// @Tags Group
// @Summary 用id查询群组
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询群组"
// @Success 200 {object} response.Response{data=bi.Group,msg=string} "查询成功"
// @Router /group/findGroup [get]
func (groupApi *GroupApi) FindGroup(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	regroup, err := groupService.GetGroup(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(regroup, c)
}

// GetGroupList 分页获取群组列表
// @Tags Group
// @Summary 分页获取群组列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.GroupSearch true "分页获取群组列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /group/getGroupList [get]
func (groupApi *GroupApi) GetGroupList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo biReq.GroupSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := groupService.GetGroupInfoList(ctx, pageInfo)
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

// GetGroupPublic 不需要鉴权的群组接口
// @Tags Group
// @Summary 不需要鉴权的群组接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /group/getGroupPublic [get]
func (groupApi *GroupApi) GetGroupPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	groupService.GetGroupPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的群组接口信息",
	}, "获取成功", c)
}
