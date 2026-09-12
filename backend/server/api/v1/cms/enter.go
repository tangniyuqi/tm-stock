package cms

import "github.com/flipped-aurora/gin-vue-admin/server/service"

type ApiGroup struct {
	AdApi
	ArticleApi
	FeedbackApi
	PageApi
}

var (
	adService       = service.ServiceGroupApp.CmsServiceGroup.AdService
	articleService  = service.ServiceGroupApp.CmsServiceGroup.ArticleService
	feedbackService = service.ServiceGroupApp.CmsServiceGroup.FeedbackService
	pageService     = service.ServiceGroupApp.CmsServiceGroup.PageService
)
