package quant

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
)

// 基础股票上的六个 AI 分析遗留列（fundamentals、financial、realization、momentum、risk、ai_analyzed_at）：
// AI 以分析师口吻写的评价性文字，功能已下线（docs/specs/ai-analysis 的 F15、决策 D16）。
// 历史值留在库里，但绝不能再从任何接口带出来（含免鉴权的开放接口），也不能被接口写入。
// 守卫放在模型的序列化标签上，所以无论哪个接口、哪个以后新增的接口返回 BaseStock，都带不出这些列。

var legacyAnalysisKeys = []string{"fundamentals", "financial", "realization", "momentum", "risk", "ai_analyzed_at"}

func TestBaseStockLegacyAnalysisColumnsNeverSerialize(t *testing.T) {
	marker := "legacy-analysis-marker-text"
	now := time.Now()
	name := "示例公司"
	s := quant.BaseStock{
		Name:         &name,
		Fundamentals: &marker,
		Financial:    &marker,
		Realization:  &marker,
		Momentum:     &marker,
		Risk:         &marker,
		AiAnalyzedAt: &now,
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, key := range legacyAnalysisKeys {
		if strings.Contains(out, `"`+key+`"`) {
			t.Errorf("序列化结果不应含有遗留列 %q：%s", key, out)
		}
	}
	if strings.Contains(out, marker) {
		t.Errorf("序列化结果不应带出遗留列的内容：%s", out)
	}
	if !strings.Contains(out, name) {
		t.Errorf("非遗留字段应照常输出（否则本测试是空转）：%s", out)
	}
}

func TestBaseStockLegacyAnalysisColumnsCannotBeWrittenFromJSON(t *testing.T) {
	var in quant.BaseStock
	payload := `{"name":"示例公司","fundamentals":"x","financial":"x","realization":"x","momentum":"x","risk":"x","ai_analyzed_at":"2026-01-01T00:00:00Z"}`
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatal(err)
	}
	if in.Name == nil || *in.Name != "示例公司" {
		t.Fatalf("非遗留字段应能正常绑定：%+v", in.Name)
	}
	if in.Fundamentals != nil || in.Financial != nil || in.Realization != nil || in.Momentum != nil || in.Risk != nil || in.AiAnalyzedAt != nil {
		t.Errorf("接口传入的遗留列不应被写进结构体：%+v", in)
	}
}
