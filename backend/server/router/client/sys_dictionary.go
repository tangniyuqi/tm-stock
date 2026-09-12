package client

import "github.com/gin-gonic/gin"

// DictionaryRouter 注册 C 端字典公开路由（无需登录，只读）
type DictionaryRouter struct{}

// InitDictionaryRouter 注册字典公开路由
func (s *DictionaryRouter) InitDictionaryRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	dictionaryRouter := clientRouter.Group("system")
	{
		dictionaryRouter.GET("dictionary", dictionaryApi.FindByType) // 按字典英名读取字典（含明细）
	}
}

// DictionaryDetailRouter 注册 C 端字典明细公开路由（无需登录，只读）
type DictionaryDetailRouter struct{}

// InitDictionaryDetailRouter 注册字典明细公开路由
func (s *DictionaryDetailRouter) InitDictionaryDetailRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	clientRouter := PublicRouter.Group("client")
	detailRouter := clientRouter.Group("system").Group("dictionaryDetail")
	{
		detailRouter.GET("tree", dictionaryDetailApi.GetTreeByType) // 按字典英名读取字典明细树
	}
}
