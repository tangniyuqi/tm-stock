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

type ClientApi struct{}

// CreateClient 创建客户端
// @Tags Client
// @Summary 创建客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Client true "创建客户端"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /client/createClient [post]
func (clientApi *ClientApi) CreateClient(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var client bi.Client
	err := c.ShouldBindJSON(&client)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	client.CreatedBy = utils.GetUserID(c)
	err = clientService.CreateClient(ctx, &client)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteClient 删除客户端
// @Tags Client
// @Summary 删除客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Client true "删除客户端"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /client/deleteClient [delete]
func (clientApi *ClientApi) DeleteClient(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := clientService.DeleteClient(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteClientByIds 批量删除客户端
// @Tags Client
// @Summary 批量删除客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /client/deleteClientByIds [delete]
func (clientApi *ClientApi) DeleteClientByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := clientService.DeleteClientByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateClient 更新客户端
// @Tags Client
// @Summary 更新客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Client true "更新客户端"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /client/updateClient [put]
func (clientApi *ClientApi) UpdateClient(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var client bi.Client
	err := c.ShouldBindJSON(&client)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	client.UpdatedBy = utils.GetUserID(c)
	err = clientService.UpdateClient(ctx, client)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindClient 用id查询客户端
// @Tags Client
// @Summary 用id查询客户端
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询客户端"
// @Success 200 {object} response.Response{data=bi.Client,msg=string} "查询成功"
// @Router /client/findClient [get]
func (clientApi *ClientApi) FindClient(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	reclient, err := clientService.GetClient(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reclient, c)
}

// GetClientList 分页获取客户端列表
// @Tags Client
// @Summary 分页获取客户端列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.ClientSearch true "分页获取客户端列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /client/getClientList [get]
func (clientApi *ClientApi) GetClientList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo biReq.ClientSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := clientService.GetClientInfoList(ctx, pageInfo)
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

// GetClientPublic 不需要鉴权的客户端接口
// @Tags Client
// @Summary 不需要鉴权的客户端接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /client/getClientPublic [get]
func (clientApi *ClientApi) GetClientPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	clientService.GetClientPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的客户端接口信息",
	}, "获取成功", c)
}
