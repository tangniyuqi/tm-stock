package bi

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

type RouterGroup struct {
	MsgRouter
	ClientRouter
	GroupRouter
	KeywordRouter
	MemberRouter
	FollowRouter
	IcRouter
}

var (
	msgApi     = api.ApiGroupApp.BiApiGroup.MsgApi
	clientApi  = api.ApiGroupApp.BiApiGroup.ClientApi
	groupApi   = api.ApiGroupApp.BiApiGroup.GroupApi
	keywordApi = api.ApiGroupApp.BiApiGroup.KeywordApi
	memberApi  = api.ApiGroupApp.BiApiGroup.MemberApi
	followApi  = api.ApiGroupApp.BiApiGroup.FollowApi
	icApi      = api.ApiGroupApp.BiApiGroup.IcApi
)
