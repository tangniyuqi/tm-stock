package quant

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ThemeApi struct{}

// CreateTheme 创建题材
// @Tags Theme
// @Summary 创建题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Theme true "创建题材"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /theme/createTheme [post]
func (themeApi *ThemeApi) CreateTheme(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var theme quant.Theme
	err := c.ShouldBindJSON(&theme)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	theme.CreatedBy = utils.GetUserID(c)
	err = themeService.CreateTheme(ctx, &theme)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteTheme 删除题材
// @Tags Theme
// @Summary 删除题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Theme true "删除题材"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /theme/deleteTheme [delete]
func (themeApi *ThemeApi) DeleteTheme(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := themeService.DeleteTheme(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteThemeByIds 批量删除题材
// @Tags Theme
// @Summary 批量删除题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /theme/deleteThemeByIds [delete]
func (themeApi *ThemeApi) DeleteThemeByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := themeService.DeleteThemeByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateTheme 更新题材
// @Tags Theme
// @Summary 更新题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.Theme true "更新题材"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /theme/updateTheme [put]
func (themeApi *ThemeApi) UpdateTheme(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var theme quant.Theme
	err := c.ShouldBindJSON(&theme)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	theme.UpdatedBy = utils.GetUserID(c)
	err = themeService.UpdateTheme(ctx, theme)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindTheme 用id查询题材
// @Tags Theme
// @Summary 用id查询题材
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询题材"
// @Success 200 {object} response.Response{data=quant.Theme,msg=string} "查询成功"
// @Router /theme/findTheme [get]
func (themeApi *ThemeApi) FindTheme(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	retheme, err := themeService.GetTheme(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(retheme, c)
}

// GetThemeList 分页获取题材列表,Tree模式下不接受参数
// @Tags Theme
// @Summary 分页获取题材列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /theme/getThemeList [get]
func (themeApi *ThemeApi) GetThemeList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	list, err := themeService.GetThemeList(ctx)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(list, "获取成功", c)
}

// GetThemeListWithoutChildren 根据层级获取题材列表
// @Tags Theme
// @Summary 根据层级获取题材列表
// @Accept application/json
// @Produce application/json
// @Param level query int true "题材层级"
// @Success 200 {object} response.Response{data=[]quant.Theme,msg=string} "获取成功"
// @Router /theme/getThemeListWithoutChildren [get]
func (themeApi *ThemeApi) GetThemeListWithoutChildren(c *gin.Context) {
	ctx := c.Request.Context()

	levelStr := c.Query("level")
	if levelStr == "" {
		response.FailWithMessage("level不能为空", c)
		return
	}

	level, err := strconv.ParseInt(levelStr, 10, 8)
	if err != nil {
		response.FailWithMessage("level必须是有效整数", c)
		return
	}

	list, err := themeService.GetThemeListWithoutChildren(ctx, int8(level))
	if err != nil {
		global.GVA_LOG.Error("按层级查询题材失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(list, "获取成功", c)
}

// GetThemeWithChildren 根据ID获取题材信息及其全部子节点
// @Tags Theme
// @Summary 根据ID获取题材信息及其全部子节点
// @Accept application/json
// @Produce application/json
// @Param id query string true "题材ID"
// @Success 200 {object} response.Response{data=quant.Theme,msg=string} "获取成功"
// @Router /theme/getThemeWithChildren [get]
func (themeApi *ThemeApi) GetThemeWithChildren(c *gin.Context) {
	ctx := c.Request.Context()

	id := c.Query("id")
	if id == "" {
		response.FailWithMessage("id不能为空", c)
		return
	}

	theme, err := themeService.GetThemeWithChildren(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询题材树失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(theme, c)
}

// GetThemePublic 不需要鉴权的题材接口
// @Tags Theme
// @Summary 不需要鉴权的题材接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /theme/getThemePublic [get]
func (themeApi *ThemeApi) GetThemePublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	themeService.GetThemePublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的题材接口信息",
	}, "获取成功", c)
}
