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

type KeywordApi struct{}

// CreateKeyword 创建关键词
// @Tags Keyword
// @Summary 创建关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Keyword true "创建关键词"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /keyword/createKeyword [post]
func (keywordApi *KeywordApi) CreateKeyword(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var keyword bi.Keyword
	err := c.ShouldBindJSON(&keyword)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	keyword.CreatedBy = utils.GetUserID(c)
	err = keywordService.CreateKeyword(ctx, &keyword)
	if err != nil {
		global.GVA_LOG.Error("创建失败!", zap.Error(err))
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteKeyword 删除关键词
// @Tags Keyword
// @Summary 删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Keyword true "删除关键词"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /keyword/deleteKeyword [delete]
func (keywordApi *KeywordApi) DeleteKeyword(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := keywordService.DeleteKeyword(ctx, id, userID)
	if err != nil {
		global.GVA_LOG.Error("删除失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteKeywordByIds 批量删除关键词
// @Tags Keyword
// @Summary 批量删除关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /keyword/deleteKeywordByIds [delete]
func (keywordApi *KeywordApi) DeleteKeywordByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := keywordService.DeleteKeywordByIds(ctx, ids, userID)
	if err != nil {
		global.GVA_LOG.Error("批量删除失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateKeyword 更新关键词
// @Tags Keyword
// @Summary 更新关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body bi.Keyword true "更新关键词"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /keyword/updateKeyword [put]
func (keywordApi *KeywordApi) UpdateKeyword(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var keyword bi.Keyword
	err := c.ShouldBindJSON(&keyword)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	keyword.UpdatedBy = utils.GetUserID(c)
	err = keywordService.UpdateKeyword(ctx, keyword)
	if err != nil {
		global.GVA_LOG.Error("更新失败!", zap.Error(err))
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindKeyword 用id查询关键词
// @Tags Keyword
// @Summary 用id查询关键词
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query uint true "用id查询关键词"
// @Success 200 {object} response.Response{data=bi.Keyword,msg=string} "查询成功"
// @Router /keyword/findKeyword [get]
func (keywordApi *KeywordApi) FindKeyword(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	rekeyword, err := keywordService.GetKeyword(ctx, id)
	if err != nil {
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(rekeyword, c)
}

// GetKeywordList 分页获取关键词列表
// @Tags Keyword
// @Summary 分页获取关键词列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query biReq.KeywordSearch true "分页获取关键词列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /keyword/getKeywordList [get]
func (keywordApi *KeywordApi) GetKeywordList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo biReq.KeywordSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := keywordService.GetKeywordInfoList(ctx, pageInfo)
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

// GetKeywordPublic 不需要鉴权的关键词接口
// @Tags Keyword
// @Summary 不需要鉴权的关键词接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /keyword/getKeywordPublic [get]
func (keywordApi *KeywordApi) GetKeywordPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	keywordService.GetKeywordPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的关键词接口信息",
	}, "获取成功", c)
}

// IncrementTimes 更新关键词搜索次数
func (keywordApi *KeywordApi) IncrementTimes(c *gin.Context) {
	var req struct {
		Name *string `json:"name" form:"keyword" binding:"omitempty"`
	}

	if err := c.ShouldBind(&req); err != nil {
		return
	}

	if req.Name == nil || *req.Name == "" {
		return
	}

	err := keywordService.IncrementTimes(c, req.Name)

	if err != nil {
		global.GVA_LOG.Error("更新关键词计数失败!", zap.Error(err))
		response.FailWithMessage("更新计数失败:"+err.Error(), c)
		return
	}

	response.OkWithMessage("次数更新成功", c)
}
