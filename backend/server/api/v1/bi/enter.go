package bi

import "github.com/flipped-aurora/gin-vue-admin/server/service"

type ApiGroup struct {
	MsgApi
	ClientApi
	GroupApi
	KeywordApi
	MemberApi
	FollowApi
	IcApi
}

var (
	msgService     = service.ServiceGroupApp.BiServiceGroup.MsgService
	clientService  = service.ServiceGroupApp.BiServiceGroup.ClientService
	groupService   = service.ServiceGroupApp.BiServiceGroup.GroupService
	keywordService = service.ServiceGroupApp.BiServiceGroup.KeywordService
	memberService  = service.ServiceGroupApp.BiServiceGroup.MemberService
	followService  = service.ServiceGroupApp.BiServiceGroup.FollowService
	icService      = service.ServiceGroupApp.BiServiceGroup.IcService
)
