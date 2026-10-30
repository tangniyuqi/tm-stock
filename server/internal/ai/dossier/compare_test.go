package dossier

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 全部正文为合成测试资料，不表示真实专题或来源许可。
func testSnapshot(t testing.TB, id int64, body string) sources.Snapshot {
	t.Helper()
	s, err := sources.BuildSnapshot(sources.Source{
		ID: 7, Domain: "example.test", Category: sources.CategoryOfficialList,
		Enabled: true, LicenseTag: "仅测试", LicenseRef: "test-fixture:dossier",
	}, "https://example.test/test-only", sources.MediaPlain, []byte(body),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(id)*time.Second), MaxBodyBytes+1)
	if err != nil {
		t.Fatalf("构造合成快照失败：%v", err)
	}
	s.ID = id
	return s
}

func TestCompareSnapshotsTextChanges(t *testing.T) {
	for _, tc := range []struct {
		name, old, new     string
		added, absent      []int
		changed, reordered bool
	}{
		{"同文不同采集", "仅测试甲。", "仅测试甲。", nil, nil, false, false},
		{"后插一句", "仅测试甲。", "仅测试甲。仅测试乙。", []int{2}, nil, true, false},
		{"前插不误判偏移", "仅测试甲。", "仅测试乙。仅测试甲。", []int{1}, nil, true, false},
		{"未再出现不是撤回", "仅测试甲。仅测试乙。", "仅测试甲。", nil, []int{2}, true, false},
		{"替换不自动关联事实", "仅测试甲。", "仅测试乙。", []int{1}, []int{1}, true, false},
		{"仅调整句序", "仅测试甲。仅测试乙。", "仅测试乙。仅测试甲。", nil, nil, true, true},
		{"重复句增加", "仅测试甲。", "仅测试甲。仅测试甲。", []int{2}, nil, true, false},
		{"重复句减少", "仅测试甲。仅测试甲。", "仅测试甲。", nil, []int{2}, true, false},
		{"重复句与换序", "仅测试甲。仅测试乙。仅测试甲。", "仅测试甲。仅测试甲。仅测试乙。", nil, nil, true, true},
		{"句间换行", "仅测试甲。仅测试乙。", "仅测试甲。\n仅测试乙。", nil, nil, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, after := testSnapshot(t, 1, tc.old), testSnapshot(t, 2, tc.new)
			originalBefore, originalAfter := fmt.Sprintf("%#v", before), fmt.Sprintf("%#v", after)
			got, err := CompareSnapshots(before, after)
			if err != nil {
				t.Fatal(err)
			}
			if got.BeforeID != before.ID || got.AfterID != after.ID || got.BeforeHash != before.Hash || got.AfterHash != after.Hash ||
				got.BodyChanged != tc.changed || got.CommonSentenceOrderChanged != tc.reordered {
				t.Fatalf("版本与变化标记不符：%+v", got)
			}
			checkRefs(t, after, got.Added, tc.added)
			checkRefs(t, before, got.NotPresentInNewVersion, tc.absent)
			again, err := CompareSnapshots(before, after)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("重复比较不确定")
			}
			if fmt.Sprintf("%#v", before) != originalBefore || fmt.Sprintf("%#v", after) != originalAfter {
				t.Fatal("比较修改了输入")
			}
			if strings.Contains(fmt.Sprintf("%+v", got), "仅测试") || strings.Contains(fmt.Sprintf("%+v", got), "example.test") {
				t.Fatal("结果夹带正文或链接")
			}
		})
	}
}

func checkRefs(t *testing.T, snapshot sources.Snapshot, refs []SentenceRef, numbers []int) {
	t.Helper()
	if len(refs) != len(numbers) {
		t.Fatalf("引用数量=%d，预期=%d", len(refs), len(numbers))
	}
	for i, ref := range refs {
		sentence := snapshot.Sentences[numbers[i]-1]
		if ref.SnapshotID != snapshot.ID || ref.Hash != snapshot.Hash || ref.Number != numbers[i] ||
			ref.StartByte != sentence.StartByte || ref.EndByte != sentence.EndByte || snapshot.Body[ref.StartByte:ref.EndByte] != sentence.Text {
			t.Fatal("差异无法回取对应版本的原句")
		}
	}
}

func TestCompareSnapshotsRejectsInvalidPairs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*sources.Snapshot, *sources.Snapshot)
	}{
		{"缺快照编号", func(a, b *sources.Snapshot) { a.ID = 0 }},
		{"重复快照编号", func(a, b *sources.Snapshot) { b.ID = a.ID }},
		{"不同来源", func(a, b *sources.Snapshot) { b.SourceID++ }},
		{"同一伪造来源", func(a, b *sources.Snapshot) { a.SourceID = 0; b.SourceID = 0 }},
		{"不同类别", func(a, b *sources.Snapshot) { b.SourceCategory = sources.CategoryAnnualReport }},
		{"不同链接", func(a, b *sources.Snapshot) { b.URL += "-other" }},
		{"颠倒采集时间", func(a, b *sources.Snapshot) { b.FetchedAt = a.FetchedAt.Add(-time.Second) }},
		{"相同采集时间", func(a, b *sources.Snapshot) { b.FetchedAt = a.FetchedAt }},
		{"无效采集时间", func(a, b *sources.Snapshot) { a.FetchedAt = time.Time{} }},
		{"篡改旧正文", func(a, b *sources.Snapshot) { a.Body = "仅测试篡改。" }},
		{"篡改新哈希", func(a, b *sources.Snapshot) { b.Hash = strings.Repeat("0", 64) }},
		{"篡改句子内容", func(a, b *sources.Snapshot) { b.Sentences[0].Text = "仅测试篡改。" }},
		{"篡改句子偏移", func(a, b *sources.Snapshot) { b.Sentences[0].StartByte++ }},
		{"缺句子", func(a, b *sources.Snapshot) { b.Sentences = nil }},
		{"超句子上限", func(a, b *sources.Snapshot) { b.Sentences = make([]sources.Sentence, MaxSentences+1) }},
		{"超正文上限", func(a, b *sources.Snapshot) { b.Body = strings.Repeat("x", MaxBodyBytes+1) }},
		{"缺授权归档", func(a, b *sources.Snapshot) { b.LicenseRef = "" }},
		{"采集时已过期", func(a, b *sources.Snapshot) { until := b.FetchedAt; b.LicenseExpires = &until }},
		{"非HTTPS", func(a, b *sources.Snapshot) { a.URL = "http://example.test/test"; b.URL = a.URL }},
		{"非规范链接", func(a, b *sources.Snapshot) { a.URL += "#page=2"; b.URL = a.URL }},
		{"畸形链接", func(a, b *sources.Snapshot) { a.URL = "https://%"; b.URL = a.URL }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := testSnapshot(t, 1, "仅测试甲。"), testSnapshot(t, 2, "仅测试乙。")
			tc.mutate(&a, &b)
			got, err := CompareSnapshots(a, b)
			if !errors.Is(err, ErrInvalidComparison) || !reflect.DeepEqual(got, SnapshotDiff{}) {
				t.Fatal("非法输入未拒绝或返回了部分结果")
			}
		})
	}
}

func TestCompareSnapshotsLimits(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"句子上限", strings.Repeat("仅测试。", MaxSentences)},
		{"字节上限", strings.Repeat("x", MaxBodyBytes-3) + "。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := testSnapshot(t, 1, tc.body), testSnapshot(t, 2, tc.body)
			got, err := CompareSnapshots(a, b)
			if err != nil || got.BodyChanged || len(got.Added) != 0 || len(got.NotPresentInNewVersion) != 0 {
				t.Fatal("合法边界比较失败")
			}
		})
	}
}
