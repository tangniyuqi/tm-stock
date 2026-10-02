package render

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
)

// badWord 取词表里的第一个词，用来构造含红线词的取值，避免在 .go 源码里直接写出词表词。
func badWord(t testing.TB) string {
	t.Helper()
	for _, n := range guard.RuleNames() {
		if w, ok := strings.CutPrefix(n, "word:"); ok {
			return w
		}
	}
	t.Fatal("词表为空")
	return ""
}

func disclosesArgs() map[string]string {
	return map[string]string{
		"company": "星河科技", "code": "300001", "source_title": "星河科技2025年年度报告",
		"source_type": "年报", "fetched_at": "2026-10-02",
	}
}

func TestRenderEachPredicate(t *testing.T) {
	cases := []struct {
		name string
		p    Predicate
		prov Provenance
		args map[string]string
		want string
	}{
		{"披露", DisclosesBusiness, ProvAIExtractedQuote, disclosesArgs(),
			"星河科技（300001）在《星河科技2025年年度报告》（年报，采集于2026-10-02）中披露："},
		{"目录", ListedInCatalog, ProvLibrary,
			map[string]string{"catalog": "战略性新兴产业分类目录", "subject": "动力电池", "category": "新能源汽车产业"},
			"《战略性新兴产业分类目录》将动力电池列入新能源汽车产业"},
		{"事件", EventOccurred, ProvLibrary,
			map[string]string{"date": "2026-09-30", "title": "关于签订算力服务框架协议的公告", "publisher": "星河科技"},
			"【2026-09-30】关于签订算力服务框架协议的公告（星河科技）"},
		{"涨跌幅", QuoteChange, ProvCodeComputed,
			map[string]string{"company": "星河科技", "code": "300001", "pct": "+3.20", "delay_min": "15", "as_of": "2026-10-02 15:00", "vendor": "示例行情商"},
			"星河科技（300001）涨跌幅 +3.20%（延时 15 分钟，数据时点2026-10-02 15:00，数据来源示例行情商）"},
		{"证据状态", EvidenceState, ProvCodeComputed, map[string]string{"state": "多来源一致"}, "该命题的证据状态：多来源一致"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := Render(c.p, c.prov, c.args)
			if err != nil {
				t.Fatalf("Render 失败：%v", err)
			}
			if n.Text != c.want || n.Kind != KindTemplate || n.Predicate != c.p || n.Provenance != c.prov {
				t.Fatalf("结果不对：%+v", n)
			}
			if guard.Blocked(n.Text) {
				t.Errorf("渲染结果自身命中红线检测：%q", n.Text)
			}
			// 节点持有参数副本：改动原 map 不得影响已渲染节点
			for k := range c.args {
				c.args[k] = "已被篡改"
			}
			if re, err := Render(n.Predicate, n.Provenance, n.Args); err != nil || re.Text != n.Text {
				t.Errorf("节点里保存的参数应能重新渲染出同样的文字：%v", err)
			}
		})
	}
	if len(Predicates()) != len(cases) {
		t.Errorf("谓词库有 %d 个谓词，测试只覆盖了 %d 个", len(Predicates()), len(cases))
	}
}

func TestRenderRejects(t *testing.T) {
	w := badWord(t)
	with := func(k, v string) map[string]string {
		m := disclosesArgs()
		m[k] = v
		return m
	}
	without := func(k string) map[string]string {
		m := disclosesArgs()
		delete(m, k)
		return m
	}
	extra := disclosesArgs()
	extra["comment"] = "额外字段"

	cases := []struct {
		name    string
		p       Predicate
		prov    Provenance
		args    map[string]string
		wantErr error
		redline bool
	}{
		{"未知谓词", Predicate("RECOMMEND"), ProvLibrary, disclosesArgs(), ErrUnknownPredicate, false},
		{"缺字段", DisclosesBusiness, ProvLibrary, without("code"), ErrFieldSet, false},
		{"多字段", DisclosesBusiness, ProvLibrary, extra, ErrFieldSet, false},
		{"字段数对但键名不对", DisclosesBusiness, ProvLibrary, func() map[string]string {
			m := disclosesArgs()
			delete(m, "code")
			m["codee"] = "300001"
			return m
		}(), ErrFieldSet, false},
		{"来源性质不允许（涨跌幅只能代码计算）", QuoteChange, ProvAIExtractedQuote, map[string]string{}, ErrProvenance, false},
		{"来源性质不在集合内", DisclosesBusiness, Provenance("model_opinion"), disclosesArgs(), ErrProvenance, false},
		{"代码不是 6 位数字", DisclosesBusiness, ProvLibrary, with("code", "30001"), nil, false},
		{"代码含字母", DisclosesBusiness, ProvLibrary, with("code", "30000A"), nil, false},
		{"来源类型不在枚举内", DisclosesBusiness, ProvLibrary, with("source_type", "研报"), nil, false},
		{"日期格式不对", DisclosesBusiness, ProvLibrary, with("fetched_at", "2026/10/02"), nil, false},
		{"日期不存在", DisclosesBusiness, ProvLibrary, with("fetched_at", "2026-02-30"), nil, false},
		{"字段含换行", DisclosesBusiness, ProvLibrary, with("company", "星河\n科技"), nil, false},
		{"字段含花括号（防模板注入）", DisclosesBusiness, ProvLibrary, with("company", "星河{code}"), nil, false},
		{"字段为空", DisclosesBusiness, ProvLibrary, with("company", ""), nil, false},
		{"名称过长", DisclosesBusiness, ProvLibrary, with("company", strings.Repeat("星", 33)), nil, false},
		{"来源标题过长", DisclosesBusiness, ProvLibrary, with("source_title", strings.Repeat("标", 201)), nil, false},
		{"字段未规范化（含零宽字符）", DisclosesBusiness, ProvLibrary, with("company", "星河"+string(rune(0x200B))+"科技"), nil, false},
		{"名称字段命中红线", DisclosesBusiness, ProvLibrary, with("company", "星河科技"+w), nil, true},
		{"来源标题命中红线", DisclosesBusiness, ProvLibrary, with("source_title", "关于"+w+"企业的公告"), nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := Render(c.p, c.prov, c.args)
			if err == nil {
				t.Fatalf("应当拒绝，实际渲染出 %+v", n)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("错误类型不对：%v，期望 %v", err, c.wantErr)
			}
			if IsRedline(err) != c.redline {
				t.Fatalf("IsRedline = %v，期望 %v（%v）", IsRedline(err), c.redline, err)
			}
		})
	}
}

func TestRenderPctAndIntFormats(t *testing.T) {
	ok := map[string]string{"company": "星河科技", "code": "300001", "pct": "-0.50", "delay_min": "0", "as_of": "2026-10-02 09:30", "vendor": "示例行情商"}
	if _, err := Render(QuoteChange, ProvCodeComputed, ok); err != nil {
		t.Fatalf("合法取值被拒：%v", err)
	}
	for _, pct := range []string{"3.2", "3", "+3.200", "abc", "", "3,20", "1e2", "+.50", "12345.00"} {
		args := map[string]string{"company": "星河科技", "code": "300001", "pct": pct, "delay_min": "15", "as_of": "2026-10-02 15:00", "vendor": "示例行情商"}
		if _, err := Render(QuoteChange, ProvCodeComputed, args); err == nil {
			t.Errorf("pct=%q 应被拒绝", pct)
		}
	}
	for _, d := range []string{"-1", "1441", "1.5", "015", "abc", "", " 15"} {
		args := map[string]string{"company": "星河科技", "code": "300001", "pct": "+3.20", "delay_min": d, "as_of": "2026-10-02 15:00", "vendor": "示例行情商"}
		if _, err := Render(QuoteChange, ProvCodeComputed, args); err == nil {
			t.Errorf("delay_min=%q 应被拒绝", d)
		}
	}
}

func goodQuote() Quote {
	return Quote{
		SnapshotID: 9, Start: 1024, End: 1180, Text: "星河科技主营算力服务与数据中心运营。",
		SourceTitle: "星河科技2025年年度报告", SourceType: "年报", URL: "https://www.cninfo.com.cn/new/disclosure/x",
		FetchedAt: 1790000000000, ContentHash: "sha256:abc123",
	}
}

func TestQuoteNode(t *testing.T) {
	w := badWord(t)
	n, err := QuoteNode(goodQuote(), ProvAIExtractedQuote)
	if err != nil || n.Kind != KindQuote || n.Text != goodQuote().Text || n.Quote == nil {
		t.Fatalf("合法引文被拒或结果不对：%+v %v", n, err)
	}
	mut := func(f func(*Quote)) Quote { q := goodQuote(); f(&q); return q }
	cases := []struct {
		name string
		q    Quote
		prov Provenance
	}{
		{"来源性质为代码计算", goodQuote(), ProvCodeComputed},
		{"来源性质不在集合内", goodQuote(), Provenance("x")},
		{"空文本", mut(func(q *Quote) { q.Text = "" }), ProvLibrary},
		{"超过 1000 字", mut(func(q *Quote) { q.Text = strings.Repeat("字", 1001) }), ProvLibrary},
		{"未规范化", mut(func(q *Quote) { q.Text = "星河" + string(rune(0x200B)) + "科技主营算力。" }), ProvLibrary},
		{"偏移区间为空", mut(func(q *Quote) { q.End = q.Start }), ProvLibrary},
		{"偏移为负", mut(func(q *Quote) { q.Start = -1 }), ProvLibrary},
		{"缺快照编号", mut(func(q *Quote) { q.SnapshotID = 0 }), ProvLibrary},
		{"缺内容哈希", mut(func(q *Quote) { q.ContentHash = "" }), ProvLibrary},
		{"缺采集时点", mut(func(q *Quote) { q.FetchedAt = 0 }), ProvLibrary},
		{"命中红线且没有人工放行", mut(func(q *Quote) { q.Text = "公司被称为" + w + "。" }), ProvAIExtractedQuote},
		{"人工放行记录缺放行人", mut(func(q *Quote) { q.Text = "公司被称为" + w + "。"; q.Reprint = &Reprint{At: 1} }), ProvAIExtractedQuote},
		{"人工放行记录缺时间", mut(func(q *Quote) { q.Text = "公司被称为" + w + "。"; q.Reprint = &Reprint{By: "审核员甲"} }), ProvAIExtractedQuote},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if n, err := QuoteNode(c.q, c.prov); err == nil {
				t.Fatalf("应当拒绝，实际 %+v", n)
			}
		})
	}
	// 人工放行且留痕：允许展示
	q := goodQuote()
	q.Text = "公司被称为" + w + "。"
	q.Reprint = &Reprint{By: "审核员甲", At: 1790000000000}
	if _, err := QuoteNode(q, ProvAIExtractedQuote); err != nil {
		t.Fatalf("人工放行且留痕的引文应允许：%v", err)
	}
}

func goodRow() StockRow {
	return StockRow{
		Name: "星河科技", Code: "300001", Node: "算力基础设施",
		Evidence: []EvidenceItem{{Type: "年报", Excerpt: "星河科技主营算力服务。", URL: "https://www.cninfo.com.cn/x", FetchedAt: 1790000000000}},
		Change:   &Change{Pct: 3.2, Basis: "收盘价较前收盘价", DelayMin: 15, AsOf: 1790000000000, Vendor: "示例行情商"},
	}
}

func TestRowNode(t *testing.T) {
	n, err := RowNode(goodRow())
	if err != nil || n.Kind != KindDataRow || n.Provenance != ProvLibrary || n.Row == nil {
		t.Fatalf("合法数据行被拒或结果不对：%+v %v", n, err)
	}
	noData := goodRow()
	noData.Change = nil // 没有行情数据：置空，不用 0 代替
	if _, err := RowNode(noData); err != nil {
		t.Fatalf("没有涨跌幅数据的行应允许（Change=nil 表示无数据）：%v", err)
	}
	w := badWord(t)
	mut := func(f func(*StockRow)) StockRow { r := goodRow(); f(&r); return r }
	cases := map[string]StockRow{
		"名称为空":       mut(func(r *StockRow) { r.Name = "" }),
		"代码不是 6 位":   mut(func(r *StockRow) { r.Code = "3001" }),
		"归属环节为空":     mut(func(r *StockRow) { r.Node = "" }),
		"没有依据":       mut(func(r *StockRow) { r.Evidence = nil }),
		"依据缺类型":      mut(func(r *StockRow) { r.Evidence[0].Type = " " }),
		"依据缺采集时点":    mut(func(r *StockRow) { r.Evidence[0].FetchedAt = 0 }),
		"依据摘录未规范化":   mut(func(r *StockRow) { r.Evidence[0].Excerpt = "甲" + string(rune(0x200B)) + "乙" }),
		"依据摘录命中红线":   mut(func(r *StockRow) { r.Evidence[0].Excerpt = "公司被称为" + w }),
		"涨跌幅缺口径":     mut(func(r *StockRow) { r.Change.Basis = "" }),
		"涨跌幅缺来源":     mut(func(r *StockRow) { r.Change.Vendor = "" }),
		"涨跌幅缺数据时点":   mut(func(r *StockRow) { r.Change.AsOf = 0 }),
		"延时为负":       mut(func(r *StockRow) { r.Change.DelayMin = -1 }),
		"涨跌幅不是数字":    mut(func(r *StockRow) { r.Change.Pct = nan() }),
		"涨跌幅低于 -100": mut(func(r *StockRow) { r.Change.Pct = -100.5 }),
		"名称命中红线":     mut(func(r *StockRow) { r.Name = "星河" + w }),
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			if n, err := RowNode(r); err == nil {
				t.Fatalf("应当拒绝，实际 %+v", n)
			}
		})
	}
}

func nan() float64 {
	z := 0.0
	return z / z
}

// AC-G6：个股相关内容只含名称、代码、归属环节、依据、涨跌幅。字段集合是封闭的，
// 有人想加"评价、排名、推荐"维度时，这条测试就是第一道拦截。
func TestStockRowFieldsAreClosed(t *testing.T) {
	names := func(v any) []string {
		rt := reflect.TypeOf(v)
		var out []string
		for i := 0; i < rt.NumField(); i++ {
			out = append(out, rt.Field(i).Name)
		}
		sort.Strings(out)
		return out
	}
	closed := map[string][]string{
		"StockRow":     {"Change", "Code", "Evidence", "Name", "Node"},
		"EvidenceItem": {"Excerpt", "FetchedAt", "Type", "URL"},
		"Change":       {"AsOf", "Basis", "DelayMin", "Pct", "Vendor"},
	}
	got := map[string][]string{"StockRow": names(StockRow{}), "EvidenceItem": names(EvidenceItem{}), "Change": names(Change{})}
	for k, want := range closed {
		if !reflect.DeepEqual(got[k], want) {
			t.Errorf("%s 的字段集合变了：%v，期望 %v。个股相关字段是封闭集合，新增字段须重新过合规评审（AC-G6）", k, got[k], want)
		}
	}
}
