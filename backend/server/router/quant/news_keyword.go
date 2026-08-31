package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type NewsKeywordRouter struct{}

// InitNewsKeywordRouter 初始化 关键词 路由信息
func (s *NewsKeywordRouter) InitNewsKeywordRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	quantRouter := Router.Group("quant")
	quantRouterWithoutAuth := PublicRouter.Group("quant")

	newsKeywordRouter := quantRouter.Group("newsKeyword").Use(middleware.OperationRecord())
	newsKeywordRouterWithoutRecord := quantRouter.Group("newsKeyword")
	newsKeywordRouterWithoutAuth := quantRouterWithoutAuth.Group("newsKeyword")
	{
		newsKeywordRouter.POST("createNewsKeyword", newsKeywordApi.CreateNewsKeyword)             // 新建关键词
		newsKeywordRouter.DELETE("deleteNewsKeyword", newsKeywordApi.DeleteNewsKeyword)           // 删除关键词
		newsKeywordRouter.DELETE("deleteNewsKeywordByIds", newsKeywordApi.DeleteNewsKeywordByIds) // 批量删除关键词
		newsKeywordRouter.PUT("updateNewsKeyword", newsKeywordApi.UpdateNewsKeyword)              // 更新关键词
		newsKeywordRouter.PUT("incrementTimes", newsKeywordApi.IncrementTimes)                    // 更新关键词次数
	}
	{
		newsKeywordRouterWithoutRecord.GET("findNewsKeyword", newsKeywordApi.FindNewsKeyword)       // 根据ID获取关键词
		newsKeywordRouterWithoutRecord.GET("getNewsKeywordList", newsKeywordApi.GetNewsKeywordList) // 获取关键词列表
	}
	{
		newsKeywordRouterWithoutAuth.GET("getNewsKeywordPublic", newsKeywordApi.GetNewsKeywordPublic) // 关键词开放接口
	}
}
