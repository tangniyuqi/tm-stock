package render

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
)

// NodeKind 是用户可见节点的类型，只有三类（AC-G1）。
type NodeKind string

const (
	KindTemplate NodeKind = "template" // 模板句
	KindQuote    NodeKind = "quote"    // 引文块
	KindDataRow  NodeKind = "data_row" // 数据行（个股行，只含客观字段，AC-G6）
)

// maxQuoteRunes 与 guard 的引文长度上限一致（对应 source_excerpt varchar(1000)）。
const maxQuoteRunes = 1000

// Reprint 记录人工放行：引文命中红线词时不原样展示，人工放行须标"原文转载"并留痕（requirements 5.4 第 4 条）。
type Reprint struct {
	By string // 放行人
	At int64  // 放行时间，毫秒时间戳
}

// Quote 是逐字引文及其证据引用。Text 必须是快照正文（规范化后）的连续子串，由 guard 按句子编号取出。
type Quote struct {
	SnapshotID  int64    `json:"snapshotId"`
	Start       int      `json:"start"` // 在快照正文里的字节偏移，左闭右开
	End         int      `json:"end"`
	Text        string   `json:"text"`
	SourceTitle string   `json:"sourceTitle"`
	SourceType  string   `json:"sourceType"`
	URL         string   `json:"url"`
	FetchedAt   int64    `json:"fetchedAt"` // 采集时点，毫秒时间戳
	ContentHash string   `json:"contentHash"`
	Trimmed     bool     `json:"trimmed"` // 被截取过，展示须标"节选"
	Reprint     *Reprint `json:"reprint,omitempty"`
}

// EvidenceItem 是个股行的"依据"：类型、摘录、链接、采集时点。
type EvidenceItem struct {
	Type      string `json:"type"`
	Excerpt   string `json:"excerpt"`
	URL       string `json:"url"`
	FetchedAt int64  `json:"fetchedAt"`
}

// Change 是涨跌幅及其口径。没有数据时 StockRow.Change 为 nil，不用 0 代替（数据真实铁律）。
type Change struct {
	Pct      float64 `json:"pct"`
	Basis    string  `json:"basis"`    // 口径
	DelayMin int     `json:"delayMin"` // 延时分钟数
	AsOf     int64   `json:"asOf"`     // 数据时点，毫秒时间戳
	Vendor   string  `json:"vendor"`
}

// StockRow 是个股数据行。刻意只有名称、代码、归属环节、依据、涨跌幅五类字段（AC-G6），
// 任何评价、排名、推荐维度都不得加入——TestStockRowFieldsAreClosed 守护这个字段集合。
type StockRow struct {
	Name     string         `json:"name"`
	Code     string         `json:"code"`
	Node     string         `json:"node"` // 归属环节
	Evidence []EvidenceItem `json:"evidence"`
	Change   *Change        `json:"change"`
}

// Node 是报告里的一个用户可见节点。Text 由本包生成：模板句由谓词与参数渲染，引文块取自 Quote.Text；
// 外部代码不应手工拼 Text，Validate 会按谓词与参数重新渲染并逐字比对，被改动过的文字一律拒绝。
type Node struct {
	Kind       NodeKind          `json:"kind"`
	Provenance Provenance        `json:"provenance"`
	Predicate  Predicate         `json:"predicate,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
	Text       string            `json:"text,omitempty"`
	Quote      *Quote            `json:"quote,omitempty"`
	Row        *StockRow         `json:"row,omitempty"`
}

var (
	// ErrUnknownPredicate：谓词不在封闭谓词库内。
	ErrUnknownPredicate = errors.New("render: 谓词不在封闭谓词库内")
	// ErrFieldSet：参数字段与谓词规格不一致（缺字段或多字段）。
	ErrFieldSet = errors.New("render: 参数字段与谓词规格不一致")
	// ErrProvenance：该谓词不允许这个来源性质（涨跌幅与证据状态只能是 code_computed）。
	ErrProvenance = errors.New("render: 来源性质与谓词不匹配")
)

// Render 按谓词模板渲染一个模板句节点。args 的字段集合必须与谓词规格完全一致，
// 每个字段按类型严格校验；渲染结果还要再过一遍红线检测作为纵深防御。
func Render(p Predicate, prov Provenance, args map[string]string) (Node, error) {
	spec, ok := specs[p]
	if !ok {
		return Node{}, fmt.Errorf("%w: %q", ErrUnknownPredicate, p)
	}
	allowed := false
	for _, a := range spec.provenances {
		if a == prov {
			allowed = true
		}
	}
	if !allowed {
		return Node{}, fmt.Errorf("%w: %s 不允许 %q", ErrProvenance, p, prov)
	}
	if len(args) != len(spec.fields) {
		return Node{}, fmt.Errorf("%w: %s 需要 %d 个字段，收到 %d 个", ErrFieldSet, p, len(spec.fields), len(args))
	}
	pairs := make([]string, 0, len(spec.fields)*2)
	copied := make(map[string]string, len(args))
	for _, f := range spec.fields {
		v, ok := args[f.name]
		if !ok {
			return Node{}, fmt.Errorf("%w: %s 缺少字段 %s", ErrFieldSet, p, f.name)
		}
		if ferr := validateField(f, v); ferr != nil {
			return Node{}, ferr
		}
		pairs = append(pairs, "{"+f.name+"}", v)
		copied[f.name] = v
	}
	text := strings.NewReplacer(pairs...).Replace(spec.pattern)
	if vs := guard.CheckText(text); len(vs) > 0 {
		return Node{}, &FieldError{Field: "(整句)", Reason: reasonRedline}
	}
	return Node{Kind: KindTemplate, Provenance: prov, Predicate: p, Args: copied, Text: text}, nil
}

// QuoteNode 把一段逐字引文包成引文块节点。引文须已规范化、长度合规；
// 命中红线检测的引文必须带人工放行记录（Reprint），否则拒绝——这是"引文命中禁用词不原样展示"的最后一道闸。
func QuoteNode(q Quote, prov Provenance) (Node, error) {
	if prov != ProvLibrary && prov != ProvAIExtractedQuote {
		return Node{}, fmt.Errorf("%w: 引文块只能是 library 或 ai_extracted_quote", ErrProvenance)
	}
	if err := checkQuote(q); err != nil {
		return Node{}, err
	}
	return Node{Kind: KindQuote, Provenance: prov, Text: q.Text, Quote: &q}, nil
}

func checkQuote(q Quote) error {
	if n := utf8.RuneCountInString(q.Text); n < 1 || n > maxQuoteRunes {
		return &FieldError{Field: "quote.text", Reason: fmt.Sprintf("长度须在 1 到 %d 个字符之间", maxQuoteRunes)}
	}
	if guard.Normalize(q.Text) != q.Text {
		return &FieldError{Field: "quote.text", Reason: "必须是规范化后的文本"}
	}
	if q.Start < 0 || q.End <= q.Start {
		return &FieldError{Field: "quote.start/end", Reason: "偏移区间不合法"}
	}
	if q.SnapshotID <= 0 || q.ContentHash == "" || q.FetchedAt <= 0 {
		return &FieldError{Field: "quote", Reason: "证据引用不完整（须有快照编号、内容哈希、采集时点）"}
	}
	if guard.Blocked(q.Text) {
		if q.Reprint == nil || strings.TrimSpace(q.Reprint.By) == "" || q.Reprint.At <= 0 {
			return &FieldError{Field: "quote.text", Reason: reasonRedline + "且没有人工放行留痕"}
		}
	}
	return nil
}

// RowNode 把个股行包成数据行节点。数据行的来源性质固定为 library：成分关系来自已审核资料库，
// 其中的涨跌幅由代码计算，但整行不冒充 AI 内容。
func RowNode(r StockRow) (Node, error) {
	if err := checkRow(r); err != nil {
		return Node{}, err
	}
	return Node{Kind: KindDataRow, Provenance: ProvLibrary, Row: &r}, nil
}

func checkRow(r StockRow) error {
	if ferr := validateField(fieldSpec{name: "row.name", kind: fName}, r.Name); ferr != nil {
		return ferr
	}
	if ferr := validateField(fieldSpec{name: "row.code", kind: fCode}, r.Code); ferr != nil {
		return ferr
	}
	if ferr := validateField(fieldSpec{name: "row.node", kind: fName}, r.Node); ferr != nil {
		return ferr
	}
	if len(r.Evidence) == 0 {
		return &FieldError{Field: "row.evidence", Reason: "没有依据的个股行不得展示（无依据禁止入库）"}
	}
	for i, e := range r.Evidence {
		if strings.TrimSpace(e.Type) == "" || e.FetchedAt <= 0 {
			return &FieldError{Field: fmt.Sprintf("row.evidence[%d]", i), Reason: "依据须有类型与采集时点"}
		}
		if e.Excerpt != "" {
			if guard.Normalize(e.Excerpt) != e.Excerpt || utf8.RuneCountInString(e.Excerpt) > maxQuoteRunes {
				return &FieldError{Field: fmt.Sprintf("row.evidence[%d].excerpt", i), Reason: "摘录须规范化且不超过 1000 字"}
			}
			if guard.Blocked(e.Excerpt) {
				return &FieldError{Field: fmt.Sprintf("row.evidence[%d].excerpt", i), Reason: reasonRedline}
			}
		}
	}
	if c := r.Change; c != nil {
		if strings.TrimSpace(c.Basis) == "" || strings.TrimSpace(c.Vendor) == "" || c.AsOf <= 0 || c.DelayMin < 0 || c.DelayMin > 1440 {
			return &FieldError{Field: "row.change", Reason: "涨跌幅须同时带口径、延时、数据时点与数据来源"}
		}
		if c.Pct != c.Pct || c.Pct > 10000 || c.Pct < -100 {
			return &FieldError{Field: "row.change.pct", Reason: "涨跌幅数值越界或非数字"}
		}
	}
	return nil
}
