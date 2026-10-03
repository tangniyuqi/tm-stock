package quant

import (
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 题材股票的路由守卫：
//  1. 免鉴权（公开）分组里不得有 find、list——它们曾直接复用后台的查询函数，会把草稿、已驳回记录与内部字段
//     （梯队、相关度、AI 入选逻辑）暴露给任何人；
//  2. 不再有 AI 选股/更新入口（梯队与相关度的来源，见 docs/specs/ai-analysis 的 F1–F4）。

func routesOf(t *testing.T, register func(private, public *gin.RouterGroup)) (private, public []string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	priv := engine.Group("/private")
	pub := engine.Group("/public")
	register(priv, pub)
	for _, r := range engine.Routes() {
		line := r.Method + " " + r.Path
		if strings.HasPrefix(r.Path, "/public/") {
			public = append(public, line)
		} else {
			private = append(private, line)
		}
	}
	sort.Strings(private)
	sort.Strings(public)
	return private, public
}

func TestThemeStockRouterNoPublicDataEndpoints(t *testing.T) {
	private, public := routesOf(t, func(priv, pub *gin.RouterGroup) {
		(&ThemeStockRouter{}).InitThemeStockRouter(priv, pub)
	})
	if len(private) == 0 {
		t.Fatal("没有注册任何鉴权路由，测试可能没跑到注册函数")
	}
	for _, r := range public {
		if strings.HasSuffix(r, "/find") || strings.HasSuffix(r, "/list") {
			t.Errorf("公开分组里不得有后台查询路由：%s", r)
		}
		if r != "GET /public/quant/themeStock/getThemeStockPublic" {
			t.Errorf("公开分组只允许保留固定提示的占位接口，实际多出：%s", r)
		}
	}
	for _, r := range private {
		low := strings.ToLower(r)
		if strings.Contains(low, "/ai") {
			t.Errorf("AI 选股/更新入口已下线，不应再有：%s", r)
		}
	}
	// 后台的增删改查仍在（用鉴权路由）
	want := []string{
		"POST /private/quant/themeStock/createThemeStock",
		"PUT /private/quant/themeStock/updateThemeStock",
		"DELETE /private/quant/themeStock/deleteThemeStock",
		"DELETE /private/quant/themeStock/deleteThemeStockByIds",
		"GET /private/quant/themeStock/findThemeStock",
		"GET /private/quant/themeStock/getThemeStockList",
	}
	have := map[string]bool{}
	for _, r := range private {
		have[r] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("缺少后台路由 %s", w)
		}
	}
}
