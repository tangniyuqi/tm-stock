package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/router"
	"github.com/gin-gonic/gin"
)

func holder(routers ...*gin.RouterGroup) {
	_ = routers
	_ = router.RouterGroupApp
}
func initBizRouter(routers ...*gin.RouterGroup) {
	privateGroup := routers[0]
	publicGroup := routers[1]
	holder(publicGroup, privateGroup)
	{
		addonRouter := router.RouterGroupApp.Addon
		addonRouter.InitWeishiRouter(privateGroup, publicGroup)
	}
	{
		biRouter := router.RouterGroupApp.Bi
		biRouter.InitMsgRouter(privateGroup, publicGroup)
		biRouter.InitClientRouter(privateGroup, publicGroup)
		biRouter.InitGroupRouter(privateGroup, publicGroup)
		biRouter.InitKeywordRouter(privateGroup, publicGroup)
		biRouter.InitMemberRouter(privateGroup, publicGroup)
		biRouter.InitFollowRouter(privateGroup, publicGroup)
		biRouter.InitIcRouter(privateGroup, publicGroup)
	}
	{
		cloudRouter := router.RouterGroupApp.Cloud
		cloudRouter.InitDocumentRouter(privateGroup, publicGroup)
		cloudRouter.InitDealRouter(privateGroup, publicGroup)
		cloudRouter.InitAccountRouter(privateGroup, publicGroup)
		cloudRouter.InitCustomerRouter(privateGroup, publicGroup)
	}
	{
		quantRouter := router.RouterGroupApp.Quant
		quantRouter.InitAccountRouter(privateGroup, publicGroup)
		quantRouter.InitConfigRouter(privateGroup, publicGroup)
		quantRouter.InitNewsRouter(privateGroup, publicGroup)
		quantRouter.InitStrategyRouter(privateGroup, publicGroup)
		quantRouter.InitTradeRecordRouter(privateGroup, publicGroup)
		quantRouter.InitNewsKeywordRouter(privateGroup, publicGroup)
		quantRouter.InitEventRouter(privateGroup, publicGroup)
		quantRouter.InitReportRouter(privateGroup, publicGroup)
		quantRouter.InitBaseStockRouter(privateGroup, publicGroup)
		quantRouter.InitTradeTaskRouter(privateGroup, publicGroup)
		quantRouter.InitUpdateRouter(privateGroup, publicGroup)
		quantRouter.InitWencaiRouter(privateGroup, publicGroup)
		quantRouter.InitScreenerRecordRouter(privateGroup, publicGroup)
		quantRouter.InitThemeRouter(privateGroup, publicGroup)
		quantRouter.InitThemeTopicRouter(privateGroup, publicGroup)
		quantRouter.InitThemeStockRouter(privateGroup, publicGroup)
		quantRouter.InitAiTaskRouter(privateGroup, publicGroup)
	}

}
