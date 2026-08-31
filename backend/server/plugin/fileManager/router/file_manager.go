package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type FileManagerRouter struct{}

func (s *FileManagerRouter) Init(public *gin.RouterGroup, private *gin.RouterGroup) {
	fileManagerRouter := private.Group("fileManager").Use(middleware.OperationRecord())
	fileManagerPublic := public.Group("fileManager")

	{
		fileManagerRouter.POST("upload", apiFileManager.UploadFile)        // 上传文件
		fileManagerPublic.GET("list", apiFileManager.GetFileList)          // 获取文件列表
		fileManagerPublic.GET("file/:id", apiFileManager.GetFile)          // 获取文件信息
		fileManagerRouter.PUT("update", apiFileManager.UpdateFile)         // 更新文件
		fileManagerRouter.DELETE("delete", apiFileManager.DeleteFiles)     // 批量删除文件
		fileManagerPublic.GET("download/:id", apiFileManager.DownloadFile) // 下载文件

		// 分类管理接口
		fileManagerPublic.GET("category/list", apiFileManager.GetCategoryList)  // 获取分类列表（树状）
		fileManagerPublic.POST("category", apiFileManager.CreateCategory)       // 新增分类
		fileManagerPublic.PUT("category", apiFileManager.UpdateCategory)        // 更新分类
		fileManagerPublic.DELETE("category/:id", apiFileManager.DeleteCategory) // 删除分类
	}
}
