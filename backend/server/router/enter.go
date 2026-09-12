package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/router/addon"
	"github.com/flipped-aurora/gin-vue-admin/server/router/bi"
	"github.com/flipped-aurora/gin-vue-admin/server/router/client"
	"github.com/flipped-aurora/gin-vue-admin/server/router/cloud"
	"github.com/flipped-aurora/gin-vue-admin/server/router/cms"
	"github.com/flipped-aurora/gin-vue-admin/server/router/example"
	"github.com/flipped-aurora/gin-vue-admin/server/router/media"
	"github.com/flipped-aurora/gin-vue-admin/server/router/member/member"
	"github.com/flipped-aurora/gin-vue-admin/server/router/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/router/system"
)

var RouterGroupApp = new(RouterGroup)

type RouterGroup struct {
	Addon   addon.RouterGroup
	Bi      bi.RouterGroup
	Client  client.RouterGroup
	Cms     cms.RouterGroup
	Cloud   cloud.RouterGroup
	Quant   quant.RouterGroup
	System  system.RouterGroup
	Example example.RouterGroup
	Member  member.RouterGroup
	Media   media.RouterGroup
}
