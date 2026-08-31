package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/service/quant"
	"go.uber.org/zap"
)

// InitMeilisearch 初始化 Meilisearch 客户端
func InitMeilisearch() {
	if !global.GVA_CONFIG.Meilisearch.Enable {
		return
	}
	
	meilisearchService := &quant.MeilisearchService{}
	
	// 初始化客户端
	if err := meilisearchService.InitializeMeilisearch(); err != nil {
		global.GVA_LOG.Error("Failed to initialize Meilisearch", zap.Error(err))
		// 不阻塞系统启动，仅记录错误
		return
	}
	
	// 创建新闻索引
	if err := meilisearchService.CreateNewsIndex(); err != nil {
		global.GVA_LOG.Error("Failed to create news index", zap.Error(err))
		// 不阻塞系统启动，仅记录错误
		return
	}
	
	// 将服务实例保存到全局变量
	global.GVA_MEILISEARCH = meilisearchService
	
	global.GVA_LOG.Info("Meilisearch initialized successfully")
}
