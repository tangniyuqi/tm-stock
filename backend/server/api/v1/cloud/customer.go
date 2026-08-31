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

type CustomerApi struct{}

// CreateCustomer 创建客户
// @Tags Customer
// @Summary 创建客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Customer true "创建客户"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /customer/createCustomer [post]
func (customerApi *CustomerApi) CreateCustomer(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var customer cloud.Customer
	err := c.ShouldBindJSON(&customer)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	customer.CreatedBy = utils.GetUserID(c)
	err = customerService.CreateCustomer(ctx, &customer)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteCustomer 删除客户
// @Tags Customer
// @Summary 删除客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Customer true "删除客户"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /customer/deleteCustomer [delete]
func (customerApi *CustomerApi) DeleteCustomer(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := customerService.DeleteCustomer(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteCustomerByIds 批量删除客户
// @Tags Customer
// @Summary 批量删除客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /customer/deleteCustomerByIds [delete]
func (customerApi *CustomerApi) DeleteCustomerByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := customerService.DeleteCustomerByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateCustomer 更新客户
// @Tags Customer
// @Summary 更新客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Customer true "更新客户"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /customer/updateCustomer [put]
func (customerApi *CustomerApi) UpdateCustomer(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var customer cloud.Customer
	err := c.ShouldBindJSON(&customer)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	customer.UpdatedBy = utils.GetUserID(c)
	err = customerService.UpdateCustomer(ctx, customer)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindCustomer 用ID查询客户
// @Tags Customer
// @Summary 用ID查询客户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询客户"
// @Success 200 {object} response.Response{data=cloud.Customer,msg=string} "查询成功"
// @Router /customer/findCustomer [get]
func (customerApi *CustomerApi) FindCustomer(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	recustomer, err := customerService.GetCustomer(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(recustomer, c)
}

// GetCustomerList 分页获取客户列表
// @Tags Customer
// @Summary 分页获取客户列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.CustomerSearch true "分页获取客户列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /customer/getCustomerList [get]
func (customerApi *CustomerApi) GetCustomerList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo cloudReq.CustomerSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	pageInfo.CreatedBy = utils.GetUserID(c)
	list, total, err := customerService.GetCustomerInfoList(ctx, pageInfo)
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

// GetCustomerPublic 不需要鉴权的客户接口
// @Tags Customer
// @Summary 不需要鉴权的客户接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /customer/getCustomerPublic [get]
func (customerApi *CustomerApi) GetCustomerPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	customerService.GetCustomerPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的客户接口信息",
	}, "获取成功", c)
}
