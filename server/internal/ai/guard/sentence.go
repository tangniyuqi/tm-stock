package guard

import "unicode/utf8"

// Sentence 是快照正文里的一个编号句子。Start、End 是 Body 里的字节偏移（左闭右开），
// Body[Start:End] 即句子原文（含句末标点，不含句间空白与换行）。
type Sentence struct {
	ID    int // 从 1 开始，连续
	Start int
	End   int
}

// isCloser 是可以紧跟在句末标点之后、归属前一个句子的右引号与右括号。
func isCloser(r rune) bool {
	switch r {
	case '”', '’', '」', '』', '）', ')', '】', ']', '》', '〉', '"', '\'':
		return true
	}
	return false
}

// isTerminator 是切句时认的句末符；半角三个在吸收后续重复标点时也算。
func isTerminator(r rune) bool {
	switch r {
	case '。', '！', '？', '；', '!', '?', ';':
		return true
	}
	return false
}

func isDigitRune(r rune) bool { return r >= '0' && r <= '9' }

// SplitSentences 按固定规则切句，句子编号从 1 开始：
//
//   - 。！？； 与换行一定结束一个句子；
//   - 半角 ! ? ; 仅当其后是空白、文本结尾或右引号/右括号时才结束；
//   - 半角 . 仅当其后是空白或文本结尾，且前一个字符不是数字时才结束（不切碎 "12.5"）；
//   - …… 与冒号不结束句子；
//   - 句末标点之后紧跟的右引号、右括号与重复的句末标点归属前一个句子。
//
// 入参应已经过 Normalize（空白已折叠）；对未规范化的文本也能工作，只是句首尾会额外去掉空格。
func SplitSentences(body string) []Sentence {
	type pos struct {
		r rune
		b int // 该字符的字节偏移
	}
	rs := make([]pos, 0, utf8.RuneCountInString(body)+1)
	for i, r := range body {
		rs = append(rs, pos{r, i})
	}
	n := len(rs)
	byteAt := func(k int) int { // 第 k 个字符的字节偏移；k==n 时是正文末尾
		if k >= n {
			return len(body)
		}
		return rs[k].b
	}
	isBoundaryAt := func(k int) bool { // 半角 ! ? ; 之后的"可结束"条件
		return k >= n || rs[k].r == ' ' || rs[k].r == '\n' || isCloser(rs[k].r)
	}

	var out []Sentence
	emit := func(from, to int) {
		for to > from && rs[to-1].r == ' ' {
			to--
		}
		if to <= from {
			return
		}
		out = append(out, Sentence{ID: len(out) + 1, Start: byteAt(from), End: byteAt(to)})
	}

	start := -1
	for i := 0; i < n; {
		r := rs[i].r
		if start < 0 {
			if r == ' ' || r == '\n' {
				i++
				continue
			}
			start = i
		}
		if r == '\n' {
			emit(start, i)
			start = -1
			i++
			continue
		}
		end := false
		switch r {
		case '。', '！', '？', '；':
			end = true
		case '!', '?', ';':
			end = isBoundaryAt(i + 1)
		case '.':
			next := i + 1
			nextOK := next >= n || rs[next].r == ' ' || rs[next].r == '\n'
			end = nextOK && !(i > 0 && isDigitRune(rs[i-1].r))
		}
		if !end {
			i++
			continue
		}
		j := i + 1
		for j < n && (isTerminator(rs[j].r) || isCloser(rs[j].r)) {
			j++
		}
		emit(start, j)
		start = -1
		i = j
	}
	if start >= 0 {
		emit(start, n)
	}
	return out
}

// Snapshot 是一份不可变来源快照：规范化后的正文与它的句子表。
// 抽取阶段模型只能返回句子编号，引文由代码按编号从 Body 取原文。
type Snapshot struct {
	Body      string
	Sentences []Sentence
}

// NewSnapshot 规范化 raw 并切句。快照落库时应保存 Body，之后句子编号与偏移可由 SplitSentences 复现。
func NewSnapshot(raw string) *Snapshot {
	body := Normalize(raw)
	return &Snapshot{Body: body, Sentences: SplitSentences(body)}
}

// verify 检查快照的内部不变量；ValidateExtraction 用它防止调用方绕过 NewSnapshot 传入脏数据。
func (s *Snapshot) verify() error {
	if s == nil {
		return ErrSnapshot
	}
	if Normalize(s.Body) != s.Body {
		return ErrSnapshot
	}
	prevEnd := 0
	for i, st := range s.Sentences {
		if st.ID != i+1 || st.Start < prevEnd || st.End <= st.Start || st.End > len(s.Body) {
			return ErrSnapshot
		}
		if !utf8.ValidString(s.Body[st.Start:st.End]) {
			return ErrSnapshot
		}
		prevEnd = st.End
	}
	return nil
}

// segments 把一组严格升序、已校验的句子编号取成原文段：相邻编号合并成一段连续原文，
// 不相邻则分成多段。调用方须先校验编号合法。
func (s *Snapshot) segments(ids []int) []string {
	var segs []string
	runStart := 0
	for k := 1; k <= len(ids); k++ {
		if k == len(ids) || ids[k] != ids[k-1]+1 {
			first := s.Sentences[ids[runStart]-1]
			last := s.Sentences[ids[k-1]-1]
			segs = append(segs, s.Body[first.Start:last.End])
			runStart = k
		}
	}
	return segs
}
