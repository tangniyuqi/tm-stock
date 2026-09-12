package service

import (
	"github.com/flipped-aurora/gin-vue-admin/server/service/addon"
	"github.com/flipped-aurora/gin-vue-admin/server/service/bi"
	"github.com/flipped-aurora/gin-vue-admin/server/service/cloud"
	"github.com/flipped-aurora/gin-vue-admin/server/service/cms"
	"github.com/flipped-aurora/gin-vue-admin/server/service/example"
	"github.com/flipped-aurora/gin-vue-admin/server/service/media"
	"github.com/flipped-aurora/gin-vue-admin/server/service/member"
	"github.com/flipped-aurora/gin-vue-admin/server/service/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
)

var ServiceGroupApp = new(ServiceGroup)

type ServiceGroup struct {
	SystemServiceGroup  system.ServiceGroup
	ExampleServiceGroup example.ServiceGroup
	AddonServiceGroup   addon.ServiceGroup
	BiServiceGroup      bi.ServiceGroup
	CloudServiceGroup   cloud.ServiceGroup
	QuantServiceGroup   quant.ServiceGroup
	CmsServiceGroup     cms.ServiceGroup
	MediaServiceGroup   media.ServiceGroup
	MemberServiceGroup  member.ServiceGroup
}
