package cms

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

type RouterGroup struct {
	AdRouter
	ArticleRouter
	FeedbackRouter
	PageRouter
}

var (
	adApi       = api.ApiGroupApp.CmsApiGroup.AdApi
	articleApi  = api.ApiGroupApp.CmsApiGroup.ArticleApi
	feedbackApi = api.ApiGroupApp.CmsApiGroup.FeedbackApi
	pageApi     = api.ApiGroupApp.CmsApiGroup.PageApi
)
