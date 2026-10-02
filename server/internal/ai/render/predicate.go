package render

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
)

// Predicate 是封闭谓词库里的谓词。新增谓词必须同时补模板、字段规格与测试，并重新过合规评审。
type Predicate string

const (
	DisclosesBusiness Predicate = "DISCLOSES_BUSINESS" // 某公司在某来源中披露（其后跟引文块）
	ListedInCatalog   Predicate = "LISTED_IN_CATALOG"  // 官方目录将某对象列入某类别
	EventOccurred     Predicate = "EVENT_OCCURRED"     // 某日发生某事件（标题为来源原文）
	QuoteChange       Predicate = "QUOTE_CHANGE"       // 涨跌幅事实（含口径、延时、数据时点、来源）
	EvidenceState     Predicate = "EVIDENCE_STATE"     // 命题的证据状态枚举
)

// SourceTypes 是来源类型的封闭枚举，与 addon_quant_theme_stock.source_type 的映射一致。
var SourceTypes = []string{"公告", "年报", "招股书", "官方产业目录", "互动易问答"}

// EvidenceStates 是证据状态的封闭枚举（design.md 3.3）。
var EvidenceStates = []string{"权威披露", "多来源一致", "存在冲突", "仅单一来源", "证据不足"}

type fieldKind int

const (
	fName     fieldKind = iota // 公司、对象名称：1–32 个字符，单行，无红线命中
	fCode                      // 6 位证券代码
	fText                      // 来源原文片段（标题、目录名、类别）：1–200 个字符，单行，已规范化，无红线命中
	fEnum                      // 封闭枚举
	fDate                      // YYYY-MM-DD
	fDateTime                  // YYYY-MM-DD HH:MM
	fPct                       // 带符号两位小数，如 +3.20、-0.50
	fInt                       // 0 到 1440 的整数（分钟）
)

type fieldSpec struct {
	name string
	kind fieldKind
	enum []string
}

type predicateSpec struct {
	pattern string // 含 {字段名} 占位符；占位符与字段规格必须一一对应
	fields  []fieldSpec
	// provenances 是该谓词允许的来源性质。涨跌幅与证据状态只能由代码计算。
	provenances []Provenance
}

// Provenance 是节点的来源性质（AC-G8）。
type Provenance string

const (
	ProvLibrary          Provenance = "library"            // 资料库人工审核内容
	ProvAIExtractedQuote Provenance = "ai_extracted_quote" // AI 抽取的引文及据此整理的句子
	ProvCodeComputed     Provenance = "code_computed"      // 代码计算的数据
)

var provenanceSet = map[Provenance]bool{ProvLibrary: true, ProvAIExtractedQuote: true, ProvCodeComputed: true}

var specs = map[Predicate]predicateSpec{
	DisclosesBusiness: {
		pattern: "{company}（{code}）在《{source_title}》（{source_type}，采集于{fetched_at}）中披露：",
		fields: []fieldSpec{
			{"company", fName, nil}, {"code", fCode, nil}, {"source_title", fText, nil},
			{"source_type", fEnum, SourceTypes}, {"fetched_at", fDate, nil},
		},
		provenances: []Provenance{ProvLibrary, ProvAIExtractedQuote},
	},
	ListedInCatalog: {
		pattern: "《{catalog}》将{subject}列入{category}",
		fields: []fieldSpec{
			{"catalog", fText, nil}, {"subject", fName, nil}, {"category", fText, nil},
		},
		provenances: []Provenance{ProvLibrary, ProvAIExtractedQuote},
	},
	EventOccurred: {
		pattern: "【{date}】{title}（{publisher}）",
		fields: []fieldSpec{
			{"date", fDate, nil}, {"title", fText, nil}, {"publisher", fName, nil},
		},
		provenances: []Provenance{ProvLibrary, ProvAIExtractedQuote},
	},
	QuoteChange: {
		pattern: "{company}（{code}）涨跌幅 {pct}%（延时 {delay_min} 分钟，数据时点{as_of}，数据来源{vendor}）",
		fields: []fieldSpec{
			{"company", fName, nil}, {"code", fCode, nil}, {"pct", fPct, nil},
			{"delay_min", fInt, nil}, {"as_of", fDateTime, nil}, {"vendor", fName, nil},
		},
		provenances: []Provenance{ProvCodeComputed},
	},
	EvidenceState: {
		pattern: "该命题的证据状态：{state}",
		fields: []fieldSpec{
			{"state", fEnum, EvidenceStates},
		},
		provenances: []Provenance{ProvCodeComputed},
	},
}

var placeholderRe = regexp.MustCompile(`\{([a-z_]+)\}`)

func init() {
	// 模板与字段规格必须一一对应：缺一个占位符或多一个字段都在启动时直接失败，不让半成品模板上线。
	for p, s := range specs {
		inPattern := map[string]bool{}
		for _, m := range placeholderRe.FindAllStringSubmatch(s.pattern, -1) {
			inPattern[m[1]] = true
		}
		inFields := map[string]bool{}
		for _, f := range s.fields {
			if inFields[f.name] {
				panic(fmt.Sprintf("render: 谓词 %s 的字段 %s 重复", p, f.name))
			}
			inFields[f.name] = true
		}
		for n := range inPattern {
			if !inFields[n] {
				panic(fmt.Sprintf("render: 谓词 %s 的模板占位符 {%s} 没有字段规格", p, n))
			}
		}
		for n := range inFields {
			if !inPattern[n] {
				panic(fmt.Sprintf("render: 谓词 %s 的字段 %s 没有出现在模板里", p, n))
			}
		}
		if len(s.provenances) == 0 {
			panic(fmt.Sprintf("render: 谓词 %s 没有声明允许的来源性质", p))
		}
	}
}

// Predicates 返回全部谓词（供测试与运营展示）。
func Predicates() []Predicate {
	out := make([]Predicate, 0, len(specs))
	for p := range specs {
		out = append(out, p)
	}
	return out
}

var (
	codeRe = regexp.MustCompile(`^[0-9]{6}$`)
	pctRe  = regexp.MustCompile(`^[+-]?[0-9]{1,4}\.[0-9]{2}$`)
)

// FieldError 说明哪个字段为什么不合法。
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("字段 %s 不合法：%s", e.Field, e.Reason)
}

// 红线命中单独成类：调用方应据此走"转人工"，而不是当作普通格式错误丢弃。
const reasonRedline = "命中红线检测"

// IsRedline 判断错误是否由红线命中引起。
func IsRedline(err error) bool {
	fe, ok := err.(*FieldError)
	return ok && fe.Reason == reasonRedline
}

func validateField(f fieldSpec, v string) *FieldError {
	bad := func(reason string) *FieldError { return &FieldError{Field: f.name, Reason: reason} }
	switch f.kind {
	case fName, fText:
		limit := 32
		if f.kind == fText {
			limit = 200
		}
		if n := utf8.RuneCountInString(v); n < 1 || n > limit {
			return bad(fmt.Sprintf("长度须在 1 到 %d 个字符之间", limit))
		}
		if strings.ContainsAny(v, "\n\r{}") {
			return bad("不得含换行与花括号")
		}
		if guard.Normalize(v) != v {
			return bad("必须是规范化后的文本")
		}
		if guard.Blocked(v) {
			return bad(reasonRedline)
		}
	case fCode:
		if !codeRe.MatchString(v) {
			return bad("必须是 6 位数字代码")
		}
	case fEnum:
		for _, e := range f.enum {
			if v == e {
				return nil
			}
		}
		return bad("不在封闭枚举内")
	case fDate:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return bad("必须是 YYYY-MM-DD 日期")
		}
	case fDateTime:
		if _, err := time.Parse("2006-01-02 15:04", v); err != nil {
			return bad("必须是 YYYY-MM-DD HH:MM 时间")
		}
	case fPct:
		if !pctRe.MatchString(v) {
			return bad("必须是带两位小数的百分比数值，如 +3.20")
		}
	case fInt:
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 1440 || strconv.Itoa(n) != v {
			return bad("必须是 0 到 1440 的整数")
		}
	}
	return nil
}
