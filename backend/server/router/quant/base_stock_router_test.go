package quant

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	"github.com/gin-gonic/gin"
)

// 基础股票的路由守卫（docs/specs/ai-analysis 的 F15、决策 D16）：
//  1. 不再有 AI 分析入口；
//  2. 免鉴权的开放接口（getBaseStockPublic）曾直接复用后台列表，会把 AI 写的评价性文字原样返回给任何人。
//     现在遗留列在模型上就不序列化，这里用真实请求再验证一遍。

func TestBaseStockRouterHasNoAiEntry(t *testing.T) {
	private, public := routesOf(t, func(priv, pub *gin.RouterGroup) {
		(&BaseStockRouter{}).InitBaseStockRouter(priv, pub)
	})
	if len(private) == 0 {
		t.Fatal("没有注册任何鉴权路由，测试可能没跑到注册函数")
	}
	for _, r := range append(append([]string{}, private...), public...) {
		if strings.Contains(strings.ToLower(r), "/ai") {
			t.Errorf("AI 分析入口已下线，不应再有：%s", r)
		}
	}
	// 后台的增删改查与同步仍在
	want := []string{
		"POST /private/quant/baseStock/createBaseStock",
		"PUT /private/quant/baseStock/updateBaseStock",
		"GET /private/quant/baseStock/findBaseStock",
		"GET /private/quant/baseStock/getBaseStockList",
		"GET /private/quant/baseStock/sync",
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

func TestBaseStockPublicEndpointNeverExposesLegacyAnalysis(t *testing.T) {
	db := testutil.NewMemoryDB(t, &quant.BaseStock{})
	marker := "legacy-analysis-marker-text"
	name := "示例公司"
	now := time.Now()
	row := quant.BaseStock{
		Name:         &name,
		Fundamentals: &marker,
		Financial:    &marker,
		Realization:  &marker,
		Momentum:     &marker,
		Risk:         &marker,
		AiAnalyzedAt: &now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	// 库里确实存着这些历史值（"历史值保留"），否则下面的断言是空转
	var stored quant.BaseStock
	if err := db.First(&stored, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Fundamentals == nil || *stored.Fundamentals != marker || stored.AiAnalyzedAt == nil {
		t.Fatalf("历史值应仍保留在库里：%+v", stored)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	(&BaseStockRouter{}).InitBaseStockRouter(engine.Group("/private"), engine.Group("/public"))

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public/quant/baseStock/getBaseStockPublic?page=1&pageSize=10", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("开放接口应返回 200，实际 %d：%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	var resp struct {
		Code int `json:"code"`
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON：%v\n%s", err, body)
	}
	if resp.Code != 0 || len(resp.Data.List) != 1 {
		t.Fatalf("应返回 1 条记录（否则本测试是空转）：%s", body)
	}
	item := resp.Data.List[0]
	if item["name"] != name {
		t.Errorf("非遗留字段应照常输出：%v", item)
	}
	for _, key := range []string{"fundamentals", "financial", "realization", "momentum", "risk", "ai_analyzed_at"} {
		if _, ok := item[key]; ok {
			t.Errorf("开放接口不应输出遗留列 %q", key)
		}
	}
	if strings.Contains(body, marker) {
		t.Errorf("开放接口不应带出遗留列的内容：%s", body)
	}
}
