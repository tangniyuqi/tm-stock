package render

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
)

// CurrentSchemaVer 是当前输出结构版本；结构有不兼容变更时递增。
const CurrentSchemaVer = 1

// SectionKey 是报告章节的封闭键；章节标题只能取下表，不接受自定义标题（命名约束，见 ValidateName）。
type SectionKey string

const (
	SecDefinition   SectionKey = "definition"   // 定义（引文块）
	SecTimeline     SectionKey = "timeline"     // 事件时间线
	SecSegment      SectionKey = "segment"      // 环节说明
	SecCompanies    SectionKey = "companies"    // 披露公司
	SecVerification SectionKey = "verification" // 核验结论（L2）
)

var sectionTitles = map[SectionKey]string{
	SecDefinition:   "题材定义",
	SecTimeline:     "事件时间线",
	SecSegment:      "环节说明",
	SecCompanies:    "披露公司",
	SecVerification: "核验结论",
}

// SectionTitle 返回章节的标准标题。
func SectionTitle(k SectionKey) (string, bool) {
	t, ok := sectionTitles[k]
	return t, ok
}

// AILabelText 是显式的"AI 生成"标识文案，由代码注入，只出现在含 AI 抽取内容的章节上。
const AILabelText = "本节内容由 AI 从来源中抽取整理，请以引文与原文链接为准"

// Section 是报告的一章。AILabel 由代码根据节点来源性质设置，Validate 会核对它与实际内容一致。
type Section struct {
	Key     SectionKey `json:"key"`
	Title   string     `json:"title"`
	AILabel bool       `json:"aiLabel"`
	Nodes   []Node     `json:"nodes"`
}

// Disclosure 是报告级的公示信息（标识、模型名称、备案号），由代码注入，不由模型输出。
type Disclosure struct {
	AIGenerated bool   `json:"aiGenerated"`
	ModelName   string `json:"modelName"`
	FilingNo    string `json:"filingNo"`
}

// Report 是 AI 报告的完整输出结构。
type Report struct {
	SchemaVer  int        `json:"schemaVer"`
	Disclosure Disclosure `json:"disclosure"`
	Sections   []Section  `json:"sections"`
}

// Options 是校验选项。
type Options struct {
	// AllowedLinkDomains 是允许出现在报告里的链接域名（含子域名）。为空则报告里不得出现任何链接。
	AllowedLinkDomains []string
	// RequireFiling 为 true 时，含 AI 内容的报告必须带模型备案号（面向公众的 L1、L2 必须；内部 L0 可不要求）。
	RequireFiling bool
}

// bannedNameWords 是产品、栏目、按钮命名的禁用字样（《金融产品网络营销管理办法》第十八条及 requirements 第 4 节）。
// 这是命名约束，不是用户可见文案词表；两者互补，不得互相替代。
var bannedNameWords = []string{
	"证券", "基金", "理财", "投资顾问", "投顾", "咨询", "期货",
	"AI选股", "AI荐股", "智能投顾", "AI分析师", "AI研判", "AI诊股", "荐股", "选股",
}

// ValidateName 检查产品、栏目、按钮等名称是否使用了禁用字样；比较在骨架视图上进行，
// 所以插空格、插符号、全角半角、大小写、繁体字形都挡得住。
func ValidateName(name string) error {
	skel := guard.Skeleton(name)
	for _, w := range bannedNameWords {
		if strings.Contains(skel, guard.Skeleton(w)) {
			return fmt.Errorf("名称 %q 含禁用字样 %q", name, w)
		}
	}
	if guard.Blocked(name) {
		return fmt.Errorf("名称 %q 命中红线检测", name)
	}
	return nil
}

// Validate 校验整份报告。它是报告落库与返回之前的最后一道结构闸门，返回的错误汇总全部问题。
func Validate(r *Report, opt Options) error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if r == nil {
		return errors.New("render: 报告为空")
	}
	if r.SchemaVer != CurrentSchemaVer {
		add("schemaVer = %d，期望 %d", r.SchemaVer, CurrentSchemaVer)
	}

	seen := map[SectionKey]bool{}
	hasAI := false
	for si, s := range r.Sections {
		title, ok := sectionTitles[s.Key]
		switch {
		case !ok:
			add("章节[%d] 的 key %q 不在封闭章节表内", si, s.Key)
			continue
		case seen[s.Key]:
			add("章节 %q 重复", s.Key)
		case s.Title != title:
			add("章节 %q 的标题 %q 不是标准标题 %q", s.Key, s.Title, title)
		}
		seen[s.Key] = true
		if err := ValidateName(title); err != nil {
			add("章节标题：%v", err)
		}
		secAI := false
		for ni, n := range s.Nodes {
			if err := validateNode(n, opt); err != nil {
				add("章节 %q 节点[%d]：%v", s.Key, ni, err)
			}
			if n.Provenance == ProvAIExtractedQuote {
				secAI = true
			}
		}
		if s.AILabel != secAI {
			add("章节 %q 的 AI 标识（%v）与实际内容（含 AI 抽取内容：%v）不符：AI 标识只能落在 AI 抽取的部分", s.Key, s.AILabel, secAI)
		}
		hasAI = hasAI || secAI
	}

	d := r.Disclosure
	if d.AIGenerated != hasAI {
		add("disclosure.aiGenerated = %v，但报告%s含 AI 抽取内容", d.AIGenerated, map[bool]string{true: "", false: "并不"}[hasAI])
	}
	if hasAI {
		if n := utf8.RuneCountInString(d.ModelName); n < 1 || n > 64 || guard.Blocked(d.ModelName) || guard.Normalize(d.ModelName) != d.ModelName {
			add("disclosure.modelName 不合法：须 1–64 个字符、已规范化且无红线命中")
		}
		if opt.RequireFiling && strings.TrimSpace(d.FilingNo) == "" {
			add("面向公众的报告必须公示模型备案号（disclosure.filingNo 为空）")
		}
	}
	return errors.Join(errs...)
}

func validateNode(n Node, opt Options) error {
	if !provenanceSet[n.Provenance] {
		return fmt.Errorf("provenance %q 不在封闭集合内", n.Provenance)
	}
	switch n.Kind {
	case KindTemplate:
		if n.Quote != nil || n.Row != nil {
			return errors.New("模板句节点不得携带引文或数据行")
		}
		re, err := Render(n.Predicate, n.Provenance, n.Args)
		if err != nil {
			return fmt.Errorf("模板句无法重新渲染：%w", err)
		}
		if re.Text != n.Text {
			return fmt.Errorf("模板句文字与按谓词重新渲染的结果不一致（文字被改动过）：%q", n.Text)
		}
	case KindQuote:
		if n.Quote == nil || n.Row != nil || n.Predicate != "" || len(n.Args) > 0 {
			return errors.New("引文块节点结构不合法")
		}
		if n.Provenance == ProvCodeComputed {
			return errors.New("引文块不可能是代码计算的数据")
		}
		if n.Text != n.Quote.Text {
			return errors.New("引文块的 text 与 quote.text 不一致")
		}
		if err := checkQuote(*n.Quote); err != nil {
			return err
		}
		if err := checkLink(n.Quote.URL, opt); err != nil {
			return err
		}
	case KindDataRow:
		if n.Row == nil || n.Quote != nil || n.Predicate != "" || len(n.Args) > 0 || n.Text != "" {
			return errors.New("数据行节点结构不合法")
		}
		if n.Provenance != ProvLibrary {
			return errors.New("数据行的来源性质必须是 library")
		}
		if err := checkRow(*n.Row); err != nil {
			return err
		}
		for _, e := range n.Row.Evidence {
			if err := checkLink(e.URL, opt); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("节点类型 %q 不在白名单内（只允许 template、quote、data_row）", n.Kind)
	}
	return nil
}

// checkLink 校验链接：空链接放行；非空必须是 https 且域名在白名单内（AC-S3）。
func checkLink(raw string, opt Options) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("链接 %q 不合法（只允许不带账号的 https 链接）", raw)
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range opt.AllowedLinkDomains {
		d = strings.ToLower(strings.TrimPrefix(d, "."))
		if d != "" && (host == d || strings.HasSuffix(host, "."+d)) {
			return nil
		}
	}
	return fmt.Errorf("链接域名 %q 不在白名单内", host)
}
