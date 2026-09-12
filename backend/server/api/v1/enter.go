package v1

import (
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/addon"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/bi"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/cloud"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/cms"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/example"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/media"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/member"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/api/v1/system"
)

var ApiGroupApp = new(ApiGroup)

type ApiGroup struct {
	SystemApiGroup  system.ApiGroup
	ExampleApiGroup example.ApiGroup
	AddonApiGroup   addon.ApiGroup
	BiApiGroup      bi.ApiGroup
	CloudApiGroup   cloud.ApiGroup
	QuantApiGroup   quant.ApiGroup
	CmsApiGroup     cms.ApiGroup
	MemberApiGroup  member.ApiGroup
	MediaApiGroup   media.ApiGroup
}
