package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type ArticleRouter struct{}

// InitArticleRouter 初始化 文章 路由信息
func (s *ArticleRouter) InitArticleRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cmsGroup := Router.Group("cms")
	articleRouter := cmsGroup.Group("article").Use(middleware.OperationRecord())
	articleRouterWithoutRecord := cmsGroup.Group("article")
	articleRouterWithoutAuth := PublicRouter.Group("client").Group("cms").Group("article")
	{
		articleRouter.POST("createArticle", articleApi.CreateArticle)             // 新建文章
		articleRouter.DELETE("deleteArticle", articleApi.DeleteArticle)           // 删除文章
		articleRouter.DELETE("deleteArticleByIds", articleApi.DeleteArticleByIds) // 批量删除文章
		articleRouter.PUT("updateArticle", articleApi.UpdateArticle)              // 更新文章
	}
	{
		articleRouterWithoutRecord.GET("findArticle", articleApi.FindArticle)       // 根据ID获取文章
		articleRouterWithoutRecord.GET("getArticleList", articleApi.GetArticleList) // 获取文章列表
	}
	{
		// C 端公开接口：/client/cms/article/list?cateId=0（帮助中心）
		articleRouterWithoutAuth.GET("list", articleApi.GetArticlePublic) // 文章公开接口
	}
}
