package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type DocumentRouter struct{}

// InitDocumentRouter 初始化 文档 路由信息
func (s *DocumentRouter) InitDocumentRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cloudRouter := Router.Group("cloud")
	cloudRouterWithoutAuth := PublicRouter.Group("cloud")

	documentRouter := cloudRouter.Group("document").Use(middleware.OperationRecord())
	documentRouterWithoutRecord := cloudRouter.Group("document")
	documentRouterWithoutAuth := cloudRouterWithoutAuth.Group("document")
	{
		documentRouter.POST("createDocument", documentApi.CreateDocument)             // 新建文档
		documentRouter.DELETE("deleteDocument", documentApi.DeleteDocument)           // 删除文档
		documentRouter.DELETE("deleteDocumentByIds", documentApi.DeleteDocumentByIds) // 批量删除文档
		documentRouter.PUT("updateDocument", documentApi.UpdateDocument)              // 更新文档
	}
	{
		documentRouterWithoutRecord.GET("findDocument", documentApi.FindDocument)       // 根据ID获取文档
		documentRouterWithoutRecord.GET("getDocumentList", documentApi.GetDocumentList) // 获取文档列表
	}
	{
		documentRouterWithoutAuth.GET("getDocumentPublic", documentApi.GetDocumentPublic) // 文档开放接口
	}
}
