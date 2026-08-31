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

type IcApi struct{}

// CreateIc 创建IC
// @Tags Ic
// @Summary 创建IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Ic true "创建IC"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /ic/createIc [post]
func (icApi *IcApi) CreateIc(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var ic bi.Ic
	err := c.ShouldBindJSON(&ic)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ic.CreatedBy = utils.GetUserID(c)
	err = icService.CreateIc(ctx, &ic)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteIc 删除IC
// @Tags Ic
// @Summary 删除IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Ic true "删除IC"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /ic/deleteIc [delete]
func (icApi *IcApi) DeleteIc(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := icService.DeleteIc(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteIcByIds 批量删除IC
// @Tags Ic
// @Summary 批量删除IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /ic/deleteIcByIds [delete]
func (icApi *IcApi) DeleteIcByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := icService.DeleteIcByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateIc 更新IC
// @Tags Ic
// @Summary 更新IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Ic true "更新IC"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /ic/updateIc [put]
func (icApi *IcApi) UpdateIc(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var ic bi.Ic
	err := c.ShouldBindJSON(&ic)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ic.UpdatedBy = utils.GetUserID(c)
	err = icService.UpdateIc(ctx, ic)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindIc 用id查询IC
// @Tags Ic
// @Summary 用id查询IC
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询IC"
// @Success 200 {object} response.Response{data=bi.Ic,msg=string} "查询成功"
// @Router /ic/findIc [get]
func (icApi *IcApi) FindIc(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	reic, err := icService.GetIc(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reic, c)
}

// GetIcList 分页获取IC列表
// @Tags Ic
// @Summary 分页获取IC列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.IcSearch true "分页获取IC列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /ic/getIcList [get]
func (icApi *IcApi) GetIcList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo biReq.IcSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := icService.GetIcInfoList(ctx, pageInfo)
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

// GetIcPublic 不需要鉴权的IC接口
// @Tags Ic
// @Summary 不需要鉴权的IC接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /ic/getIcPublic [get]
func (icApi *IcApi) GetIcPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	icService.GetIcPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的IC接口信息",
	}, "获取成功", c)
}
