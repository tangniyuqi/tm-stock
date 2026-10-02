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
	// ⚠️ 不要把 quant.ThemeStock{} 加回下面的 AutoMigrate 列表。
	// addon_quant_theme_stock 是 tm-stock 的合规命门表（依据 NOT NULL + CHECK 非空，ADR-0003），
	// 由 server/migrations 里的 SQL 迁移管理（20260730 建表、20261002 收敛）。
	// 历史上这里注册过它：GVA 先启动会建出没有依据约束的旧形态，tm-stock 先建表则会被静默改写
	// （去掉 NOT NULL、丢默认值、多出五个评价类列）。详见 docs/specs/ai-analysis/requirements.md 的 F7。
	// 部署顺序：先执行迁移，再部署本版本 GVA（本版本的 ThemeStock 读写依赖该表已存在）。
	err := db.AutoMigrate(addon.Weishi{}, cloud.Document{}, cloud.Deal{}, cloud.Account{}, cloud.Customer{}, quant.Account{}, quant.Config{}, quant.News{}, quant.Strategy{}, quant.TradeRecord{}, quant.BaseStock{}, quant.Theme{}, quant.ThemeTopic{}, quant.QuantAiTask{}, cms.Ad{}, cms.Article{}, cms.Feedback{}, cms.Page{}, member.MemberAssetLog{}, member.SmsLog{})
	if err != nil {
		return err
	}
	// 启动 AI 定时任务调度器（后台 goroutine 扫描待调度任务）
	serviceQuant.StartAiTaskScheduler()
	return nil
}
