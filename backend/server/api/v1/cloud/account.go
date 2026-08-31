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

type AccountApi struct{}

// CreateAccount 创建财务
// @Tags Account
// @Summary 创建财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Account true "创建财务"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /account/createAccount [post]
func (accountApi *AccountApi) CreateAccount(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var account cloud.Account
	err := c.ShouldBindJSON(&account)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	account.CreatedBy = utils.GetUserID(c)
	err = accountService.CreateAccount(ctx, &account)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteAccount 删除财务
// @Tags Account
// @Summary 删除财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Account true "删除财务"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /account/deleteAccount [delete]
func (accountApi *AccountApi) DeleteAccount(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := accountService.DeleteAccount(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteAccountByIds 批量删除财务
// @Tags Account
// @Summary 批量删除财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /account/deleteAccountByIds [delete]
func (accountApi *AccountApi) DeleteAccountByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := accountService.DeleteAccountByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateAccount 更新财务
// @Tags Account
// @Summary 更新财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cloud.Account true "更新财务"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /account/updateAccount [put]
func (accountApi *AccountApi) UpdateAccount(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var account cloud.Account
	err := c.ShouldBindJSON(&account)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	account.UpdatedBy = utils.GetUserID(c)
	err = accountService.UpdateAccount(ctx, account)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindAccount 用ID查询财务
// @Tags Account
// @Summary 用ID查询财务
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询财务"
// @Success 200 {object} response.Response{data=cloud.Account,msg=string} "查询成功"
// @Router /account/findAccount [get]
func (accountApi *AccountApi) FindAccount(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	reaccount, err := accountService.GetAccount(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reaccount, c)
}

// GetAccountList 分页获取财务列表
// @Tags Account
// @Summary 分页获取财务列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cloudReq.AccountSearch true "分页获取财务列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /account/getAccountList [get]
func (accountApi *AccountApi) GetAccountList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo cloudReq.AccountSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	pageInfo.CreatedBy = utils.GetUserID(c)
	list, total, err := accountService.GetAccountInfoList(ctx, pageInfo)
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

// GetAccountPublic 不需要鉴权的财务接口
// @Tags Account
// @Summary 不需要鉴权的财务接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /account/getAccountPublic [get]
func (accountApi *AccountApi) GetAccountPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	accountService.GetAccountPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的财务接口信息",
	}, "获取成功", c)
}
