package initialize

import (
	"context"

	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/plugin-tool/utils"
)

func Menu(ctx context.Context) {
	entities := []model.SysBaseMenu{{ParentId: 0, Path: "fileManagerMenu", Name: "fileManagerMenu", Hidden: false, Component: "view/routerHolder.vue", Sort: 0, Meta: model.Meta{Title: "文件管理", Icon: "school"}}, {ParentId: 0, Path: "FMG", Name: "FMG", Hidden: false, Component: "plugin/fileManager/view/index.vue", Sort: 0, Meta: model.Meta{Title: "文件管理", Icon: "files"}}}
	utils.RegisterMenus(entities...)
}
