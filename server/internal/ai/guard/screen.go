package guard

import (
	"strings"
	"unicode/utf8"
)

// Action 是引文筛查的结论。
type Action int

const (
	// Pass：引文没有任何红线命中，原样展示。
	Pass Action = iota
	// Trim：有命中，但能截取出不含命中的最小事实子句；展示截取后的文字，并须标注"节选"。
	Trim
	// Human：截取不出，转人工。人工放行须标"原文转载"并留痕（requirements 5.4 第 4 条）。
	Human
)

func (a Action) String() string {
	switch a {
	case Pass:
		return "pass"
	case Trim:
		return "trim"
	case Human:
		return "human"
	}
	return "unknown"
}

// Screened 是 ScreenQuote 的结果。
type Screened struct {
	Action Action
	// Text 是应当展示的文字：Pass 为规范化后的引文，Trim 为截取结果，Human 为空。
	Text string
	// Violations 是对整条引文检测得到的命中，供留痕与人工复核。
	Violations []Violation
}

// minTrimRunes 是截取结果的最短长度：太短的片段没有"事实"可言，直接转人工。
const minTrimRunes = 6

// dependentStarts 是依附性连接词：以它们开头的片段离开上文就没有完整含义，截取后不得展示。
// 按前缀匹配（"其他业务…" 也会被当作以"其"开头——有意取严，转人工即可）。
var dependentStarts = []string{
	"但是", "但", "然而", "因此", "所以", "同时", "另外", "此外", "其中", "其", "该", "上述", "前述", "以上",
	"并且", "而且", "而", "且", "因而", "从而", "对此", "为此", "据此", "由此", "故",
}

// span 是文本里的一个字节区间 [start, end)。
type span struct{ start, end int }

// ScreenQuote 对待展示的引文做红线筛查，口径见 eval/README.md 第 2 节：
//
//  1. 规范化后若无命中：Pass；
//  2. 否则按句切分，找出由连续"干净句"（自身无命中）组成的最长一段，取其原文连续子串；
//  3. 没有任何干净句时退到分句粒度（以 ，,、：: 及句末符切分）重复上一步；
//  4. 候选须长度 ≥ 6 个字符、不以依附性连接词开头、且再检测仍无命中，否则 Human。
//
// 只试一个候选：试不过就转人工，不去找"次优候选"——口径越简单越可审计。
func ScreenQuote(quote string) Screened {
	q := Normalize(quote)
	vs := CheckText(q)
	if len(vs) == 0 {
		return Screened{Action: Pass, Text: q}
	}
	for _, units := range [][]span{sentenceSpans(q), clauseSpans(q)} {
		text, found := bestCleanRun(q, units)
		if !found {
			continue // 这个粒度上没有干净单元，退到更细的粒度
		}
		if utf8.RuneCountInString(text) < minTrimRunes || startsDependent(text) || len(CheckText(text)) > 0 {
			return Screened{Action: Human, Violations: vs}
		}
		return Screened{Action: Trim, Text: text, Violations: vs}
	}
	return Screened{Action: Human, Violations: vs}
}

func startsDependent(text string) bool {
	for _, p := range dependentStarts {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

// sentenceSpans 把规范化文本切成句子区间。
func sentenceSpans(q string) []span {
	sents := SplitSentences(q)
	out := make([]span, len(sents))
	for i, s := range sents {
		out[i] = span{s.Start, s.End}
	}
	return out
}

// clauseSpans 在句子区间基础上再按分句符切开；区间不含分句符本身，也不含首尾空格。
func clauseSpans(q string) []span {
	var out []span
	for _, s := range SplitSentences(q) {
		from := s.Start
		flush := func(to int) {
			a, b := from, to
			for a < b && q[a] == ' ' {
				a++
			}
			for b > a && q[b-1] == ' ' {
				b--
			}
			if b > a {
				out = append(out, span{a, b})
			}
		}
		for i, r := range q[s.Start:s.End] {
			switch r {
			case '，', ',', '、', '：', ':':
				flush(s.Start + i)
				from = s.Start + i + utf8.RuneLen(r)
			}
		}
		flush(s.End)
	}
	return out
}

// bestCleanRun 在 units 里找由连续干净单元组成的最长一段（按字符数，并列取靠前的），
// 返回其在 q 中的连续子串（首尾去掉空白与分隔符）。找不到干净单元时 found=false。
func bestCleanRun(q string, units []span) (text string, found bool) {
	bestFrom, bestTo, bestLen := -1, -1, -1
	runFrom := -1
	closeRun := func(lastIdx int) {
		if runFrom < 0 {
			return
		}
		seg := q[units[runFrom].start:units[lastIdx].end]
		if n := utf8.RuneCountInString(seg); n > bestLen {
			bestFrom, bestTo, bestLen = runFrom, lastIdx, n
		}
		runFrom = -1
	}
	for i, u := range units {
		if len(CheckText(q[u.start:u.end])) == 0 {
			if runFrom < 0 {
				runFrom = i
			}
			continue
		}
		closeRun(i - 1)
	}
	closeRun(len(units) - 1)
	if bestFrom < 0 {
		return "", false
	}
	seg := q[units[bestFrom].start:units[bestTo].end]
	return strings.Trim(seg, " \n，,、；;：:"), true
}
