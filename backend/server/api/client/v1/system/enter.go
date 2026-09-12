package system

import "github.com/flipped-aurora/gin-vue-admin/server/service"

// ApiGroup C 端字典相关接口聚合入口（免登录，只读）。
type ApiGroup struct {
	DictionaryApi
	DictionaryDetailApi
}

var (
	dictionaryService       = service.ServiceGroupApp.SystemServiceGroup.DictionaryService
	dictionaryDetailService = service.ServiceGroupApp.SystemServiceGroup.DictionaryDetailService
)
