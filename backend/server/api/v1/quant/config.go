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

type ConfigApi struct{}

// CreateConfig 创建配置
// @Tags Config
// @Summary 创建配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Config true "创建配置"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /config/createConfig [post]
func (configApi *ConfigApi) CreateConfig(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var config quant.Config
	err := c.ShouldBindJSON(&config)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	config.CreatedBy = utils.GetUserID(c)
	err = configService.CreateConfig(ctx, &config)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteConfig 删除配置
// @Tags Config
// @Summary 删除配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Config true "删除配置"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /config/deleteConfig [delete]
func (configApi *ConfigApi) DeleteConfig(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := configService.DeleteConfig(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteConfigByIds 批量删除配置
// @Tags Config
// @Summary 批量删除配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /config/deleteConfigByIds [delete]
func (configApi *ConfigApi) DeleteConfigByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := configService.DeleteConfigByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateConfig 更新配置
// @Tags Config
// @Summary 更新配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Config true "更新配置"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /config/updateConfig [put]
func (configApi *ConfigApi) UpdateConfig(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var config quant.Config
	err := c.ShouldBindJSON(&config)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	config.UpdatedBy = utils.GetUserID(c)
	err = configService.UpdateConfig(ctx, config)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindConfig 用ID查询配置
// @Tags Config
// @Summary 用ID查询配置
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询配置"
// @Success 200 {object} response.Response{data=quant.Config,msg=string} "查询成功"
// @Router /config/findConfig [get]
func (configApi *ConfigApi) FindConfig(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	reconfig, err := configService.GetConfig(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reconfig, c)
}

// GetConfigList 分页获取配置列表
// @Tags Config
// @Summary 分页获取配置列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ConfigSearch true "分页获取配置列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /config/getConfigList [get]
func (configApi *ConfigApi) GetConfigList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.ConfigSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := configService.GetConfigInfoList(ctx, pageInfo)
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

// GetConfigPublic 不需要鉴权的配置接口
// @Tags Config
// @Summary 不需要鉴权的配置接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /config/getConfigPublic [get]
func (configApi *ConfigApi) GetConfigPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	configService.GetConfigPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的配置接口信息",
	}, "获取成功", c)
}
