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

type NewsKeywordApi struct{}

// CreateNewsKeyword 创建关键词
// @Tags NewsKeyword
// @Summary 创建关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.NewsKeyword true "创建关键词"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /newsKeyword/createNewsKeyword [post]
func (newsKeywordApi *NewsKeywordApi) CreateNewsKeyword(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var newsKeyword quant.NewsKeyword
	err := c.ShouldBindJSON(&newsKeyword)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	newsKeyword.CreatedBy = utils.GetUserID(c)
	err = newsKeywordService.CreateNewsKeyword(ctx, &newsKeyword)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteNewsKeyword 删除关键词
// @Tags NewsKeyword
// @Summary 删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.NewsKeyword true "删除关键词"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /newsKeyword/deleteNewsKeyword [delete]
func (newsKeywordApi *NewsKeywordApi) DeleteNewsKeyword(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := newsKeywordService.DeleteNewsKeyword(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteNewsKeywordByIds 批量删除关键词
// @Tags NewsKeyword
// @Summary 批量删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /newsKeyword/deleteNewsKeywordByIds [delete]
func (newsKeywordApi *NewsKeywordApi) DeleteNewsKeywordByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := newsKeywordService.DeleteNewsKeywordByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateNewsKeyword 更新关键词
// @Tags NewsKeyword
// @Summary 更新关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.NewsKeyword true "更新关键词"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /newsKeyword/updateNewsKeyword [put]
func (newsKeywordApi *NewsKeywordApi) UpdateNewsKeyword(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var newsKeyword quant.NewsKeyword
	err := c.ShouldBindJSON(&newsKeyword)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	newsKeyword.UpdatedBy = utils.GetUserID(c)
	err = newsKeywordService.UpdateNewsKeyword(ctx, newsKeyword)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindNewsKeyword 用id查询关键词
// @Tags NewsKeyword
// @Summary 用id查询关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询关键词"
// @Success 200 {object} response.Response{data=quant.NewsKeyword,msg=string} "查询成功"
// @Router /newsKeyword/findNewsKeyword [get]
func (newsKeywordApi *NewsKeywordApi) FindNewsKeyword(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	renewsKeyword, err := newsKeywordService.GetNewsKeyword(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(renewsKeyword, c)
}

// GetNewsKeywordList 分页获取关键词列表
// @Tags NewsKeyword
// @Summary 分页获取关键词列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.NewsKeywordSearch true "分页获取关键词列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /newsKeyword/getNewsKeywordList [get]
func (newsKeywordApi *NewsKeywordApi) GetNewsKeywordList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo quantReq.NewsKeywordSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := newsKeywordService.GetNewsKeywordInfoList(ctx, pageInfo)
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

// GetNewsKeywordPublic 不需要鉴权的关键词接口
// @Tags NewsKeyword
// @Summary 不需要鉴权的关键词接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /newsKeyword/getNewsKeywordPublic [get]
func (newsKeywordApi *NewsKeywordApi) GetNewsKeywordPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	newsKeywordService.GetNewsKeywordPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的关键词接口信息",
	}, "获取成功", c)
}

// IncrementTimes 更新关键词搜索次数
func (newsKeywordApi *NewsKeywordApi) IncrementTimes(c *gin.Context) {
	var req struct {
		Name *string `json:"name" form:"keyword" binding:"omitempty"`
	}

	if err := c.ShouldBind(&req); err != nil {
		return
	}

	if req.Name == nil || *req.Name == "" {
		return
	}

	err := newsKeywordService.IncrementTimes(c, req.Name)

	if err != nil {
		global.GVA_LOG.Error("更新关键词计数失败!", zap.Error(err))
		response.FailWithMessage("更新计数失败:"+err.Error(), c)
		return
	}

	response.OkWithMessage("次数更新成功", c)
}
