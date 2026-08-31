package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type KeywordRouter struct{}

// InitKeywordRouter 初始化 关键词 路由信息
func (s *KeywordRouter) InitKeywordRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	biRouter := Router.Group("bi")
	biRouterWithoutAuth := PublicRouter.Group("bi")

	keywordRouter := biRouter.Group("keyword").Use(middleware.OperationRecord())
	keywordRouterWithoutRecord := biRouter.Group("keyword")
	keywordRouterWithoutAuth := biRouterWithoutAuth.Group("keyword")
	{
		keywordRouter.POST("createKeyword", keywordApi.CreateKeyword)             // 新建关键词
		keywordRouter.DELETE("deleteKeyword", keywordApi.DeleteKeyword)           // 删除关键词
		keywordRouter.DELETE("deleteKeywordByIds", keywordApi.DeleteKeywordByIds) // 批量删除关键词
		keywordRouter.PUT("updateKeyword", keywordApi.UpdateKeyword)              // 更新关键词
		keywordRouter.PUT("incrementTimes", keywordApi.IncrementTimes)            // 更新关键词次数
	}
	{
		keywordRouterWithoutRecord.GET("findKeyword", keywordApi.FindKeyword)       // 根据ID获取关键词
		keywordRouterWithoutRecord.GET("getKeywordList", keywordApi.GetKeywordList) // 获取关键词列表
	}
	{
		keywordRouterWithoutAuth.GET("getKeywordPublic", keywordApi.GetKeywordPublic) // 关键词开放接口
	}
}
