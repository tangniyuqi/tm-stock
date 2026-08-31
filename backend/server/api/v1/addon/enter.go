package addon

import "github.com/flipped-aurora/gin-vue-admin/server/service"

type ApiGroup struct{ WeishiApi }

var weishiService = service.ServiceGroupApp.AddonServiceGroup.WeishiService
