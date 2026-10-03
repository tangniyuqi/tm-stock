package quant

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
)

// 本文件守的是 AC-O2 的 DTO 部分：C 端题材接口不得再带地位标识、梯队、相关度，
// 也不得带 AI 或人工的"入选逻辑"——这些是对个股的价值评价，不是客观事实。

// 禁止出现在 C 端 DTO 的 JSON 字段名片段（小写比较）。
var forbiddenDTOStems = []string{"leader", "tier", "relevance", "rank", "score", "target", "confidence", "aireason", "manualreason", "reason", "purity"}

func collectJSONKeys(t reflect.Type, seen map[reflect.Type]bool, out *[]string) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		*out = append(*out, name)
		collectJSONKeys(f.Type, seen, out)
	}
}

func TestClientThemeDTOHasNoEvaluativeFields(t *testing.T) {
	var keys []string
	seen := map[reflect.Type]bool{}
	for _, v := range []any{TreasureResp{}, ThemeDetailResp{}, ThemeStockItem{}, ThemeStockEvidence{}, ThemeRowItem{}, TopThemeItem{}, ThemeFieldGroup{}, SegmentInfo{}} {
		collectJSONKeys(reflect.TypeOf(v), seen, &keys)
	}
	if len(keys) < 20 {
		t.Fatalf("只收集到 %d 个字段，反射范围可能出了问题", len(keys))
	}
	for _, k := range keys {
		low := strings.ToLower(k)
		for _, stem := range forbiddenDTOStems {
			if strings.Contains(low, stem) {
				t.Errorf("C 端 DTO 出现了评价类字段 %q（含 %q）：删掉该字段，不要改名保留", k, stem)
			}
		}
	}
}

func TestMapThemeStockOutputsOnlyObjectiveFacts(t *testing.T) {
	collected := time.Date(2026, 9, 30, 10, 30, 0, 0, time.Local)
	stockID := int64(7)
	stype := int8(quant.ThemeStockSourceAnnualReport)
	pct := 3.21
	name := "甲股份"
	ts := quant.ThemeStock{
		StockId: &stockID, TsCode: "688502.SH", SourceType: &stype,
		SourceExcerpt: "公司光源模组业务收入占比34.2%。", SourceUrl: "https://www.example.com/r.pdf", CollectedAt: &collected,
		Stock: &quant.BaseStock{Name: &name, ChangePct: &pct},
	}
	ts.ID = 42
	item := (&ThemeApi{}).mapThemeStock(ts)

	if item.Key != "42" || item.Id != 7 || item.Name != "甲股份" || item.Code != "688502.SH" || item.Pct != 3.21 {
		t.Errorf("基础字段映射不对：%+v", item)
	}
	ev := item.Evidence
	if ev.SourceType != "年报" || ev.Excerpt != "公司光源模组业务收入占比34.2%。" || ev.Url != "https://www.example.com/r.pdf" || ev.CollectedAt != collected.UnixMilli() {
		t.Errorf("依据映射不对：%+v", ev)
	}
	if item.UpdateAt != collected.Format("2006-01-02 15:04") {
		t.Errorf("依据采集时间映射不对：%q", item.UpdateAt)
	}

	// 输出的 JSON 键集合固定，且不含任何评价类键
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	var top []string
	for k := range m {
		top = append(top, k)
	}
	sort.Strings(top)
	if got := strings.Join(top, ","); got != "code,evidence,id,key,name,pct,updateAt" {
		t.Errorf("C 端个股行的键集合应固定为 code,evidence,id,key,name,pct,updateAt，实际 %s", got)
	}
	for _, bad := range []string{"leader", "tier", "relevance", "aiReason", "manualReason", "reason"} {
		if _, ok := m[bad]; ok {
			t.Errorf("C 端个股行不应出现 %q", bad)
		}
	}
}

func TestMapThemeStockToleratesMissingParts(t *testing.T) {
	ts := quant.ThemeStock{SourceExcerpt: "公司披露了相关业务的进展情况。", SourceUrl: "https://www.example.com/x"}
	item := (&ThemeApi{}).mapThemeStock(ts)
	if item.Name != "" || item.Evidence.SourceType != "" || item.Evidence.CollectedAt != 0 || item.UpdateAt != "" {
		t.Errorf("缺少股票、依据类型与采集时点时应输出空值而不是编造：%+v", item)
	}
	// 未知的依据类型不会被映射成某个看起来合理的名字
	bad := int8(99)
	ts.SourceType = &bad
	if got := (&ThemeApi{}).mapThemeStock(ts).Evidence.SourceType; got != "" {
		t.Errorf("未知依据类型应输出空串，实际 %q", got)
	}
}
