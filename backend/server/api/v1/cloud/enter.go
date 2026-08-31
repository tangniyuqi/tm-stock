package cloud

import "github.com/flipped-aurora/gin-vue-admin/server/service"

type ApiGroup struct {
	DocumentApi
	DealApi
	AccountApi
	CustomerApi
}

var (
	documentService = service.ServiceGroupApp.CloudServiceGroup.DocumentService
	dealService     = service.ServiceGroupApp.CloudServiceGroup.DealService
	accountService  = service.ServiceGroupApp.CloudServiceGroup.AccountService
	customerService = service.ServiceGroupApp.CloudServiceGroup.CustomerService
)
