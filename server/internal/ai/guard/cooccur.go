package guard

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// CoReason 是共现校验的原因码；CoOK 表示通过。
type CoReason string

const (
	CoOK              CoReason = "ok"
	CoUnusableAlias   CoReason = "unusable_alias"    // 去掉不可用项后，别名或锚词为空
	CoNoEntity        CoReason = "no_entity"         // 全文没有任何别名命中
	CoNoAnchor        CoReason = "no_anchor"         // 全文没有任何锚词命中
	CoNotSameSentence CoReason = "not_same_sentence" // 别名与锚词都出现过，但从未在同一句
	CoNegatedAnchor   CoReason = "negated_anchor"    // 同句共现，但每处锚词前都有否定提示
)

// negationCues 是否定提示词：出现在锚词之前且间隔不超过 2 个字符时，视为"否定该锚词"。
// 只收下面这些，"不仅""不但""不断"一类不算。
var negationCues = []string{
	"没有", "并未", "尚未", "不涉及", "不属于", "不包括", "不从事", "不存在", "并非", "不含", "不是", "不再",
	"未", "非", "无",
}

// maxNegationGap 是否定提示词尾与锚词开头之间允许的最大间隔字符数。
const maxNegationGap = 2

// compactRunes 规范化 s 后去掉全部空白，转小写，返回字符切片；用于别名与锚词的子串匹配。
func compactRunes(s string) []rune {
	n := Normalize(s)
	out := make([]rune, 0, utf8.RuneCountInString(n))
	for _, r := range n {
		if unicode.IsSpace(r) {
			continue
		}
		out = append(out, unicode.ToLower(r))
	}
	return out
}

func isASCIIAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func allASCIIAlnum(rs []rune) bool {
	for _, r := range rs {
		if !isASCIIAlnum(r) {
			return false
		}
	}
	return len(rs) > 0
}

// usableTerms 过滤别名或锚词：规范化、去空白、长度不足 2 个字符的丢弃，去重。
func usableTerms(terms []string) [][]rune {
	var out [][]rune
	seen := map[string]bool{}
	for _, t := range terms {
		rs := compactRunes(t)
		if len(rs) < 2 || seen[string(rs)] {
			continue
		}
		seen[string(rs)] = true
		out = append(out, rs)
	}
	return out
}

// findTerm 返回 term 在 hay 里全部命中的起点；纯 ASCII 字母数字的 term 要求前后不紧邻其它 ASCII 字母数字。
func findTerm(hay, term []rune) []int {
	var starts []int
	boundary := allASCIIAlnum(term)
	for i := 0; i+len(term) <= len(hay); i++ {
		match := true
		for j, r := range term {
			if hay[i+j] != r {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if boundary {
			if i > 0 && isASCIIAlnum(hay[i-1]) {
				continue
			}
			if e := i + len(term); e < len(hay) && isASCIIAlnum(hay[e]) {
				continue
			}
		}
		starts = append(starts, i)
	}
	return starts
}

// runeSpan 是 hay（字符切片）里的一段半开区间 [start, end)；注意与 screen.go 里按字节计的 span 区分。
type runeSpan struct{ start, end int }

func overlapsAny(spans []runeSpan, start, end int) bool {
	for _, sp := range spans {
		if start < sp.end && sp.start < end {
			return true
		}
	}
	return false
}

// negatedAt 判断起点为 anchorStart 的锚词前是否有否定提示：
// 某个否定词的词尾 e 满足 0 <= anchorStart-e <= maxNegationGap。
// 落在别名内部的字不是否定词（"无锡""非凡科技"里的"无""非"是名字的一部分），所以与别名重叠的否定词不计。
func negatedAt(hay []rune, anchorStart int, aliases []runeSpan) bool {
	for _, cue := range negationCues {
		c := []rune(cue)
		for _, s := range findTerm(hay, c) {
			if overlapsAny(aliases, s, s+len(c)) {
				continue
			}
			if e := s + len(c); e <= anchorStart && anchorStart-e <= maxNegationGap {
				return true
			}
		}
	}
	return false
}

// CoOccur 校验实体与题材锚词是否在同一个句子里共现。口径见 eval/README.md 第 3 节：
// 先规范化并切句，每句去掉空格后做子串匹配；长度不足 2 个字符的别名与锚词忽略；
// 纯 ASCII 的名字要求不紧邻其它 ASCII 字母数字；同句共现且锚词前无否定提示才算通过。
// 判定顺序固定：unusable_alias → no_entity → no_anchor → not_same_sentence → negated_anchor → ok。
//
// 共现是必要条件而不是充分条件：它挡住"推断出的关联"，但"是否真的蕴含"仍要由
// 蕴含裁判与人工审核判断。
func CoOccur(quote string, names, anchors []string) CoReason {
	ns, as := usableTerms(names), usableTerms(anchors)
	if len(ns) == 0 || len(as) == 0 {
		return CoUnusableAlias
	}
	q := Normalize(quote)
	var anyEntity, anyAnchor, sameSentence, okSentence bool
	for _, st := range SplitSentences(q) {
		hay := compactRunes(q[st.Start:st.End])
		var aliasSpans []runeSpan
		for _, n := range ns {
			for _, s := range findTerm(hay, n) {
				aliasSpans = append(aliasSpans, runeSpan{s, s + len(n)})
			}
		}
		entityHit := len(aliasSpans) > 0
		// 锚词只算"独立出现"的：与别名重叠的不算。别名本身带着锚词（如公司名"云岫算力"）时，
		// 提到公司名就等于"共现"了，那不是任何业务披露，只是名字里有这两个字。
		var anchorStarts []int
		for _, a := range as {
			for _, s := range findTerm(hay, a) {
				if !overlapsAny(aliasSpans, s, s+len(a)) {
					anchorStarts = append(anchorStarts, s)
				}
			}
		}
		anyEntity = anyEntity || entityHit
		anyAnchor = anyAnchor || len(anchorStarts) > 0
		if !entityHit || len(anchorStarts) == 0 {
			continue
		}
		sameSentence = true
		for _, s := range anchorStarts {
			if !negatedAt(hay, s, aliasSpans) {
				okSentence = true
			}
		}
	}
	switch {
	case !anyEntity:
		return CoNoEntity
	case !anyAnchor:
		return CoNoAnchor
	case !sameSentence:
		return CoNotSameSentence
	case !okSentence:
		return CoNegatedAnchor
	}
	return CoOK
}

// containsCompact 判断 hay 与 needle 各自规范化并去空白、转小写后，needle 是否为 hay 的子串。
// 这是纯子串语义（没有 ASCII 边界要求），仅用于"实体字符串须出现在展示引文里"的存在性检查；
// 带边界要求的匹配由 CoOccur 负责。
func containsCompact(hay, needle string) bool {
	h, n := compactRunes(hay), compactRunes(needle)
	if len(n) == 0 {
		return false
	}
	return strings.Contains(string(h), string(n))
}
