package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AccountApi struct{}

// CreateAccount 创建账户
// @Tags Account
// @Summary 创建账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Account true "创建账户"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /account/createAccount [post]
func (accountApi *AccountApi) CreateAccount(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var account quant.Account
	err := c.ShouldBindJSON(&account)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if account.MemberId == nil || *account.MemberId == 0 {
		userID := uint32(utils.GetUserID(c))
		account.MemberId = &userID
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

// DeleteAccount 删除账户
// @Tags Account
// @Summary 删除账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Account true "删除账户"
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

// DeleteAccountByIds 批量删除账户
// @Tags Account
// @Summary 批量删除账户
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

// UpdateAccount 更新账户
// @Tags Account
// @Summary 更新账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Account true "更新账户"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /account/updateAccount [put]
func (accountApi *AccountApi) UpdateAccount(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var account quant.Account
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

// FindAccount 用ID查询账户
// @Tags Account
// @Summary 用ID查询账户
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询账户"
// @Success 200 {object} response.Response{data=quant.Account,msg=string} "查询成功"
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

// GetAccountList 分页获取账户列表
// @Tags Account
// @Summary 分页获取账户列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.AccountSearch true "分页获取账户列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /account/getAccountList [get]
func (accountApi *AccountApi) GetAccountList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.AccountSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
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

// GetAccountPublic 不需要鉴权的账户接口
// @Tags Account
// @Summary 不需要鉴权的账户接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /account/getAccountPublic [get]
func (accountApi *AccountApi) GetAccountPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()
	id := c.Query("id")
	reaccount, err := accountService.GetAccountPublic(ctx, id)

	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}

	response.OkWithData(reaccount, c)
}

// GetMyAccount 获取我的账户
// @Tags Account
// @Summary 获取我的账户
// @Security ApiKeyAuth
// @Produce application/json
// @Success 200 {object} response.Response{data=quant.Account,msg=string} "获取成功"
// @Router /account/getMyAccount [get]
func (accountApi *AccountApi) GetMyAccount(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()
	userID := utils.GetUserID(c)
	account, err := accountService.GetMyAccount(ctx, userID)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("未设置证券账户！", c)
		return
	}
	response.OkWithData(account, c)
}
