package render

import (
	"strings"
	"testing"
)

var linkOpt = Options{AllowedLinkDomains: []string{"cninfo.com.cn", "sse.com.cn"}, RequireFiling: true}

func mustNode(t *testing.T, n Node, err error) Node {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// goodReport 构造一份含三类节点、含 AI 抽取内容与资料库内容的合法报告。
func goodReport(t *testing.T) *Report {
	t.Helper()
	intro, err := Render(DisclosesBusiness, ProvAIExtractedQuote, disclosesArgs())
	quote, err2 := QuoteNode(goodQuote(), ProvAIExtractedQuote)
	event, err3 := Render(EventOccurred, ProvLibrary,
		map[string]string{"date": "2026-09-30", "title": "关于签订算力服务框架协议的公告", "publisher": "星河科技"})
	row, err4 := RowNode(goodRow())
	chg, err5 := Render(QuoteChange, ProvCodeComputed,
		map[string]string{"company": "星河科技", "code": "300001", "pct": "+3.20", "delay_min": "15", "as_of": "2026-10-02 15:00", "vendor": "示例行情商"})
	return &Report{
		SchemaVer:  CurrentSchemaVer,
		Disclosure: Disclosure{AIGenerated: true, ModelName: "示例模型", FilingNo: "示例备案号-0001"},
		Sections: []Section{
			{Key: SecTimeline, Title: "事件时间线", AILabel: false, Nodes: []Node{mustNode(t, event, err3)}},
			{Key: SecCompanies, Title: "披露公司", AILabel: true, Nodes: []Node{
				mustNode(t, intro, err), mustNode(t, quote, err2), mustNode(t, row, err4), mustNode(t, chg, err5),
			}},
		},
	}
}

func TestValidateAcceptsGoodReport(t *testing.T) {
	if err := Validate(goodReport(t), linkOpt); err != nil {
		t.Fatalf("合法报告被拒：%v", err)
	}
	// 没有 AI 内容的报告：不得声称 AI 生成，也不要求模型名与备案号
	r := goodReport(t)
	r.Sections = r.Sections[:1]
	r.Disclosure = Disclosure{}
	if err := Validate(r, linkOpt); err != nil {
		t.Fatalf("纯资料库报告被拒：%v", err)
	}
	// 内部 L0：不要求备案号
	r = goodReport(t)
	r.Disclosure.FilingNo = ""
	if err := Validate(r, Options{AllowedLinkDomains: linkOpt.AllowedLinkDomains}); err != nil {
		t.Fatalf("不要求备案号时，缺备案号不应被拒：%v", err)
	}
}

// 每条变异都必须被 Validate 拒绝——这是"输出结构校验器拒绝白名单外节点"（AC-G1）的反例集。
func TestValidateRejectsMutations(t *testing.T) {
	w := badWord(t)
	cases := []struct {
		name    string
		mutate  func(*Report)
		wantSub string
	}{
		{"schema 版本不对", func(r *Report) { r.SchemaVer = 99 }, "schemaVer"},
		{"模板句文字被改动", func(r *Report) { r.Sections[0].Nodes[0].Text += "，前景广阔" }, "被改动过"},
		{"模板句参数被改动但文字没改", func(r *Report) { r.Sections[0].Nodes[0].Args["title"] = "另一个标题" }, "被改动过"},
		{"模板句携带引文", func(r *Report) { q := goodQuote(); r.Sections[0].Nodes[0].Quote = &q }, "不得携带"},
		{"白名单外的节点类型（自由段落）", func(r *Report) {
			r.Sections[0].Nodes[0] = Node{Kind: NodeKind("paragraph"), Provenance: ProvAIExtractedQuote, Text: "这家公司前景看起来很不错。"}
		}, "白名单"},
		{"自由文本伪装成模板句", func(r *Report) {
			r.Sections[0].Nodes[0] = Node{Kind: KindTemplate, Provenance: ProvAIExtractedQuote, Predicate: Predicate("FREE_TEXT"), Text: "随便写点什么"}
		}, "重新渲染"},
		{"未知来源性质", func(r *Report) { r.Sections[0].Nodes[0].Provenance = Provenance("model_opinion") }, "provenance"},
		{"引文块文字与 quote.text 不一致", func(r *Report) { r.Sections[1].Nodes[1].Text = "改过的引文" }, "不一致"},
		{"引文块来源性质是代码计算", func(r *Report) { r.Sections[1].Nodes[1].Provenance = ProvCodeComputed }, "代码计算"},
		{"引文块命中红线且无放行记录", func(r *Report) {
			r.Sections[1].Nodes[1].Text = "公司被称为" + w + "。"
			r.Sections[1].Nodes[1].Quote.Text = "公司被称为" + w + "。"
		}, "红线"},
		{"引文块缺证据引用", func(r *Report) { r.Sections[1].Nodes[1].Quote.ContentHash = "" }, "证据引用"},
		{"引文链接域名不在白名单", func(r *Report) { r.Sections[1].Nodes[1].Quote.URL = "https://evil.example.org/x" }, "白名单"},
		{"引文链接是 http", func(r *Report) { r.Sections[1].Nodes[1].Quote.URL = "http://www.cninfo.com.cn/x" }, "不合法"},
		{"引文链接带账号（域名欺骗）", func(r *Report) { r.Sections[1].Nodes[1].Quote.URL = "https://www.cninfo.com.cn@evil.example.org/x" }, "不合法"},
		{"引文链接是域名后缀欺骗", func(r *Report) { r.Sections[1].Nodes[1].Quote.URL = "https://evilcninfo.com.cn/x" }, "白名单"},
		{"数据行没有依据", func(r *Report) { r.Sections[1].Nodes[2].Row.Evidence = nil }, "依据"},
		{"数据行来源性质是 AI", func(r *Report) { r.Sections[1].Nodes[2].Provenance = ProvAIExtractedQuote }, "library"},
		{"数据行带了文字", func(r *Report) { r.Sections[1].Nodes[2].Text = "这是行业领先企业" }, "结构不合法"},
		{"数据行依据链接不在白名单", func(r *Report) { r.Sections[1].Nodes[2].Row.Evidence[0].URL = "https://evil.example.org/x" }, "白名单"},
		{"章节 key 不在封闭表内", func(r *Report) { r.Sections[0].Key = SectionKey("highlights") }, "封闭章节表"},
		{"章节标题被改成自定义标题", func(r *Report) { r.Sections[0].Title = "AI 研判" }, "标准标题"},
		{"章节重复", func(r *Report) { r.Sections = append(r.Sections, r.Sections[0]) }, "重复"},
		{"AI 标识落在没有 AI 内容的章节上", func(r *Report) { r.Sections[0].AILabel = true }, "AI 标识"},
		{"含 AI 抽取内容的章节没有 AI 标识", func(r *Report) { r.Sections[1].AILabel = false }, "AI 标识"},
		{"报告含 AI 内容但没声明 AI 生成", func(r *Report) { r.Disclosure.AIGenerated = false }, "aiGenerated"},
		{"报告没有 AI 内容却声称 AI 生成", func(r *Report) {
			r.Sections = r.Sections[:1]
		}, "aiGenerated"},
		{"缺模型名称", func(r *Report) { r.Disclosure.ModelName = "" }, "modelName"},
		{"模型名称命中红线", func(r *Report) { r.Disclosure.ModelName = "模型" + w }, "modelName"},
		{"面向公众缺备案号", func(r *Report) { r.Disclosure.FilingNo = "  " }, "备案号"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := goodReport(t)
			c.mutate(r)
			err := Validate(r, linkOpt)
			if err == nil {
				t.Fatal("变异后的报告应被拒绝")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("错误信息应包含 %q，实际：%v", c.wantSub, err)
			}
		})
	}
	if err := Validate(nil, linkOpt); err == nil {
		t.Error("nil 报告应被拒绝")
	}
}

func TestValidateLinksWithoutAllowlistAreRejected(t *testing.T) {
	err := Validate(goodReport(t), Options{RequireFiling: true})
	if err == nil || !strings.Contains(err.Error(), "白名单") {
		t.Fatalf("白名单为空时不得出现任何链接：%v", err)
	}
}

func TestValidateReportsAllProblemsAtOnce(t *testing.T) {
	r := goodReport(t)
	r.SchemaVer = 99
	r.Sections[0].Nodes[0].Text += "x"
	r.Disclosure.ModelName = ""
	err := Validate(r, linkOpt)
	if err == nil {
		t.Fatal("应被拒绝")
	}
	for _, sub := range []string{"schemaVer", "被改动过", "modelName"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("错误汇总里缺少 %q：%v", sub, err)
		}
	}
}

func TestValidateName(t *testing.T) {
	bad := []string{
		"AI选股", "AI 选 股", "ai*荐*股", "智能投顾", "ＡＩ分析师", "AI研判", "证券资料库", "基金助手", "理财课堂",
		"投资顾问", "投顾助手", "期货早报", "咨询服务", "智能" + string(rune(0x200B)) + "投顾", "AI診股",
	}
	for _, n := range bad {
		if err := ValidateName(n); err == nil {
			t.Errorf("名称 %q 应被拒绝", n)
		}
	}
	ok := []string{"题材资料简报", "事件时间线", "披露公司", "证据档案", "资料整理", "核验结论"}
	for _, n := range ok {
		if err := ValidateName(n); err != nil {
			t.Errorf("名称 %q 不应被拒绝：%v", n, err)
		}
	}
	if err := ValidateName("题材" + badWord(t)); err == nil {
		t.Error("命中红线检测的名称应被拒绝")
	}
}

// 标准章节标题自己必须合规：别让校验器的白名单本身违反命名约束。
func TestSectionTitlesAreCompliant(t *testing.T) {
	for k := range sectionTitles {
		title, ok := SectionTitle(k)
		if !ok {
			t.Fatalf("章节 %s 没有标题", k)
		}
		if err := ValidateName(title); err != nil {
			t.Errorf("章节标题 %q 不合规：%v", title, err)
		}
	}
	if _, ok := SectionTitle(SectionKey("nope")); ok {
		t.Error("未知章节不应有标题")
	}
}

// AI 标识文案本身也要合规，并且不能是"AI 分析师""AI 研判"一类暗示分析能力的措辞（AC-G8）。
func TestAILabelTextIsCompliant(t *testing.T) {
	if err := ValidateName(AILabelText); err != nil {
		t.Errorf("AI 标识文案不合规：%v", err)
	}
}

func TestLinkAllowlistMatching(t *testing.T) {
	opt := Options{AllowedLinkDomains: []string{"cninfo.com.cn", ".sse.com.cn", ""}}
	for raw, wantOK := range map[string]bool{
		"":                                 true,
		"https://cninfo.com.cn/a":          true,
		"https://www.cninfo.com.cn/a":      true,
		"https://static.sse.com.cn/a":      true,
		"https://sse.com.cn/a":             true,
		"http://cninfo.com.cn/a":           false,
		"https://evilcninfo.com.cn/a":      false,
		"https://cninfo.com.cn.evil.org/a": false,
		"https://user@cninfo.com.cn/a":     false,
		"ftp://cninfo.com.cn/a":            false,
		"//cninfo.com.cn/a":                false,
		"https://":                         false,
		"javascript:alert(1)":              false,
	} {
		err := checkLink(raw, opt)
		if (err == nil) != wantOK {
			t.Errorf("checkLink(%q) err=%v，期望放行=%v", raw, err, wantOK)
		}
	}
}
