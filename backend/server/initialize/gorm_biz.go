package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/addon"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cloud"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	serviceQuant "github.com/flipped-aurora/gin-vue-admin/server/service/quant"
)

func bizModel() error {
	db := global.GVA_DB
	err := db.AutoMigrate(addon.Weishi{}, cloud.Document{}, cloud.Deal{}, cloud.Account{}, cloud.Customer{}, quant.Account{}, quant.Config{}, quant.News{}, quant.Strategy{}, quant.TradeRecord{}, quant.BaseStock{}, quant.Theme{}, quant.ThemeTopic{}, quant.ThemeStock{}, quant.QuantAiTask{}, cms.Ad{}, cms.Article{}, cms.Feedback{}, cms.Page{}, member.MemberAssetLog{}, member.SmsLog{})
	if err != nil {
		return err
	}
	// 启动 AI 定时任务调度器（后台 goroutine 扫描待调度任务）
	serviceQuant.StartAiTaskScheduler()
	return nil
}
