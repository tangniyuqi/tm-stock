package member

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

type RouterGroup struct {
	MemberRouter
	SmsLogRouter
}

var memberApi = api.ApiGroupApp.MemberApiGroup.MemberApi

var smsLogApi = api.ApiGroupApp.MemberApiGroup.SmsLogApi