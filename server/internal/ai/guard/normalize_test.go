package guard

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestNormalizeTable(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"空串", "", ""},
		{"纯空白", " \t\n  ", ""},
		{"零宽空格被删", "星" + zwsp + "河", "星河"},
		{"零宽连接符与断词符被删", "星" + zwj + "河" + zwnj + "科" + wordJoiner + "技", "星河科技"},
		{"字节序标记被删", bom + "星河", "星河"},
		{"软连字符被删", "co" + softHyphen + "op", "coop"},
		{"双向控制符被删", "a" + rtlOverride + "b" + isolateLTR + "c", "abc"},
		{"变体选择符被删", "a" + variationSelector16 + "b", "ab"},
		{"填充字符被删", "a" + hangulFiller + "b" + hangulChoseongFiller + "c", "abc"},
		{"控制字符被删", "a\x00b\x07c", "abc"},
		{"非法 UTF-8 被丢弃", "a\xffb", "ab"},
		{"全角数字转半角", "２０２５年１２月", "2025年12月"},
		{"全角字母转半角", "ＡＩ大模型", "AI大模型"},
		{"兼容字符展开", "ﬁ①㈱", "fi1(株)"},
		{"预组合字符", "e" + combiningAcute, eAcute},
		{"零宽字符夹在组合序列里：先删再组合", "e" + zwj + combiningAcute, eAcute},
		{"不换行空格与全角空格变普通空格", "a" + nbsp + "b" + ideographicSpace + "c", "a b c"},
		{"连续空白折成一个空格", "a  \t  b", "a b"},
		{"首尾空白去掉", "  星河  ", "星河"},
		{"换行保留为单个换行", "甲\n\n\n乙", "甲\n乙"},
		{"回车换行统一为换行", "甲\r\n乙\r丙", "甲\n乙\n丙"},
		{"行分隔符当换行", "甲" + lineSeparator + "乙" + paragraphSeparator + "丙", "甲\n乙\n丙"},
		{"换行两侧的空格被吞掉", "甲 \n 乙", "甲\n乙"},
		{"全角感叹问号分号保持原样", "好！真的？是的；行", "好！真的？是的；行"},
		{"省略号保持原样不展开", "他说……然后", "他说……然后"},
		{"半角标点原样", "ok! yes? a;b", "ok! yes? a;b"},
		{"全角冒号逗号转半角", "公司：主营，算力", "公司:主营,算力"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Normalize(c.in); got != c.want {
				t.Fatalf("Normalize(%q) = %q，期望 %q", c.in, got, c.want)
			}
		})
	}
}

// normalizeInvariants 检查 Normalize 结果必须满足的不变量，返回违反的描述；空表示全部满足。
func normalizeInvariants(in string) string {
	out := Normalize(in)
	if !utf8.ValidString(out) {
		return "结果不是合法 UTF-8"
	}
	if again := Normalize(out); again != out {
		return "不幂等：" + again
	}
	if strings.ContainsFunc(out, isInvisible) {
		return "结果仍含不可见字符"
	}
	if strings.Contains(out, "  ") || strings.Contains(out, "\n\n") || strings.Contains(out, " \n") || strings.Contains(out, "\n ") {
		return "空白没有折叠干净"
	}
	if out != strings.Trim(out, " \n") {
		return "首尾有空白"
	}
	for _, r := range out {
		if r != ' ' && r != '\n' && unicode.IsSpace(r) {
			return "存在未归一的空白字符"
		}
	}
	return ""
}

func TestNormalizeInvariantsOnSamples(t *testing.T) {
	samples := []string{
		"", " ", "星" + zwsp + "河", bom + zwsp, "ＡＩ" + ideographicSpace + "大模型\r\n",
		"e" + zwj + combiningAcute + combiningAcute, "a\xff\xfeb", "！" + zwsp + "？" + zwsp + "；…",
		lineSeparator + paragraphSeparator + "\n \t", "①②③ ﬁ ㈱", "x" + nbsp + nbsp + "y",
		variationSelector17 + "a" + variationSelector256 + "b",
		grinningFace + " emoji " + grinningFace + zwj + grinningFace, "a" + combiningAcute + combiningAcute + "b",
	}
	for _, s := range samples {
		if msg := normalizeInvariants(s); msg != "" {
			t.Errorf("样本 %q 违反不变量：%s", s, msg)
		}
	}
}

func FuzzNormalize(f *testing.F) {
	for _, s := range []string{
		"", "星河科技", "星" + zwsp + "河", "ＡＩ" + ideographicSpace + "大模型", "甲\r\n乙", "e" + zwj + combiningAcute,
		"好！？；…", "a\xffb", "  \n  ", lineSeparator + "x" + paragraphSeparator, grinningFace,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if msg := normalizeInvariants(s); msg != "" {
			t.Fatalf("输入 %q 违反不变量：%s", s, msg)
		}
	})
}

func TestFoldTableWellFormed(t *testing.T) {
	if len(tradToSimp) < 100 {
		t.Fatalf("繁简表条目过少：%d", len(tradToSimp))
	}
	for trad, simp := range tradToSimp {
		if trad == simp {
			t.Errorf("繁简表出现自映射：%c", trad)
		}
		if _, chained := tradToSimp[simp]; chained {
			t.Errorf("繁简表出现链式映射：%c -> %c -> ...", trad, simp)
		}
	}
	if got := foldRune('龍'); got != '龙' {
		t.Errorf("foldRune(龍) = %c", got)
	}
	if got := foldRune('星'); got != '星' {
		t.Errorf("表外字符不应被改动：%c", got)
	}
}

func TestBuildFoldTablePanicsOnBadInput(t *testing.T) {
	for name, in := range map[string]string{
		"单字符条目": "龍龙 頭",
		"三字符条目": "龍龙頭",
		"重复键":   "龍龙 龍龍",
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("坏表 %q 应当 panic", in)
				}
			}()
			buildFoldTable(in)
		})
	}
}

func TestSkeleton(t *testing.T) {
	cases := []struct{ in, want string }{
		{"星 河" + zwsp + "科*技", "星河科技"},
		{"ＡＢ c！d", "abc！d"},
		{"增长 8.5%，同比", "增长85%同比"},
		{"甲。乙；丙?丁!戊", "甲。乙；丙?丁!戊"},
		{"表情" + grinningFace + "符号★也去掉", "表情符号也去掉"},
		{"龍斷", "龙断"},
		{"甲\n乙", "甲乙"},
	}
	for _, c := range cases {
		if got := skeleton(Normalize(c.in)); got != c.want {
			t.Errorf("skeleton(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}
