package member

import "github.com/flipped-aurora/gin-vue-admin/server/service"

type ApiGroup struct {
	MemberApi
	SmsLogApi
}

var memberService = service.ServiceGroupApp.MemberServiceGroup.MemberService

var smsService = service.ServiceGroupApp.MemberServiceGroup.SmsService