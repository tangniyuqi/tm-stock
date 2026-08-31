package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	cloudReq "github.com/flipped-aurora/gin-vue-admin/server/model/cloud/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DealApi struct{}

// CreateDeal 创建商机
// @Tags Deal
// @Summary 创建商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Deal true "创建商机"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /deal/createDeal [post]
func (dealApi *DealApi) CreateDeal(c *gin.Context) {
	var deal cloud.Deal
	err := c.ShouldBindJSON(&deal)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	deal.CreatedBy = utils.GetUserID(c)
	err = dealService.CreateDeal(&deal)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteDeal 删除商机
// @Tags Deal
// @Summary 删除商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Deal true "删除商机"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /deal/deleteDeal [delete]
func (dealApi *DealApi) DeleteDeal(c *gin.Context) {
	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := dealService.DeleteDeal(id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteDealByIds 批量删除商机
// @Tags Deal
// @Summary 批量删除商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /deal/deleteDealByIds [delete]
func (dealApi *DealApi) DeleteDealByIds(c *gin.Context) {
	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := dealService.DeleteDealByIds(ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateDeal 更新商机
// @Tags Deal
// @Summary 更新商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Deal true "更新商机"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /deal/updateDeal [put]
func (dealApi *DealApi) UpdateDeal(c *gin.Context) {
	var deal cloud.Deal
	err := c.ShouldBindJSON(&deal)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	deal.UpdatedBy = utils.GetUserID(c)
	err = dealService.UpdateDeal(deal)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindDeal 用ID查询商机
// @Tags Deal
// @Summary 用ID查询商机
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询商机"
// @Success 200 {object} response.Response{data=cloud.Deal,msg=string} "查询成功"
// @Router /deal/findDeal [get]
func (dealApi *DealApi) FindDeal(c *gin.Context) {
	id := c.Query("id")
	redeal, err := dealService.GetDeal(id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(redeal, c)
}

// GetDealList 分页获取商机列表
// @Tags Deal
// @Summary 分页获取商机列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.DealSearch true "分页获取商机列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /deal/getDealList [get]
func (dealApi *DealApi) GetDealList(c *gin.Context) {
	var pageInfo cloudReq.DealSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	pageInfo.CreatedBy = utils.GetUserID(c)
	list, total, err := dealService.GetDealInfoList(pageInfo)
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

// GetDealPublic 不需要鉴权的商机接口
// @Tags Deal
// @Summary 不需要鉴权的商机接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /deal/getDealPublic [get]
func (dealApi *DealApi) GetDealPublic(c *gin.Context) {
	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	dealService.GetDealPublic()
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的商机接口信息",
	}, "获取成功", c)
}
