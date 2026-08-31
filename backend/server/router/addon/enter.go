package addon

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

type RouterGroup struct{ WeishiRouter }

var weishiApi = api.ApiGroupApp.AddonApiGroup.WeishiApi
