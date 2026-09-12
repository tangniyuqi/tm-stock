package client

import clientApi "github.com/flipped-aurora/gin-vue-admin/server/api/client"

type RouterGroup struct {
	AuthRouter
	MemberRouter
	NewsRouter
	ThemeRouter
	ThemeTopicRouter
	DictionaryRouter
	DictionaryDetailRouter
}

var (
	authApi             = clientApi.ApiGroupApp.AuthApi
	memberApi           = clientApi.ApiGroupApp.MemberApi
	newsApi             = clientApi.ApiGroupApp.NewsApi
	themeApi            = clientApi.ApiGroupApp.ThemeApi
	themeTopicApi       = clientApi.ApiGroupApp.ThemeTopicApi
	dictionaryApi       = clientApi.ApiGroupApp.DictionaryApi
	dictionaryDetailApi = clientApi.ApiGroupApp.DictionaryDetailApi
)
