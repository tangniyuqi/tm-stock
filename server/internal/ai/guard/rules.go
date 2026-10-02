package guard

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

// wordsTxt 是 scripts/compliance-forbidden-words.txt 的构建时副本（go:embed 不能越出包目录）。
// 由 scripts/sync-guard-words.sh 同步；TestWordsMatchSource 保证与真源一致。
//
//go:embed words.txt
var wordsTxt string

// patternsTxt 是句式规则，格式见该文件头部注释。
//
//go:embed patterns.txt
var patternsTxt string

// Violation 是一次红线命中。
type Violation struct {
	// Rule 是规则标识："word:<词表词>" 或 "pat:<规则名>"。
	Rule string
	// Match 是命中的片段（骨架视图里的文字，@raw 规则则是规范化视图里的文字；词表词命中时就是词本身）。
	Match string
}

// Category 返回规则大类：word（词表词）或 pat（句式规则）。
func (v Violation) Category() string {
	if i := strings.IndexByte(v.Rule, ':'); i > 0 {
		return v.Rule[:i]
	}
	return v.Rule
}

type patternRule struct {
	name string
	// raw 为 true 时在"规范化视图"（Normalize 的结果，保留标点）上匹配，否则在骨架视图上匹配。
	// 骨架视图会丢掉 & # \ 这类符号，所以"编码字符引用"这类规则只能写成 raw。
	raw bool
	re  *regexp.Regexp
}

var (
	// ruleWords 是词表词的骨架形式：键为骨架，值为词表原词。
	ruleWords []wordRule
	// rulePatterns 是编译好的句式规则，保持文件内顺序。
	rulePatterns []patternRule
)

type wordRule struct {
	word string // 词表原词
	skel string // 骨架形式
}

func init() {
	var err error
	if ruleWords, err = parseWords(wordsTxt); err != nil {
		panic(err)
	}
	if rulePatterns, err = parsePatterns(patternsTxt); err != nil {
		panic(err)
	}
}

// parseWords 解析词表：一行一个词，# 开头为注释；兼容 CRLF。
func parseWords(src string) ([]wordRule, error) {
	var out []wordRule
	seen := map[string]bool{}
	for i, line := range strings.Split(src, "\n") {
		w := strings.TrimSpace(line)
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		skel := skeleton(Normalize(w))
		if skel == "" {
			return nil, fmt.Errorf("guard: words.txt 第 %d 行规范化后为空: %q", i+1, w)
		}
		if seen[skel] {
			continue
		}
		seen[skel] = true
		out = append(out, wordRule{word: w, skel: skel})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("guard: words.txt 没有任何词条")
	}
	return out, nil
}

// rawMarker 写在正则之前，表示该规则在规范化视图（而不是骨架视图）上匹配。
const rawMarker = "@raw"

// parsePatterns 解析句式规则：规则名 = [@raw] 正则；规则名唯一，正则必须能编译。
func parsePatterns(src string) ([]patternRule, error) {
	var out []patternRule
	seen := map[string]bool{}
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, expr, ok := strings.Cut(line, "=")
		name, expr = strings.TrimSpace(name), strings.TrimSpace(expr)
		if !ok || name == "" || expr == "" {
			return nil, fmt.Errorf("guard: patterns.txt 第 %d 行格式应为 \"规则名 = 正则\": %q", i+1, line)
		}
		if seen[name] {
			return nil, fmt.Errorf("guard: patterns.txt 规则名重复: %s", name)
		}
		seen[name] = true
		raw := false
		if expr == rawMarker || strings.HasPrefix(expr, rawMarker+" ") || strings.HasPrefix(expr, rawMarker+"\t") {
			raw = true
			expr = strings.TrimSpace(strings.TrimPrefix(expr, rawMarker))
			if expr == "" {
				return nil, fmt.Errorf("guard: patterns.txt 第 %d 行 %s 之后没有正则: %q", i+1, rawMarker, line)
			}
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("guard: patterns.txt 规则 %s 正则无法编译: %w", name, err)
		}
		out = append(out, patternRule{name: name, raw: raw, re: re})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("guard: patterns.txt 没有任何规则")
	}
	return out, nil
}

// RuleNames 返回全部规则标识（词表词在前，句式规则在后），供覆盖率测试与运营展示使用。
func RuleNames() []string {
	names := make([]string, 0, len(ruleWords)+len(rulePatterns))
	for _, w := range ruleWords {
		names = append(names, "word:"+w.word)
	}
	for _, p := range rulePatterns {
		names = append(names, "pat:"+p.name)
	}
	return names
}

// CheckText 检测文本里的红线：词表词与句式规则。返回全部命中；空表示未命中。
//
// 检测在"骨架视图"上进行（见 skeleton），因此对插空格、插标点、插 emoji、零宽字符、
// 换行拆词、全角半角混写、大小写与繁体字形这几类规避都有效；句末标点会截断匹配，
// 所以 "龙。头" 不算命中。命中是"宁可错杀"的取向：调用方（引文筛查、抽取校验）
// 拿到命中后走"截取或转人工"，而不是直接丢弃。
//
// 编码字符引用（HTML 数字实体、\u 转义）不解码，而是"出现即拦"（patterns.txt 里的 @raw 规则）：
// 正常文本里不会有这些写法，它们的唯一用途是把词表词藏起来绕过骨架视图。Base64、百分号编码、
// 拼音/谐音/emoji 替代与纯语义改写拦不住，按设计由第二道防线负责（design.md 3.4.1）。
func CheckText(s string) []Violation {
	norm := Normalize(s)
	skel := skeleton(norm)
	if skel == "" {
		return nil
	}
	var out []Violation
	for _, w := range ruleWords {
		if strings.Contains(skel, w.skel) {
			out = append(out, Violation{Rule: "word:" + w.word, Match: w.word})
		}
	}
	for _, p := range rulePatterns {
		target := skel
		if p.raw {
			target = norm
		}
		if m := p.re.FindString(target); m != "" {
			out = append(out, Violation{Rule: "pat:" + p.name, Match: m})
		}
	}
	return out
}

// Blocked 是 CheckText 的布尔形式。
func Blocked(s string) bool { return len(CheckText(s)) > 0 }
