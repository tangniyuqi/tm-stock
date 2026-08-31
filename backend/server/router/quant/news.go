package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type NewsRouter struct{}

func (s *NewsRouter) InitNewsRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	newsRouter := quantRouter.Group("news").Use(middleware.OperationRecord())
	newsRouterWithoutRecord := quantRouter.Group("news")
	newsRouterWithoutAuth := quantRouterWithoutAuth.Group("news")
	{
		newsRouter.POST("createNews", newsApi.CreateNews)
		newsRouter.DELETE("deleteNews", newsApi.DeleteNews)
		newsRouter.DELETE("deleteNewsByIds", newsApi.DeleteNewsByIds)
		newsRouter.PUT("updateNews", newsApi.UpdateNews)
		newsRouter.POST("fullSyncToMeilisearch", newsApi.FullSyncToMeilisearch)
	}
	{
		newsRouterWithoutRecord.GET("findNews", newsApi.FindNews)
		newsRouterWithoutRecord.GET("meilisearch", newsApi.Meilisearch)
		newsRouterWithoutRecord.GET("checkMeilisearchConsistency", newsApi.CheckMeilisearchConsistency)
	}
	{
		newsRouterWithoutAuth.GET("getNewsList", newsApi.GetNewsList)
		newsRouterWithoutAuth.GET("getNewsPublic", newsApi.GetNewsPublic)
		newsRouterWithoutAuth.POST("createPublic", newsApi.CreatePublic)
	}
}
