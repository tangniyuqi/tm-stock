package cms

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type ArticleApi struct{}

// CreateArticle 创建文章
// @Tags Article
// @Summary 创建文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Article true "创建文章"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /article/createArticle [post]
func (articleApi *ArticleApi) CreateArticle(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var article cms.Article
	err := c.ShouldBindJSON(&article)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	article.CreatedBy = utils.GetUserID(c)
	err = articleService.CreateArticle(ctx, &article)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("创建失败!")
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteArticle 删除文章
// @Tags Article
// @Summary 删除文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Article true "删除文章"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /article/deleteArticle [delete]
func (articleApi *ArticleApi) DeleteArticle(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := articleService.DeleteArticle(ctx, id, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("删除失败!")
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteArticleByIds 批量删除文章
// @Tags Article
// @Summary 批量删除文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /article/deleteArticleByIds [delete]
func (articleApi *ArticleApi) DeleteArticleByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := articleService.DeleteArticleByIds(ctx, ids, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("批量删除失败!")
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateArticle 更新文章
// @Tags Article
// @Summary 更新文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Article true "更新文章"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /article/updateArticle [put]
func (articleApi *ArticleApi) UpdateArticle(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var article cms.Article
	err := c.ShouldBindJSON(&article)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	article.UpdatedBy = utils.GetUserID(c)
	err = articleService.UpdateArticle(ctx, article)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("更新失败!")
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindArticle 用id查询文章
// @Tags Article
// @Summary 用id查询文章
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询文章"
// @Success 200 {object} response.Response{data=cms.Article,msg=string} "查询成功"
// @Router /article/findArticle [get]
func (articleApi *ArticleApi) FindArticle(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	rearticle, err := articleService.GetArticle(ctx, id)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("查询失败!")
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(rearticle, c)
}

// GetArticleList 分页获取文章列表
// @Tags Article
// @Summary 分页获取文章列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cmsReq.ArticleSearch true "分页获取文章列表"
// @Success 200 {object} response.Response{data=response.PageResult{list=[]cms.Article},msg=string} "获取成功"
// @Router /article/getArticleList [get]
func (articleApi *ArticleApi) GetArticleList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo cmsReq.ArticleSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := articleService.GetArticleInfoList(ctx, pageInfo)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("获取失败!")
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

// GetArticlePublic 不需要鉴权的文章列表接口（按分类获取，用于帮助中心等）
// @Tags Article
// @Summary 不需要鉴权的文章列表接口
// @Accept application/json
// @Produce application/json
// @Param cateId query int false "分类ID（可选，不传则返回全部）"
// @Success 200 {object} response.Response{data=[]cms.Article,msg=string} "获取成功"
// @Router /article/getArticlePublic [get]
func (articleApi *ArticleApi) GetArticlePublic(c *gin.Context) {
	ctx := c.Request.Context()
	cateID := int32(0)
	if cid := c.Query("cateId"); cid != "" {
		var parsed int64
		parsed, _ = strconv.ParseInt(cid, 10, 32)
		cateID = int32(parsed)
	}
	list, err := articleService.GetArticleListPublic(ctx, cateID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("获取文章列表失败!")
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithData(list, c)
}
