package initialize

import (
	"fmt"
	"path/filepath"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// loadAIConfig 加载独立的 AI 大模型与第三方联网搜索配置（config.ai.yaml）
// 文件与主配置同目录且必存在（缺失将启动报错）；需在 Viper() 之后调用，
// 通过 GVA_VP.ConfigFileUsed() 确定主配置路径；支持 config.ai.yaml 热更新
func loadAIConfig() {
	dir := filepath.Dir(global.GVA_VP.ConfigFileUsed())

	aiViper := viper.New()
	aiViper.SetConfigName("config.ai")
	aiViper.SetConfigType("yaml")
	aiViper.AddConfigPath(dir)
	if err := aiViper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config.ai.yaml file: %w", err))
	}

	unmarshalAIConfig := func() error {
		if err := aiViper.UnmarshalKey("ai", &global.GVA_CONFIG.AI); err != nil {
			return fmt.Errorf("fatal error unmarshal config.ai.yaml ai: %w", err)
		}
		if err := aiViper.UnmarshalKey("web-search", &global.GVA_CONFIG.WebSearch); err != nil {
			return fmt.Errorf("fatal error unmarshal config.ai.yaml web-search: %w", err)
		}
		return nil
	}
	if err := unmarshalAIConfig(); err != nil {
		panic(err)
	}

	aiViper.WatchConfig()
	aiViper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config.ai.yaml file changed:", e.Name)
		if err := unmarshalAIConfig(); err != nil {
			fmt.Println(err)
		}
	})
}
