package guard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	maxRawBytes    = 1 << 20 // 模型原始输出上限，防止超大输出拖垮校验
	maxClaims      = 50      // 单次输出最多 claim 条数，超过视为畸形
	maxSentenceIDs = 8       // 单条 claim 最多引用的句子数
	maxQuoteRunes  = 1000    // 引文总长度上限，对应 source_excerpt varchar(1000)
	minEntityRunes = 2
	maxEntityRunes = 32
	segmentJoiner  = "……" // 不相邻的多段原文之间的展示连接符
)

var (
	// ErrMalformed 表示模型输出整体畸形：provider 层应按"畸形至多修复重试 1 次"处理，仍失败则丢弃该来源。
	ErrMalformed = errors.New("guard: 抽取输出畸形")
	// ErrSnapshot 表示快照不满足不变量（正文未规范化、句子表与正文不符等），属于调用方的程序错误。
	ErrSnapshot = errors.New("guard: 快照不满足不变量")
)

// Vocabulary 是抽取校验用的封闭词汇表：谓词白名单与"题材键 → 锚词"。题材键本身也算锚词。
type Vocabulary struct {
	Predicates map[string]bool
	Themes     map[string][]string
}

// Claim 是模型返回的一条抽取结论。封闭结构：只有这四个字段，没有评级、分数、理由之类的字段。
type Claim struct {
	Predicate   string `json:"predicate"`
	Entity      string `json:"entity"`
	Theme       string `json:"theme"`
	SentenceIDs []int  `json:"sentence_ids"`
}

// RejectReason 是逐条拒绝的原因码；校验按下面的声明顺序进行，取第一个失败项。
type RejectReason string

const (
	RejectPredicate        RejectReason = "predicate_not_allowed"
	RejectTheme            RejectReason = "theme_not_allowed"
	RejectSentenceIDs      RejectReason = "sentence_ids_invalid"
	RejectQuoteTooLong     RejectReason = "quote_too_long"
	RejectEntityInvalid    RejectReason = "entity_invalid"
	RejectRedlineQuote     RejectReason = "redline_quote"
	RejectEntityNotInQuote RejectReason = "entity_not_in_quote"
	RejectNoCooccurrence   RejectReason = "no_cooccurrence"
)

// Accepted 是通过校验的 claim。Segments 是应展示的逐字引文段（来自快照正文，或其不含命中的截取），
// Quote 是它们用 "……" 连接后的展示文字，Trimmed 表示至少有一段被截取过（展示时须标"节选"）。
type Accepted struct {
	Index    int
	Claim    Claim
	Segments []string
	Quote    string
	Trimmed  bool
}

// Rejected 是被拒绝的 claim 及原因。
type Rejected struct {
	Index  int
	Reason RejectReason
}

// Report 是 ValidateExtraction 的逐条裁决结果，两个切片各自按 claim 下标升序。
type Report struct {
	Accepted []Accepted
	Rejected []Rejected
}

// 键名校验用：claim 对象只允许这四个键（区分大小写——encoding/json 默认大小写不敏感，不能直接依赖它）。
var claimKeys = map[string]bool{"predicate": true, "entity": true, "theme": true, "sentence_ids": true}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformed, fmt.Sprintf(format, args...))
}

// firstDuplicateKey 逐个 token 遍历 JSON，返回任一对象里第一个重复出现的键。
// Go 的解码器对重复键取最后一个，别的解析器可能取第一个——同一份输出被不同解析器读出不同含义，
// 对封闭结构的闸门来说就是歧义，一律按畸形处理。输入不是合法 JSON 时返回 ("", false)，由后续解析报畸形。
func firstDuplicateKey(raw []byte) (string, bool) {
	type frame struct {
		obj       bool
		expectKey bool
		keys      map[string]bool
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var stack []frame
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", false
		}
		d, isDelim := tok.(json.Delim)
		if isDelim && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return "", false // 顶层值结束；其后的多余内容由后续解析判定
			}
			if top := &stack[len(stack)-1]; top.obj {
				top.expectKey = true
			}
			continue
		}
		if n := len(stack); n > 0 && stack[n-1].obj && stack[n-1].expectKey {
			key, _ := tok.(string)
			if stack[n-1].keys[key] {
				return key, true
			}
			stack[n-1].keys[key] = true
			stack[n-1].expectKey = false
			continue
		}
		if isDelim { // '{' 或 '['
			stack = append(stack, frame{obj: d == '{', expectKey: d == '{', keys: map[string]bool{}})
			continue
		}
		if len(stack) == 0 {
			return "", false // 顶层是标量
		}
		if top := &stack[len(stack)-1]; top.obj {
			top.expectKey = true
		}
	}
}

// parseClaims 严格解析模型原始输出：顶层只能是 {"claims":[…]}，claim 只能含四个已知键，
// 类型必须正确，JSON 之后不得有多余内容（代码围栏、解释性文字都算畸形）。
func parseClaims(raw []byte) ([]Claim, error) {
	if len(raw) > maxRawBytes {
		return nil, malformed("输出超过 %d 字节", maxRawBytes)
	}
	if k, dup := firstDuplicateKey(raw); dup {
		return nil, malformed("对象里出现重复的键 %q", k)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var top map[string]json.RawMessage
	if err := dec.Decode(&top); err != nil {
		return nil, malformed("不是合法的 JSON 对象")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, malformed("JSON 之后还有多余内容")
	}
	if top == nil {
		return nil, malformed("顶层不能为 null")
	}
	for k := range top {
		if k != "claims" {
			return nil, malformed("未知顶层字段 %q", k)
		}
	}
	rawClaims, ok := top["claims"]
	if !ok || bytes.Equal(bytes.TrimSpace(rawClaims), []byte("null")) {
		return nil, malformed("缺少 claims 数组")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(rawClaims, &items); err != nil {
		return nil, malformed("claims 不是数组")
	}
	if len(items) > maxClaims {
		return nil, malformed("claims 超过 %d 条", maxClaims)
	}
	claims := make([]Claim, 0, len(items))
	for i, it := range items {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(it, &keys); err != nil || keys == nil {
			return nil, malformed("claims[%d] 不是对象", i)
		}
		for k := range keys {
			if !claimKeys[k] {
				return nil, malformed("claims[%d] 含未知字段 %q", i, k)
			}
		}
		var c Claim
		if err := json.Unmarshal(it, &c); err != nil {
			return nil, malformed("claims[%d] 字段类型错误", i)
		}
		claims = append(claims, c)
	}
	return claims, nil
}

// validSentenceIDs：1 到 maxSentenceIDs 个，严格升序（同时排除重复），且都在 [1, total]。
func validSentenceIDs(ids []int, total int) bool {
	if len(ids) == 0 || len(ids) > maxSentenceIDs {
		return false
	}
	for i, id := range ids {
		if id < 1 || id > total || (i > 0 && id <= ids[i-1]) {
			return false
		}
	}
	return true
}

// ValidateExtraction 校验模型返回的抽取结果（口径见 eval/README.md 第 5 节）。
//
// 返回 ErrMalformed（可用 errors.Is 判断）表示输出整体畸形；否则逐条裁决，每条 claim 按固定顺序检查：
//
//	谓词白名单 → 题材键 → 句子编号 → 取原文并检查总长 → 实体合法性 →
//	引文红线筛查（逐段 ScreenQuote；截取的段改用截取文字）→ 实体须在引文里 → 实体与题材锚词共现。
//
// 引文由代码按编号从快照正文取出：模型不返回自由文本引文，所以"引文逐字来自快照"是构造出来的，
// 不依赖模型诚实。共现按段判断（任一段通过即可），避免实体与锚词分别来自两个不相邻的段而被拼出关联。
func ValidateExtraction(raw []byte, snap *Snapshot, vocab Vocabulary) (*Report, error) {
	if err := snap.verify(); err != nil {
		return nil, err
	}
	claims, err := parseClaims(raw)
	if err != nil {
		return nil, err
	}
	rep := &Report{}
	for i, c := range claims {
		acc, reason := validateClaim(i, c, snap, vocab)
		if reason != "" {
			rep.Rejected = append(rep.Rejected, Rejected{Index: i, Reason: reason})
			continue
		}
		rep.Accepted = append(rep.Accepted, *acc)
	}
	return rep, nil
}

func validateClaim(index int, c Claim, snap *Snapshot, vocab Vocabulary) (*Accepted, RejectReason) {
	if !vocab.Predicates[c.Predicate] {
		return nil, RejectPredicate
	}
	synonyms, ok := vocab.Themes[c.Theme]
	if !ok {
		return nil, RejectTheme
	}
	if !validSentenceIDs(c.SentenceIDs, len(snap.Sentences)) {
		return nil, RejectSentenceIDs
	}
	segs := snap.segments(c.SentenceIDs)
	total := 0
	for _, s := range segs {
		total += utf8.RuneCountInString(s)
	}
	if total > maxQuoteRunes {
		return nil, RejectQuoteTooLong
	}
	entity := Normalize(c.Entity)
	if n := utf8.RuneCountInString(entity); n < minEntityRunes || n > maxEntityRunes ||
		strings.Contains(entity, "\n") || Blocked(entity) {
		return nil, RejectEntityInvalid
	}

	trimmed := false
	final := make([]string, 0, len(segs))
	for _, s := range segs {
		sc := ScreenQuote(s)
		switch sc.Action {
		case Human:
			return nil, RejectRedlineQuote
		case Trim:
			trimmed = true
		}
		final = append(final, sc.Text)
	}
	quote := strings.Join(final, segmentJoiner)
	if !containsCompact(quote, entity) {
		return nil, RejectEntityNotInQuote
	}
	anchors := append([]string{c.Theme}, synonyms...)
	cooccurs := false
	for _, s := range final {
		if CoOccur(s, []string{entity}, anchors) == CoOK {
			cooccurs = true
			break
		}
	}
	if !cooccurs {
		return nil, RejectNoCooccurrence
	}
	return &Accepted{Index: index, Claim: c, Segments: final, Quote: quote, Trimmed: trimmed}, ""
}
