package dossier

import "slices"

// CardChange只描述所选档案摘录和字段的差异，不能解释为业务事实或事实更正。
type CardChange struct {
	BeforeKey, AfterKey                                           string
	QuoteChanged, SourceChanged, ReferenceChanged, SectionChanged bool
	Fields                                                        []string
}
type GapChange struct{ BeforeKey, AfterKey string }
type VersionDiff struct {
	Added, NotPresent          []string
	Changed                    []CardChange
	Gaps                       []GapChange
	QuotedSentenceOrderChanged bool
}

type documentKey struct {
	source        int64
	category, url string
}
type quoteKey struct {
	document documentKey
	text     string
}

// CompareVersions按档案版本和编制时点排序，允许新版重新引用旧快照或相同正文。
// 只比较已绑定、已选入档案的摘录；调用者须先完成双方当前授权。
func CompareVersions(before, after *BoundDraft) (VersionDiff, error) {
	a, b := before.Identity(), after.Identity()
	if a.ThemeID <= 0 || a.ThemeID != b.ThemeID || a.Version <= 0 || a.Version >= b.Version || a.CreatedAt <= 0 || a.CreatedAt >= b.CreatedAt || before.Digest() == "" || after.Digest() == "" {
		return VersionDiff{}, ErrInvalidComparison
	}
	out := VersionDiff{Added: []string{}, NotPresent: []string{}, Changed: []CardChange{}, Gaps: []GapChange{}}
	origins := map[int64]Origin{}
	for _, v := range []*BoundDraft{before, after} {
		for _, o := range v.Origins() {
			if old, ok := origins[o.SnapshotID]; ok && old != o {
				return VersionDiff{}, ErrInvalidComparison
			}
			origins[o.SnapshotID] = o
		}
	}
	key := func(c BoundCard) quoteKey {
		o := origins[c.StatementRef.SnapshotID]
		return quoteKey{documentKey{o.SourceID, o.Category, o.URL}, c.Statement}
	}
	old, new := before.Cards(), after.Cards()
	matched := make([]bool, len(new))
	pairs := make([]int, len(old))
	for i := range pairs {
		pairs[i] = -1
	}
	// 优先稳定键的同文配对，再按同文出现次数配对，重命名不制造新增/消失。
	for i, c := range old {
		for j, n := range new {
			if !matched[j] && c.Key == n.Key && key(c) == key(n) {
				pairs[i] = j
				matched[j] = true
				break
			}
		}
	}
	for i, c := range old {
		if pairs[i] >= 0 {
			continue
		}
		for j, n := range new {
			if !matched[j] && key(c) == key(n) {
				pairs[i] = j
				matched[j] = true
				break
			}
		}
	}
	// 剩余稳定键只表示该卡摘录调整，不断言两个表述对应同一个事实。
	for i, c := range old {
		if pairs[i] >= 0 {
			continue
		}
		for j, n := range new {
			if !matched[j] && c.Key == n.Key {
				pairs[i] = j
				matched[j] = true
				break
			}
		}
	}
	for i, c := range old {
		if pairs[i] < 0 {
			out.NotPresent = append(out.NotPresent, c.Key)
			continue
		}
		n := new[pairs[i]]
		change := CardChange{BeforeKey: c.Key, AfterKey: n.Key, QuoteChanged: c.Statement != n.Statement, SourceChanged: key(c).document != key(n).document, ReferenceChanged: c.StatementRef != n.StatementRef, SectionChanged: c.Section != n.Section, Fields: []string{}}
		left := []BoundField{c.Subject, c.PublishedDate, c.EffectiveDate, c.Period, c.Scope, c.Stage, c.Basis, c.Unit}
		right := []BoundField{n.Subject, n.PublishedDate, n.EffectiveDate, n.Period, n.Scope, n.Stage, n.Basis, n.Unit}
		for k, name := range []string{"subject", "publishedDate", "effectiveDate", "period", "scope", "stage", "basis", "unit"} {
			if left[k].State != right[k].State || left[k].Text != right[k].Text || left[k].Ref != right[k].Ref {
				change.Fields = append(change.Fields, name)
			}
		}
		if change.BeforeKey != change.AfterKey || change.QuoteChanged || change.SourceChanged || change.ReferenceChanged || change.SectionChanged || len(change.Fields) > 0 {
			out.Changed = append(out.Changed, change)
		}
	}
	for j, c := range new {
		if !matched[j] {
			out.Added = append(out.Added, c.Key)
		}
	}
	oldGaps, newGaps := before.Gaps(), after.Gaps()
	seen := map[string]bool{}
	for _, g := range oldGaps {
		found := false
		for _, n := range newGaps {
			if n.Key == g.Key {
				found = true
				seen[n.Key] = true
				if n != g {
					out.Gaps = append(out.Gaps, GapChange{g.Key, n.Key})
				}
				break
			}
		}
		if !found {
			out.Gaps = append(out.Gaps, GapChange{BeforeKey: g.Key})
		}
	}
	for _, g := range newGaps {
		if !seen[g.Key] {
			out.Gaps = append(out.Gaps, GapChange{AfterKey: g.Key})
		}
	}
	// 用原文句号位置检测已引用原句的相对顺序，忽略卡片键排序和未引用的全文。
	type sentence struct {
		doc  documentKey
		ref  SentenceRef
		text string
	}
	collect := func(cards []BoundCard) []sentence {
		rows := []sentence{}
		used := map[SentenceRef]bool{}
		documents := map[documentKey]int64{}
		ambiguous := map[documentKey]bool{}
		for _, c := range cards {
			doc := key(c).document
			if id, ok := documents[doc]; ok && id != c.StatementRef.SnapshotID {
				ambiguous[doc] = true
			}
			documents[doc] = c.StatementRef.SnapshotID
		}
		for _, c := range cards {
			// 同一档案引用同文档的多个采集版本时，没有单一原文顺序可供推断。
			if ambiguous[key(c).document] {
				continue
			}
			if !used[c.StatementRef] {
				used[c.StatementRef] = true
				rows = append(rows, sentence{key(c).document, c.StatementRef, c.Statement})
			}
		}
		slices.SortFunc(rows, func(a, b sentence) int {
			if a.ref.Number < b.ref.Number {
				return -1
			}
			if a.ref.Number > b.ref.Number {
				return 1
			}
			return 0
		})
		return rows
	}
	positions := map[quoteKey][]int{}
	for _, s := range collect(new) {
		k := quoteKey{s.doc, s.text}
		positions[k] = append(positions[k], s.ref.Number)
	}
	last := map[documentKey]int{}
	for _, s := range collect(old) {
		k := quoteKey{s.doc, s.text}
		queue := positions[k]
		if len(queue) == 0 {
			continue
		}
		n := queue[0]
		positions[k] = queue[1:]
		if n < last[s.doc] {
			out.QuotedSentenceOrderChanged = true
		}
		last[s.doc] = n
	}
	return out, nil
}
