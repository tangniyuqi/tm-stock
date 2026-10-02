package guard

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func sentenceTexts(body string) []string {
	var out []string
	for _, s := range SplitSentences(body) {
		out = append(out, body[s.Start:s.End])
	}
	return out
}

func TestSplitSentencesTable(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"空串", "", nil},
		{"纯空白", "  \n ", nil},
		{"句号分句", "甲公司成立于2010年。乙公司成立于2012年。", []string{"甲公司成立于2010年。", "乙公司成立于2012年。"}},
		{"全角感叹问号分号一定结束", "好！真的？是的；行", []string{"好！", "真的？", "是的；", "行"}},
		{"半角感叹后跟字母不结束", "Yahoo!Finance is good", []string{"Yahoo!Finance is good"}},
		{"半角感叹后跟空白结束", "OK! Next one", []string{"OK!", "Next one"}},
		{"半角分号夹在字母间不结束", "a;b", []string{"a;b"}},
		{"半角问号后跟汉字不结束", "吗?是的。", []string{"吗?是的。"}},
		{"小数点不切碎", "营收12.5亿元。同比增长8.5%。", []string{"营收12.5亿元。", "同比增长8.5%。"}},
		{"数字后的句点后跟空白也不结束", "编号3. 公司", []string{"编号3. 公司"}},
		{"汉字后的句点后跟空白结束", "详见附件. 公司说明", []string{"详见附件.", "公司说明"}},
		{"句点后不是空白不结束", "Co.,Ltd公司", []string{"Co.,Ltd公司"}},
		{"省略号不结束", "他说……然后离开。", []string{"他说……然后离开。"}},
		{"冒号不结束", "公司：主营算力。", []string{"公司：主营算力。"}},
		{"右引号归属前句", "他说：“好。”然后走了。", []string{"他说：“好。”", "然后走了。"}},
		{"右括号归属前句", "详见公告（第3号。）后续", []string{"详见公告（第3号。）", "后续"}},
		{"重复句末标点归属前句", "真的吗？？是的。", []string{"真的吗？？", "是的。"}},
		{"换行一定结束", "标题\n正文第一句。正文第二句。", []string{"标题", "正文第一句。", "正文第二句。"}},
		{"没有句末符的尾巴也成句", "甲。乙", []string{"甲。", "乙"}},
		{"句间空格不属于任何句子", "甲。 乙。", []string{"甲。", "乙。"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sentenceTexts(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("SplitSentences(%q) = %q，期望 %q", c.in, got, c.want)
			}
		})
	}
}

// splitInvariants 检查切句结果的不变量，返回违反描述；空表示满足。
func splitInvariants(body string) string {
	sents := SplitSentences(body)
	covered := make([]bool, len(body))
	prevEnd := 0
	for i, s := range sents {
		if s.ID != i+1 {
			return "编号不连续"
		}
		if s.Start < prevEnd || s.End <= s.Start || s.End > len(body) {
			return "区间乱序、重叠或越界"
		}
		if !utf8.ValidString(body[s.Start:s.End]) {
			return "区间切断了多字节字符"
		}
		text := body[s.Start:s.End]
		if text != strings.Trim(text, " \n") {
			return "句子首尾含空白或换行"
		}
		for k := s.Start; k < s.End; k++ {
			covered[k] = true
		}
		prevEnd = s.End
	}
	for k, r := range []byte(body) {
		if !covered[k] && r != ' ' && r != '\n' {
			return "存在未被任何句子覆盖的非空白字节"
		}
	}
	return ""
}

func TestSplitSentencesInvariantsOnSamples(t *testing.T) {
	for _, s := range []string{
		"", "。", "！！！", "甲。 乙！ 丙？\n丁；戊", "a. b. c.", "x!y?z;w", "“好。”(完)。", "12.5.3. 末", "\n\n甲\n\n",
		"…… 。 ……", "甲" + nbsp + "乙。", "ok! \n next? ",
	} {
		if msg := splitInvariants(Normalize(s)); msg != "" {
			t.Errorf("样本 %q 违反切句不变量：%s", s, msg)
		}
	}
}

func FuzzSplitSentences(f *testing.F) {
	for _, s := range []string{"", "甲。乙。", "Yahoo!Finance. ok", "他说：“好。”然后", "标题\n正文。", "12.5. 3. x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if msg := splitInvariants(Normalize(s)); msg != "" {
			t.Fatalf("输入 %q 违反切句不变量：%s", s, msg)
		}
	})
}

func TestNewSnapshotNormalizesAndVerifies(t *testing.T) {
	snap := NewSnapshot("甲公司" + zwsp + "成立于2010年。  乙公司成立于２０１２年。")
	if snap.Body != "甲公司成立于2010年。 乙公司成立于2012年。" {
		t.Fatalf("快照正文未规范化：%q", snap.Body)
	}
	if len(snap.Sentences) != 2 {
		t.Fatalf("句子数 = %d，期望 2", len(snap.Sentences))
	}
	if err := snap.verify(); err != nil {
		t.Fatalf("合法快照校验失败：%v", err)
	}
}

func TestSnapshotVerifyRejectsTampering(t *testing.T) {
	good := func() *Snapshot { return NewSnapshot("甲。乙。丙。") }
	cases := map[string]func(*Snapshot){
		"正文未规范化":  func(s *Snapshot) { s.Body = "甲" + zwsp + "。乙。丙。" },
		"编号不连续":   func(s *Snapshot) { s.Sentences[1].ID = 5 },
		"区间越界":    func(s *Snapshot) { s.Sentences[2].End = len(s.Body) + 3 },
		"区间重叠":    func(s *Snapshot) { s.Sentences[1].Start = 0 },
		"区间为空":    func(s *Snapshot) { s.Sentences[0].End = s.Sentences[0].Start },
		"切断多字节字符": func(s *Snapshot) { s.Sentences[0].End = s.Sentences[0].Start + 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := good()
			mutate(s)
			if err := s.verify(); !errors.Is(err, ErrSnapshot) {
				t.Fatalf("被篡改的快照应返回 ErrSnapshot，实际 %v", err)
			}
		})
	}
	var nilSnap *Snapshot
	if err := nilSnap.verify(); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("nil 快照应返回 ErrSnapshot，实际 %v", err)
	}
}

func TestSnapshotSegments(t *testing.T) {
	snap := NewSnapshot("第一句。第二句。第三句。第四句。第五句。")
	cases := []struct {
		ids  []int
		want []string
	}{
		{[]int{2}, []string{"第二句。"}},
		{[]int{1, 2}, []string{"第一句。第二句。"}},
		{[]int{1, 3}, []string{"第一句。", "第三句。"}},
		{[]int{1, 2, 4, 5}, []string{"第一句。第二句。", "第四句。第五句。"}},
		{[]int{1, 3, 5}, []string{"第一句。", "第三句。", "第五句。"}},
	}
	for _, c := range cases {
		if got := snap.segments(c.ids); !reflect.DeepEqual(got, c.want) {
			t.Errorf("segments(%v) = %q，期望 %q", c.ids, got, c.want)
		}
	}
}
