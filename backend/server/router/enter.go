package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/router/addon"
	"github.com/flipped-aurora/gin-vue-admin/server/router/bi"
	"github.com/flipped-aurora/gin-vue-admin/server/router/cloud"
	"github.com/flipped-aurora/gin-vue-admin/server/router/example"
	"github.com/flipped-aurora/gin-vue-admin/server/router/media"
	"github.com/flipped-aurora/gin-vue-admin/server/router/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/router/system"
)

var RouterGroupApp = new(RouterGroup)

type RouterGroup struct {
	Addon   addon.RouterGroup
	Bi      bi.RouterGroup
	Cloud   cloud.RouterGroup
	Quant   quant.RouterGroup
	System  system.RouterGroup
	Example example.RouterGroup
	Media   media.RouterGroup
}
