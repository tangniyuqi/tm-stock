package guard

import "testing"

// 测试里需要的不可见与特殊字符。一律用 string(rune(..)) 构造，源码里既不写 \u 转义，
// 也不写真实的不可见字符：前者会被部分编辑与生成工具悄悄转成真实字符，后者读代码的人看不见。
var (
	zwsp                 = string(rune(0x200B))  // 零宽空格
	zwnj                 = string(rune(0x200C))  // 零宽不连字
	zwj                  = string(rune(0x200D))  // 零宽连字
	wordJoiner           = string(rune(0x2060))  // 词连接符
	bom                  = string(rune(0xFEFF))  // 字节序标记
	softHyphen           = string(rune(0x00AD))  // 软连字符
	rtlOverride          = string(rune(0x202E))  // 从右到左覆盖
	isolateLTR           = string(rune(0x2066))  // 从左到右隔离
	variationSelector16  = string(rune(0xFE0F))  // 变体选择符 16
	hangulFiller         = string(rune(0x3164))  // 韩文填充符
	hangulChoseongFiller = string(rune(0x115F))  // 韩文初声填充符
	nbsp                 = string(rune(0x00A0))  // 不换行空格
	ideographicSpace     = string(rune(0x3000))  // 全角空格
	lineSeparator        = string(rune(0x2028))  // 行分隔符
	paragraphSeparator   = string(rune(0x2029))  // 段分隔符
	combiningAcute       = string(rune(0x0301))  // 组合锐音符
	eAcute               = string(rune(0x00E9))  // é（预组合）
	variationSelector17  = string(rune(0xE0100)) // 补充区变体选择符
	variationSelector256 = string(rune(0xE01EF))
	grinningFace         = string(rune(0x1F600)) // 😀
)

// bad 返回词表里的第一个词，供测试构造"含红线词"的文本，
// 这样 .go 源码里不必出现词表词本身（合规门禁会扫描 server/ 下的 .go 文件）。
func bad(t testing.TB) string {
	t.Helper()
	if len(ruleWords) == 0 {
		t.Fatal("词表为空")
	}
	return ruleWords[0].word
}

// badSecond 返回词表里的第二个词，用于需要两个不同红线词的用例。
func badSecond(t testing.TB) string {
	t.Helper()
	if len(ruleWords) < 2 {
		t.Fatal("词表词数不足 2 个")
	}
	return ruleWords[1].word
}
