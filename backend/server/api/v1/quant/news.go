package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	quantService "github.com/flipped-aurora/gin-vue-admin/server/service/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type NewsApi struct{}

// CreateNews 创建快讯
// @Tags News
// @Summary 创建快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.News true "创建快讯"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /news/createNews [post]
func (newsApi *NewsApi) CreateNews(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var news quant.News
	err := c.ShouldBindJSON(&news)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	news.CreatedBy = utils.GetUserID(c)
	err = newsService.CreateNews(ctx, &news)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteNews 删除快讯
// @Tags News
// @Summary 删除快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.News true "删除快讯"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /news/deleteNews [delete]
func (newsApi *NewsApi) DeleteNews(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := newsService.DeleteNews(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteNewsByIds 批量删除快讯
// @Tags News
// @Summary 批量删除快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /news/deleteNewsByIds [delete]
func (newsApi *NewsApi) DeleteNewsByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := newsService.DeleteNewsByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateNews 更新快讯
// @Tags News
// @Summary 更新快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body quant.News true "更新快讯"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /news/updateNews [put]
func (newsApi *NewsApi) UpdateNews(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var news quant.News
	err := c.ShouldBindJSON(&news)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	news.UpdatedBy = utils.GetUserID(c)
	err = newsService.UpdateNews(ctx, news)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindNews 用ID查询快讯
// @Tags News
// @Summary 用ID查询快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用ID查询快讯"
// @Success 200 {object} response.Response{data=quant.News,msg=string} "查询成功"
// @Router /news/findNews [get]
func (newsApi *NewsApi) FindNews(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	renews, err := newsService.GetNews(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(renews, c)
}

// GetNewsList 分页获取快讯列表
// @Tags News
// @Summary 分页获取快讯列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.NewsSearch true "分页获取快讯列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /news/getNewsList [get]
func (newsApi *NewsApi) GetNewsList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()
	var pageInfo quantReq.NewsSearch

	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	list, total, err := newsService.GetNewsInfoList(ctx, pageInfo)
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

// GetNewsPublic 不需要鉴权的快讯接口
// @Tags News
// @Summary 不需要鉴权的快讯接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /news/getNewsPublic [get]
func (newsApi *NewsApi) GetNewsPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	newsService.GetNewsPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的快讯接口信息",
	}, "获取成功", c)
}

// CreatePublic 创建快讯（不鉴权）
// @Tags News
// @Summary 创建快讯（不鉴权）
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.NewsSearch true "成功"
// @Success 200 {object} response.Response{data=object,msg=string} "成功"
// @Router /news/createPublic [POST]
func (newsApi *NewsApi) CreatePublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var news quant.News
	err := c.ShouldBindJSON(&news)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	err = newsService.CreatePublic(ctx, &news)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}


// Meilisearch 使用 Meilisearch 搜索快讯
// @Tags News
// @Summary 使用 Meilisearch 搜索快讯
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query quantReq.NewsSearch true "搜索快讯"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "搜索成功"
// @Router /news/meilisearch [get]
func (newsApi *NewsApi) Meilisearch(c *gin.Context) {
	var pageInfo quantReq.NewsSearch
	
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 参数验证
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize < 1 || pageInfo.PageSize > 100 {
		pageInfo.PageSize = 10
	}
	
	// 如果搜索关键词为空，使用原有的 GetNewsList
	if pageInfo.Keyword == nil || *pageInfo.Keyword == "" {
		newsApi.GetNewsList(c)
		return
	}
	
	// 检查 Meilisearch 服务是否可用
	if global.GVA_MEILISEARCH == nil {
		global.GVA_LOG.Warn("Meilisearch service not available, falling back to MySQL")
		// 降级到原有的 MySQL 搜索
		newsApi.GetNewsList(c)
		return
	}
	
	// 执行搜索
	meilisearchService, ok := global.GVA_MEILISEARCH.(*quantService.MeilisearchService)
	if !ok {
		global.GVA_LOG.Error("Failed to cast Meilisearch service")
		response.FailWithMessage("搜索服务不可用", c)
		return
	}
	
	result, err := meilisearchService.SearchNews(pageInfo)
	if err != nil {
		global.GVA_LOG.Error("搜索失败!", zap.Error(err))
		response.FailWithMessage("搜索失败: "+err.Error(), c)
		return
	}
	
	response.OkWithDetailed(response.PageResult{
		List:     result.List,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	}, "搜索成功", c)
}

// FullSyncToMeilisearch 全量同步历史数据到 Meilisearch
// @Tags News
// @Summary 全量同步历史数据到 Meilisearch
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "同步成功"
// @Router /news/fullSyncToMeilisearch [post]
func (newsApi *NewsApi) FullSyncToMeilisearch(c *gin.Context) {
	// 检查 Meilisearch 服务是否可用
	if global.GVA_MEILISEARCH == nil {
		response.FailWithMessage("Meilisearch 服务不可用", c)
		return
	}
	
	// 执行全量同步
	meilisearchService, ok := global.GVA_MEILISEARCH.(*quantService.MeilisearchService)
	if !ok {
		global.GVA_LOG.Error("Failed to cast Meilisearch service")
		response.FailWithMessage("搜索服务不可用", c)
		return
	}
	
	global.GVA_LOG.Info("开始全量同步数据到 Meilisearch")
	result, err := meilisearchService.FullSyncToMeilisearch()
	if err != nil {
		global.GVA_LOG.Error("全量同步失败!", zap.Error(err))
		response.FailWithMessage("全量同步失败: "+err.Error(), c)
		return
	}
	
	response.OkWithDetailed(gin.H{
		"total":   result.Total,
		"success": result.Success,
		"failure": result.Failure,
	}, "全量同步完成", c)
}

// CheckMeilisearchConsistency 检查 MySQL 和 Meilisearch 数据一致性
// @Tags News
// @Summary 检查 MySQL 和 Meilisearch 数据一致性
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "检查成功"
// @Router /news/checkMeilisearchConsistency [get]
func (newsApi *NewsApi) CheckMeilisearchConsistency(c *gin.Context) {
	// 检查 Meilisearch 服务是否可用
	if global.GVA_MEILISEARCH == nil {
		response.FailWithMessage("Meilisearch 服务不可用", c)
		return
	}
	
	// 执行一致性检查
	meilisearchService, ok := global.GVA_MEILISEARCH.(*quantService.MeilisearchService)
	if !ok {
		global.GVA_LOG.Error("Failed to cast Meilisearch service")
		response.FailWithMessage("搜索服务不可用", c)
		return
	}
	
	result, err := meilisearchService.CheckDataConsistency()
	if err != nil {
		global.GVA_LOG.Error("一致性检查失败!", zap.Error(err))
		response.FailWithMessage("一致性检查失败: "+err.Error(), c)
		return
	}
	
	message := "数据一致"
	if !result.IsConsistent {
		message = "数据不一致，建议执行全量同步"
	}
	
	response.OkWithDetailed(gin.H{
		"mysql_count":       result.MySQLCount,
		"meilisearch_count": result.MeilisearchCount,
		"is_consistent":     result.IsConsistent,
		"difference":        result.Difference,
	}, message, c)
}
