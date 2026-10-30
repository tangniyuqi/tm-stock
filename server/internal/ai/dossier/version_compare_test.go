package dossier

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func versionFixture(t *testing.T, v int64, s sources.Snapshot, keys []string, edit func(*Draft)) *BoundDraft {
	t.Helper()
	u := Field{State: "unknown"}
	d := Draft{Protocol: DraftVersion, ThemeID: 7, Version: v, CreatedAt: time.Date(2026, 1, 1, 1, 0, int(v), 0, time.UTC).UnixMilli()}
	for i, key := range keys {
		d.Cards = append(d.Cards, FactCard{Key: key, Section: "progress", Statement: reference(s, s.Sentences[i]), Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u})
	}
	if edit != nil {
		edit(&d)
	}
	var snaps []sources.Snapshot
	if len(keys) > 0 {
		snaps = []sources.Snapshot{s}
	}
	b, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestCompareDossierVersions(t *testing.T) {
	for _, c := range []struct {
		name, old, new         string
		oldKeys, newKeys       []string
		added, absent, changed int
		order                  bool
	}{
		{"同文新版", "仅测试甲。", "仅测试甲。", []string{"a"}, []string{"a"}, 0, 0, 1, false},
		{"新增摘录", "仅测试甲。", "仅测试甲。仅测试乙。", []string{"a"}, []string{"a", "b"}, 1, 0, 1, false},
		{"本版未出现", "仅测试甲。仅测试乙。", "仅测试甲。", []string{"a", "b"}, []string{"a"}, 0, 1, 1, false},
		{"同键摘录调整", "仅测试甲。", "仅测试乙。", []string{"a"}, []string{"a"}, 0, 0, 1, false},
		{"句序调整", "仅测试甲。仅测试乙。", "仅测试乙。仅测试甲。", []string{"a", "b"}, []string{"b", "a"}, 0, 0, 2, true},
		{"改键保留同文", "仅测试甲。", "仅测试甲。", []string{"a"}, []string{"b"}, 0, 0, 1, false},
		{"重复句增加", "仅测试甲。", "仅测试甲。仅测试甲。", []string{"a"}, []string{"a", "b"}, 1, 0, 1, false},
		{"重复句减少", "仅测试甲。仅测试甲。", "仅测试甲。", []string{"a", "b"}, []string{"a"}, 0, 1, 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := versionFixture(t, 1, testSnapshot(t, 1, c.old), c.oldKeys, nil)
			b := versionFixture(t, 2, testSnapshot(t, 2, c.new), c.newKeys, nil)
			before, after := a.Canonical(), b.Canonical()
			got, err := CompareVersions(a, b)
			if err != nil || len(got.Added) != c.added || len(got.NotPresent) != c.absent || len(got.Changed) != c.changed || got.QuotedSentenceOrderChanged != c.order {
				t.Fatal("比较分类错误", err, got)
			}
			again, err := CompareVersions(a, b)
			if err != nil || !reflect.DeepEqual(got, again) || !reflect.DeepEqual(before, a.Canonical()) || !reflect.DeepEqual(after, b.Canonical()) {
				t.Fatal("比较不确定或修改了输入")
			}
		})
	}
}
func TestCompareDossierReusedSnapshotsAndMetadata(t *testing.T) {
	s := testSnapshot(t, 1, "仅测试甲。")
	a := versionFixture(t, 1, s, []string{"a"}, nil)
	t.Run("同编号快照元数据冲突", func(t *testing.T) {
		b := versionFixture(t, 2, testSnapshot(t, 1, "仅测试乙。"), []string{"a"}, nil)
		if d, e := CompareVersions(a, b); e == nil || !reflect.DeepEqual(d, VersionDiff{}) {
			t.Fatal("混同两个不同的快照原文")
		}
	})
	t.Run("同文档多采集不推断句序", func(t *testing.T) {
		first := testSnapshot(t, 1, "仅测试甲。")
		second := testSnapshot(t, 2, "仅测试甲。仅测试乙。")
		d, _ := testDraft(t)
		d.Cards[0].Statement = reference(first, first.Sentences[0])
		d.Cards[0].Key = "a"
		d.Cards[0].Subject = Field{State: "unknown"}
		d.Cards[0].Stage = Field{State: "unknown"}
		c := d.Cards[0]
		c.Key = "b"
		c.Statement = reference(second, second.Sentences[1])
		d.Cards = append(d.Cards, c)
		before, e := BindDraft(d, []sources.Snapshot{first, second})
		if e != nil {
			t.Fatal(e)
		}
		after := versionFixture(t, 2, testSnapshot(t, 3, "仅测试乙。仅测试甲。"), []string{"b", "a"}, nil)
		if diff, e := CompareVersions(before, after); e != nil || diff.QuotedSentenceOrderChanged {
			t.Fatal("多个采集版本不存在单一旧顺序", e)
		}
	})
	t.Run("同快照编制升级", func(t *testing.T) {
		b := versionFixture(t, 2, s, []string{"a"}, nil)
		d, e := CompareVersions(a, b)
		if e != nil || len(d.Added)+len(d.NotPresent)+len(d.Changed) != 0 || d.QuotedSentenceOrderChanged {
			t.Fatal("同快照误判变化")
		}
	})
	t.Run("旧快照回用", func(t *testing.T) {
		b := versionFixture(t, 2, testSnapshot(t, 2, "仅测试乙。"), []string{"a"}, nil)
		c := versionFixture(t, 3, s, []string{"a"}, nil)
		d, e := CompareVersions(b, c)
		if e != nil || len(d.Changed) != 1 || !d.Changed[0].QuoteChanged {
			t.Fatal("不能比较A到B到A")
		}
	})
	t.Run("已知字段变未知", func(t *testing.T) {
		b := versionFixture(t, 2, s, []string{"a"}, func(d *Draft) { d.Cards[0].Subject = Field{State: "known", Ref: d.Cards[0].Statement} })
		d, e := CompareVersions(b, versionFixture(t, 3, s, []string{"a"}, nil))
		if e != nil || len(d.Changed) != 1 || !reflect.DeepEqual(d.Changed[0].Fields, []string{"subject"}) {
			t.Fatal("未知字段被当成删除事实")
		}
	})
	t.Run("出处调整", func(t *testing.T) {
		s2 := s
		s2.ID = 2
		s2.URL = "https://example.test/another"
		b := versionFixture(t, 2, s2, []string{"a"}, nil)
		d, e := CompareVersions(a, b)
		if e != nil || len(d.Changed) != 1 || !d.Changed[0].SourceChanged || d.Changed[0].QuoteChanged {
			t.Fatal("出处变化被当成正文变化")
		}
	})
	t.Run("栏目调整", func(t *testing.T) {
		b := versionFixture(t, 2, s, []string{"a"}, func(d *Draft) { d.Cards[0].Section = "policy" })
		d, e := CompareVersions(a, b)
		if e != nil || len(d.Changed) != 1 || !d.Changed[0].SectionChanged {
			t.Fatal("栏目变化丢失")
		}
	})
	t.Run("缺口调整", func(t *testing.T) {
		gap := func(reason string) func(*Draft) {
			return func(d *Draft) { d.Gaps = []Gap{{"gap_a", "progress", reason}} }
		}
		b := versionFixture(t, 2, s, []string{"a"}, gap("time_unverified"))
		c := versionFixture(t, 3, s, []string{"a"}, gap("stage_unverified"))
		for _, pair := range [][2]*BoundDraft{{a, b}, {b, c}, {c, versionFixture(t, 4, s, []string{"a"}, nil)}} {
			d, e := CompareVersions(pair[0], pair[1])
			if e != nil || len(d.Gaps) != 1 {
				t.Fatal("缺口差异丢失")
			}
		}
	})
	for i, pair := range [][2]*BoundDraft{{nil, a}, {a, nil}, {a, a}, {versionFixture(t, 2, s, []string{"a"}, nil), a}, {a, versionFixture(t, 2, s, []string{"a"}, func(d *Draft) { d.ThemeID = 8 })}} {
		t.Run(fmt.Sprintf("拒绝配对%d", i), func(t *testing.T) {
			if d, e := CompareVersions(pair[0], pair[1]); e == nil || !reflect.DeepEqual(d, VersionDiff{}) {
				t.Fatal("无效配对未清空")
			}
		})
	}
}
