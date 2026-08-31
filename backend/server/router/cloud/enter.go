package cloud

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

type RouterGroup struct {
	DocumentRouter
	DealRouter
	AccountRouter
	CustomerRouter
}

var (
	documentApi = api.ApiGroupApp.CloudApiGroup.DocumentApi
	dealApi     = api.ApiGroupApp.CloudApiGroup.DealApi
	accountApi  = api.ApiGroupApp.CloudApiGroup.AccountApi
	customerApi = api.ApiGroupApp.CloudApiGroup.CustomerApi
)
