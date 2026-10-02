package guard

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// protectedPunct 是不参与 NFKC 全角转半角的标点：
//   - 全角 ！？； 若被转成半角，就丢掉了"一定结束一个句子"的语义（半角版本有更严的切句条件）；
//   - … 若被 NFKC 展开成三个半角点，会与小数点和句号混淆。
//
// 它们保持原样，切句与骨架视图都显式认得它们。
func protectedPunct(r rune) bool {
	switch r {
	case '！', '？', '；', '…':
		return true
	}
	return false
}

// isNewlineLike 判断是否换行类字符；它们在空白折叠时统一成 "\n"。
func isNewlineLike(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// isInvisible 判断是否应当直接删除的不可见字符：格式类（零宽、双向控制、软连字符等）、
// 控制字符、私有区、代理区、变体选择符、组合字素连接符与几个常被拿来当"隐形字"的填充字符。
// 空白与换行类不在此列，由 collapseSpace 处理。
func isInvisible(r rune) bool {
	switch {
	case r == '\t' || isNewlineLike(r):
		return false
	case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r), unicode.Is(unicode.Cs, r), unicode.IsControl(r):
		return true
	case r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0100 && r <= 0xE01EF:
		return true // 变体选择符
	case r == 0x034F, r == 0x115F, r == 0x1160, r == 0x3164, r == 0xFFA0, r == 0x180E:
		return true // 组合字素连接符、韩文填充符、蒙古文元音分隔符
	}
	return false
}

func stripInvisible(s string) string {
	if !strings.ContainsFunc(s, isInvisible) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !isInvisible(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// nfkcKeepPunct 对 s 做 NFKC，但 protectedPunct 里的字符原样保留。
func nfkcKeepPunct(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	start := 0
	for i, r := range s {
		if protectedPunct(r) {
			b.WriteString(norm.NFKC.String(s[start:i]))
			b.WriteRune(r)
			start = i + utf8.RuneLen(r)
		}
	}
	b.WriteString(norm.NFKC.String(s[start:]))
	return b.String()
}

// collapseSpace 折叠空白：连续空白折成一个空格；含换行的连续空白折成一个 "\n"；首尾去空白。
func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var pending byte // 0 无；' ' 空格；'\n' 换行（换行优先）
	for _, r := range s {
		switch {
		case isNewlineLike(r):
			pending = '\n'
		case unicode.IsSpace(r):
			if pending == 0 {
				pending = ' '
			}
		default:
			if b.Len() > 0 && pending != 0 {
				b.WriteByte(pending)
			}
			pending = 0
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Normalize 是来源与模型输出进入系统前的规范化：
//
//  1. 丢弃非法 UTF-8 字节；
//  2. 删除零宽字符、双向控制符、软连字符、变体选择符、控制字符等不可见字符；
//  3. NFKC（全角转半角、兼容字符展开；全角 ！？； 与 … 除外，见 protectedPunct）；
//  4. 折叠空白，换行保留为 "\n"，首尾去空白。
//
// 结果是幂等的：Normalize(Normalize(s)) == Normalize(s)。快照正文固化前必须先规范化，
// 这样"引文是快照正文的子串"这条校验才有意义。
func Normalize(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = stripInvisible(s)
	s = nfkcKeepPunct(s)
	s = stripInvisible(s)
	return collapseSpace(s)
}

// isBoundaryPunct 是骨架视图里保留的句末符：它们截断匹配，避免跨句拼出词表词。
func isBoundaryPunct(r rune) bool {
	switch r {
	case '。', '！', '？', '!', '?', '；', ';':
		return true
	}
	return false
}

// skeleton 生成"骨架视图"，红线检测在它上面匹配：
// 转小写、繁转简，去掉全部空白与标点符号（含符号、emoji），只保留句末符与百分号。
// 这样 "龙 头""龙*头""龙😀头""龍頭" 都会还原成同一个骨架。
// 入参必须已经过 Normalize。
func skeleton(normalized string) string {
	var b strings.Builder
	b.Grow(len(normalized))
	for _, r := range normalized {
		r = foldRune(unicode.ToLower(r))
		switch {
		case unicode.IsSpace(r):
			continue
		case isBoundaryPunct(r), r == '%':
			b.WriteRune(r)
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Skeleton 返回文本的骨架视图（先 Normalize，再转小写、繁转简、去掉空白与除句末符和百分号外的全部标点符号）。
// 供需要"抗插字规避"的子串匹配使用，例如命名禁用字样检查；红线检测本身用的就是它。
func Skeleton(s string) string { return skeleton(Normalize(s)) }
