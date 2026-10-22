package initialize

import (
	"context"
	quantAPI "github.com/flipped-aurora/gin-vue-admin/server/api/v1/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/service/feedbackconsole"
	"github.com/gin-gonic/gin"
)

// 仅挂在既有JWT/强制改密/Casbin/DataScope之后，关闭不注册；坏配置拒绝启动。
func registerFeedbackConsole(private *gin.RouterGroup) {
	cfg, err := config.LoadFeedbackConsole()
	if err != nil {
		panic("反馈后台配置无效（详情已隐藏）")
	}
	if !cfg.Enabled {
		return
	}
	if global.GVA_DB == nil {
		panic("反馈后台缺少当前用户数据库")
	}
	if err := feedbackconsole.EnsureCatalog(context.Background(), global.GVA_DB); err != nil {
		panic("反馈后台目录登记失败（未覆盖现有菜单或权限）")
	}
	client, err := feedbackconsole.New(cfg, feedbackconsole.NewAdminIdentity(global.GVA_DB))
	if err != nil {
		panic("反馈后台配置无效（详情已隐藏）")
	}
	quantAPI.NewFeedbackConsoleAPI(client).Register(private)
}
