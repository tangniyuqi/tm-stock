package client

import (
	"github.com/flipped-aurora/gin-vue-admin/server/api/client/v1/member"
	"github.com/flipped-aurora/gin-vue-admin/server/api/client/v1/quant"
	"github.com/flipped-aurora/gin-vue-admin/server/api/client/v1/system"
)

// ApiGroup C 端（移动端「题材宝典」）对外接口聚合入口。
//
// 说明：
//   - 本包仅承载「接口设计 / 契约」层，业务逻辑待接入 service 后落地，
//     当前各方法返回占位示例数据，真实字段以 Swagger 注解与各子包 DTO 为准。
//   - 题材 / 快讯接口在 v1/quant 子包，复用现有 quant 模块
//     （Theme / ThemeStock / ThemeTopic / News）；
//     会员 / 鉴权接口在 v1/common 子包（member.go、auth.go），为 C 端新增（模型待建）。
var ApiGroupApp = new(ApiGroup)

type ApiGroup struct {
	member.AuthApi
	member.MemberApi
	quant.ThemeApi
	quant.ThemeTopicApi
	quant.NewsApi
	system.DictionaryApi
	system.DictionaryDetailApi
}

// 路由规划（供引用，最终以 router 层实现为准）：
//
//	Public 组（无需登录）：/client/theme/**、/client/news/**、/client/auth/**
//	Private 组（需登录） ：/client/member/**
