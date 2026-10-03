package guard

import (
	"strings"
	"testing"
)

// 本文件的用例用词表里的词（bad、badSecond）运行时拼出含红线词的文本，
// 避免在 .go 源码里直接写出词表词。更多正反例在 testdata/own_cases.jsonl 与 eval/ 夹具里。

func TestScreenQuotePass(t *testing.T) {
	q := "星河科技主营算力服务。  2025年度营业收入12.3亿元，同比增长8.5%。"
	got := ScreenQuote(q)
	if got.Action != Pass || got.Text != "星河科技主营算力服务。 2025年度营业收入12.3亿元,同比增长8.5%。" {
		t.Fatalf("干净引文应 Pass 并返回规范化文本，实际 %+v", got)
	}
	if len(got.Violations) != 0 {
		t.Errorf("Pass 不应带命中：%+v", got.Violations)
	}
	if got := ScreenQuote(""); got.Action != Pass || got.Text != "" {
		t.Errorf("空引文：%+v", got)
	}
}

func TestScreenQuoteTrimsToLongestCleanSentenceRun(t *testing.T) {
	w := bad(t)
	// 三句：干净（12 字）、违规、干净（13 字）：最长的干净段是第三句
	q := "星河科技主营算力服务。该公司是" + w + "。营业收入同比增长百分之八。"
	got := ScreenQuote(q)
	if got.Action != Trim || got.Text != "营业收入同比增长百分之八。" {
		t.Fatalf("应截取第三句，实际 %+v", got)
	}
	if len(got.Violations) == 0 {
		t.Error("Trim 应保留对整条引文的命中，供留痕")
	}
	if Blocked(got.Text) {
		t.Error("截取结果自身不得再命中红线")
	}
	if !strings.Contains(Normalize(q), got.Text) {
		t.Error("截取结果必须是原文的连续子串")
	}
}

func TestScreenQuoteTieBreaksToEarlierRun(t *testing.T) {
	w := bad(t)
	// 两段干净句等长：取靠前的
	q := "星河科技主营算力。" + w + "。云岫算力主营光伏。"
	got := ScreenQuote(q)
	if got.Action != Trim || got.Text != "星河科技主营算力。" {
		t.Fatalf("并列时应取靠前的一段，实际 %+v", got)
	}
}

func TestScreenQuoteKeepsContiguousRunTogether(t *testing.T) {
	w := bad(t)
	q := w + "。星河科技主营算力服务。营业收入同比增长百分之八。"
	got := ScreenQuote(q)
	if got.Action != Trim || got.Text != "星河科技主营算力服务。营业收入同比增长百分之八。" {
		t.Fatalf("连续的干净句应作为一段整体保留，实际 %+v", got)
	}
}

func TestScreenQuoteFallsBackToClauses(t *testing.T) {
	w := bad(t)
	// 单句且整句违规：退到分句粒度，最长的干净分句段是"主营算力服务与数据中心运营"
	q := "星河科技是" + w + "，主营算力服务与数据中心运营"
	got := ScreenQuote(q)
	if got.Action != Trim || got.Text != "主营算力服务与数据中心运营" {
		t.Fatalf("应退到分句粒度截取，实际 %+v", got)
	}
}

func TestScreenQuoteHumanCases(t *testing.T) {
	w := bad(t)
	w2 := badSecond(t)
	cases := []struct {
		name string
		q    string
	}{
		{"全文只有违规内容", "星河科技是" + w + "。"},
		{"每句都违规", "星河科技是" + w + "。云岫算力被称为" + w2 + "。"},
		{"截取结果太短", "甲是" + w + "，乙乙乙"},
		{"截取结果以依附性连接词开头", "星河科技是" + w + "。其中，算力业务保持增长。"},
		{"截取结果以该字开头", w + "。该公司主营算力服务。"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ScreenQuote(c.q)
			if got.Action != Human || got.Text != "" {
				t.Fatalf("应转人工，实际 %+v", got)
			}
			if len(got.Violations) == 0 {
				t.Error("Human 应带命中供人工复核")
			}
		})
	}
}

// 跨换行拆开的红线词：逐句检测看不到，必须靠"截取结果再检测一遍"兜住，不能把它当干净句展示。
func TestScreenQuoteRecheckCatchesCrossLineWord(t *testing.T) {
	w := []rune(bad(t))
	if len(w) < 2 {
		t.Skip("首个词不足两个字")
	}
	q := "星河科技主营算力服务" + string(w[:1]) + "\n" + string(w[1:]) + "，营业收入稳定"
	got := ScreenQuote(q)
	if got.Action == Pass {
		t.Fatalf("跨行拆开的红线词不得 Pass：%+v", got)
	}
	if got.Action == Trim && Blocked(got.Text) {
		t.Fatalf("Trim 结果不得再命中红线：%+v", got)
	}
}

func TestActionString(t *testing.T) {
	for a, want := range map[Action]string{Pass: "pass", Trim: "trim", Human: "human", Action(99): "unknown"} {
		if a.String() != want {
			t.Errorf("Action(%d).String() = %q，期望 %q", a, a.String(), want)
		}
	}
}

func TestStartsDependent(t *testing.T) {
	for s, want := range map[string]bool{
		"但是公司仍然亏损": true, "然而市场不同": true, "该公司主营算力": true, "其中算力业务增长": true,
		"星河科技主营算力": false, "营业收入增长": false, "公司拥有数据中心": false,
	} {
		if got := startsDependent(s); got != want {
			t.Errorf("startsDependent(%q) = %v，期望 %v", s, got, want)
		}
	}
}

func TestClauseSpans(t *testing.T) {
	q := "甲公司主营算力,乙公司主营光伏、丙公司主营储能:详见公告。第二句,结束"
	var got []string
	for _, sp := range clauseSpans(q) {
		got = append(got, q[sp.start:sp.end])
	}
	want := []string{"甲公司主营算力", "乙公司主营光伏", "丙公司主营储能", "详见公告。", "第二句", "结束"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("clauseSpans = %q，期望 %q", got, want)
	}
}
