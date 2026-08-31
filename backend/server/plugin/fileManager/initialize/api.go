package initialize

import (
	"context"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/plugin-tool/utils"
)

func Api(ctx context.Context) {
	entities := []model.SysApi{{Path: "/fileManager/upload", Description: "上传文件", ApiGroup: "文件管理", Method: "POST"}, {Path: "/fileManager/list", Description: "获取文件列表", ApiGroup: "文件管理", Method: "GET"}, {Path: "/fileManager/file/:id", Description: "获取文件信息", ApiGroup: "文件管理", Method: "GET"}, {Path: "/fileManager/update", Description: "更新文件", ApiGroup: "文件管理", Method: "PUT"}, {Path: "/fileManager/delete", Description: "删除文件", ApiGroup: "文件管理", Method: "DELETE"}, {Path: "/fileManager/download/:id", Description: "下载文件", ApiGroup: "文件管理", Method: "GET"}}
	utils.RegisterApis(entities...)
}
