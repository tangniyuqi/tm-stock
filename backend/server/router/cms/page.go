package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type PageRouter struct{}

// InitPageRouter 初始化 单页 路由信息
func (s *PageRouter) InitPageRouter(Router *gin.RouterGroup, PublicRouter *gin.RouterGroup) {
	cmsGroup := Router.Group("cms")
	pageRouter := cmsGroup.Group("page").Use(middleware.OperationRecord())
	pageRouterWithoutRecord := cmsGroup.Group("page")
	pageRouterWithoutAuth := PublicRouter.Group("client").Group("cms").Group("page")
	{
		pageRouter.POST("createPage", pageApi.CreatePage)             // 新建单页
		pageRouter.DELETE("deletePage", pageApi.DeletePage)           // 删除单页
		pageRouter.DELETE("deletePageByIds", pageApi.DeletePageByIds) // 批量删除单页
		pageRouter.PUT("updatePage", pageApi.UpdatePage)              // 更新单页
	}
	{
		pageRouterWithoutRecord.GET("findPage", pageApi.FindPage)       // 根据ID获取单页
		pageRouterWithoutRecord.GET("getPageList", pageApi.GetPageList) // 获取单页列表
	}
	{
		// C 端公开接口：/client/cms/page/detail?name=about|contact
		pageRouterWithoutAuth.GET("detail", pageApi.GetPagePublic) // 单页公开接口
	}
}
