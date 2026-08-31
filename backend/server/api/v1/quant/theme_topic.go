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

type ThemeTopicApi struct{}

// CreateThemeTopic 创建题材话题
// @Tags ThemeTopic
// @Summary 创建题材话题
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ThemeTopic true "创建题材话题"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /themeTopic/createThemeTopic [post]
func (themeTopicApi *ThemeTopicApi) CreateThemeTopic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var themeTopic quant.ThemeTopic
	err := c.ShouldBindJSON(&themeTopic)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	themeTopic.CreatedBy = utils.GetUserID(c)
	err = themeTopicService.CreateThemeTopic(ctx, &themeTopic)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteThemeTopic 删除题材话题
// @Tags ThemeTopic
// @Summary 删除题材话题
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ThemeTopic true "删除题材话题"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /themeTopic/deleteThemeTopic [delete]
func (themeTopicApi *ThemeTopicApi) DeleteThemeTopic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := themeTopicService.DeleteThemeTopic(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteThemeTopicByIds 批量删除题材话题
// @Tags ThemeTopic
// @Summary 批量删除题材话题
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /themeTopic/deleteThemeTopicByIds [delete]
func (themeTopicApi *ThemeTopicApi) DeleteThemeTopicByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := themeTopicService.DeleteThemeTopicByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateThemeTopic 更新题材话题
// @Tags ThemeTopic
// @Summary 更新题材话题
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.ThemeTopic true "更新题材话题"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /themeTopic/updateThemeTopic [put]
func (themeTopicApi *ThemeTopicApi) UpdateThemeTopic(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var themeTopic quant.ThemeTopic
	err := c.ShouldBindJSON(&themeTopic)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	themeTopic.UpdatedBy = utils.GetUserID(c)
	err = themeTopicService.UpdateThemeTopic(ctx, themeTopic)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindThemeTopic 用id查询题材话题
// @Tags ThemeTopic
// @Summary 用id查询题材话题
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询题材话题"
// @Success 200 {object} response.Response{data=quant.ThemeTopic,msg=string} "查询成功"
// @Router /themeTopic/findThemeTopic [get]
func (themeTopicApi *ThemeTopicApi) FindThemeTopic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	reThemeTopic, err := themeTopicService.GetThemeTopic(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(reThemeTopic, c)
}

// GetThemeTopicList 分页获取题材话题列表
// @Tags ThemeTopic
// @Summary 分页获取题材话题列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.ThemeTopicSearch true "分页获取题材话题列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /themeTopic/getThemeTopicList [get]
func (themeTopicApi *ThemeTopicApi) GetThemeTopicList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.ThemeTopicSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := themeTopicService.GetThemeTopicInfoList(ctx, pageInfo)
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

// GetThemeTopicPublic 不需要鉴权的题材话题接口
// @Tags ThemeTopic
// @Summary 不需要鉴权的题材话题接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /themeTopic/getThemeTopicPublic [get]
func (themeTopicApi *ThemeTopicApi) GetThemeTopicPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	themeTopicService.GetThemeTopicPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的题材话题接口信息",
	}, "获取成功", c)
}
