package dossier

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func testDraft(t *testing.T) (Draft, []sources.Snapshot) {
	t.Helper()
	s := testSnapshot(t, 1, "仅测试：测试主体甲的样品处于送样阶段。")
	unknown := Field{State: "unknown"}
	c := FactCard{Key: "progress_a", Section: "progress", Statement: reference(s, s.Sentences[0]),
		Subject: unknown, PublishedDate: unknown, EffectiveDate: unknown, Period: unknown,
		Scope: unknown, Stage: unknown, Basis: unknown, Unit: unknown}
	c.Subject = fieldFor(t, s, "测试主体甲")
	c.Stage = fieldFor(t, s, "送样")
	return Draft{Protocol: DraftVersion, ThemeID: 7, Version: 1, CreatedAt: s.FetchedAt.Add(time.Minute).UnixMilli(), Cards: []FactCard{c}}, []sources.Snapshot{s}
}

func fieldFor(t *testing.T, s sources.Snapshot, text string) Field {
	t.Helper()
	start := strings.Index(s.Body, text)
	if start < 0 {
		t.Fatal("测试字段不在合成资料中")
	}
	ref := reference(s, s.Sentences[0])
	ref.StartByte, ref.EndByte = start, start+len(text)
	return Field{State: "known", Ref: ref}
}

func TestBindDraftEvidenceAndUnknownFields(t *testing.T) {
	d, snaps := testDraft(t)
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	if len(bound.Digest()) != 64 || len(bound.Canonical()) == 0 {
		t.Fatal("没有稳定存储依据")
	}
	if bound.Identity() != (DraftIdentity{d.ThemeID, d.Version, d.CreatedAt}) {
		t.Fatal("持久化身份不匹配")
	}
	cards := bound.Cards()
	if len(cards) != 1 || cards[0].Statement != snaps[0].Body || cards[0].Subject.Text != "测试主体甲" ||
		cards[0].Stage.Text != "送样" || cards[0].Stage.Context != snaps[0].Body {
		t.Fatal("没有按原文回取字段和上下文")
	}
	for _, f := range []BoundField{cards[0].PublishedDate, cards[0].EffectiveDate, cards[0].Period, cards[0].Scope, cards[0].Basis, cards[0].Unit} {
		if f.State != "unknown" || f.Text != "" || f.Context != "" || f.Ref != (SentenceRef{}) {
			t.Fatal("未知字段被填充")
		}
	}
	for _, v := range []any{d, *bound, cards[0], cards[0].Stage} {
		if strings.Contains(fmt.Sprintf("%+v %#v", v, v), "测试主体甲") {
			t.Fatal("格式化泄露私有原文")
		}
	}
	// 返回值和调用者输入均不可用于事后改变已绑定结果。
	canonical := bound.Canonical()
	canonical[0] = 'x'
	cards[0].Stage.Text = "篡改"
	d.Cards[0].Section = "policy"
	snaps[0].Sentences[0].Text = "篡改"
	if bound.Canonical()[0] != '{' || bound.Cards()[0].Stage.Text != "送样" || bound.Cards()[0].Section != "progress" {
		t.Fatal("绑定结果被外部修改")
	}
}

func TestBindDraftCanonicalOrderingAndDigest(t *testing.T) {
	d, snaps := testDraft(t)
	second := testSnapshot(t, 2, "仅测试：第二份目录记录技术分工。")
	c := d.Cards[0]
	c.Key = "definition_a"
	c.Section = "definition"
	c.Statement = reference(second, second.Sentences[0])
	c.Subject, c.Stage = Field{State: "unknown"}, Field{State: "unknown"}
	d.Cards = append(d.Cards, c)
	snaps = append(snaps, second)
	d.Gaps = []Gap{{"gap_z", "policy", "source_missing"}, {"gap_a", "chain", "basis_unverified"}}
	a, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	d.Cards[0], d.Cards[1] = d.Cards[1], d.Cards[0]
	d.Gaps[0], d.Gaps[1] = d.Gaps[1], d.Gaps[0]
	snaps[0], snaps[1] = snaps[1], snaps[0]
	b, err := BindDraft(d, snaps)
	if err != nil || !bytes.Equal(a.Canonical(), b.Canonical()) || a.Digest() != b.Digest() {
		t.Fatal("输入排序影响规范摘要")
	}
	if d.Cards[0].Key != "definition_a" || a.Cards()[0].Key != "definition_a" {
		t.Fatal("卡片排序不符")
	}
	d.Version++
	changed, err := BindDraft(d, snaps)
	if err != nil || changed.Digest() == a.Digest() {
		t.Fatal("版本未绑定摘要")
	}
	d.Version--
	snaps[0].LicenseRef = "test-fixture:dossier-changed"
	changed, err = BindDraft(d, snaps)
	if err != nil || changed.Digest() == a.Digest() {
		t.Fatal("来源元数据未绑定摘要")
	}
}

func TestBindDraftRejectsInvalidEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Draft, []sources.Snapshot)
	}{
		{"协议", func(d *Draft, s []sources.Snapshot) { d.Protocol = "other" }},
		{"题材", func(d *Draft, s []sources.Snapshot) { d.ThemeID = 0 }},
		{"超安全整数版本", func(d *Draft, s []sources.Snapshot) { d.Version = conversation.MaxID + 1 }},
		{"编制时间", func(d *Draft, s []sources.Snapshot) { d.CreatedAt = 0 }},
		{"编制时间超限", func(d *Draft, s []sources.Snapshot) { d.CreatedAt = 8640000000000001 }},
		{"空版本", func(d *Draft, s []sources.Snapshot) { d.Cards = nil }},
		{"卡片超限", func(d *Draft, s []sources.Snapshot) { d.Cards = make([]FactCard, MaxCards+1) }},
		{"缺口超限", func(d *Draft, s []sources.Snapshot) { d.Gaps = make([]Gap, MaxCards+1) }},
		{"非法卡键", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Key = "任意文本" }},
		{"未知章节", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Section = "other" }},
		{"重复卡片", func(d *Draft, s []sources.Snapshot) { d.Cards = append(d.Cards, d.Cards[0]) }},
		{"缺正文编号", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Statement.Number = 0 }},
		{"正文编号越界", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Statement.Number = 2 }},
		{"假快照", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Statement.SnapshotID = 99 }},
		{"错误哈希", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Statement.Hash = strings.Repeat("0", 64) }},
		{"正文不是整句", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Statement.StartByte += 3 }},
		{"引用负偏移", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Subject.Ref.StartByte = -1 }},
		{"引用越界", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Subject.Ref.EndByte = len(s[0].Body) + 1 }},
		{"空引用", func(d *Draft, s []sources.Snapshot) {
			d.Cards[0].Subject.Ref.EndByte = d.Cards[0].Subject.Ref.StartByte
		}},
		{"切断UTF8", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Subject.Ref.StartByte++ }},
		{"未知字段夹带引用", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Subject.State = "unknown" }},
		{"未知状态", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Subject.State = "pending" }},
		{"漏写未知", func(d *Draft, s []sources.Snapshot) { d.Cards[0].Period = Field{} }},
		{"未知缺口原因", func(d *Draft, s []sources.Snapshot) { d.Gaps = []Gap{{"gap", "policy", "other"}} }},
		{"缺口键重复", func(d *Draft, s []sources.Snapshot) { d.Gaps = []Gap{{d.Cards[0].Key, "policy", "source_missing"}} }},
		{"缺口章节无效", func(d *Draft, s []sources.Snapshot) { d.Gaps = []Gap{{"gap", "other", "source_missing"}} }},
		{"采集晚于编制", func(d *Draft, s []sources.Snapshot) { d.CreatedAt = s[0].FetchedAt.UnixMilli() - 1 }},
		{"采集亚毫秒", func(d *Draft, s []sources.Snapshot) { s[0].FetchedAt = s[0].FetchedAt.Add(time.Nanosecond) }},
		{"快照正文篡改", func(d *Draft, s []sources.Snapshot) { s[0].Body += "篡改" }},
		{"快照编号过大", func(d *Draft, s []sources.Snapshot) { s[0].ID = conversation.MaxID + 1 }},
		{"来源编号无效", func(d *Draft, s []sources.Snapshot) { s[0].SourceID = 0 }},
		{"许可引用过长", func(d *Draft, s []sources.Snapshot) { s[0].LicenseRef = strings.Repeat("x", 513) }},
		{"许可标签过长", func(d *Draft, s []sources.Snapshot) { s[0].LicenseTag = strings.Repeat("x", 129) }},
		{"许可标签非法编码", func(d *Draft, s []sources.Snapshot) { s[0].LicenseTag = string([]byte{0xff}) }},
		{"许可引用非法编码", func(d *Draft, s []sources.Snapshot) { s[0].LicenseRef = string([]byte{0xff}) }},
		{"许可亚毫秒", func(d *Draft, s []sources.Snapshot) {
			v := s[0].FetchedAt.Add(time.Minute + time.Nanosecond)
			s[0].LicenseExpires = &v
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, s := testDraft(t)
			tc.mutate(&d, s)
			b, e := BindDraft(d, s)
			if !errors.Is(e, ErrInvalidDraft) || b != nil {
				t.Fatal("非法候选未拒绝或泄露部分结果")
			}
		})
	}
}

func TestBindDraftSnapshotSet(t *testing.T) {
	for _, kind := range []string{"missing", "duplicate", "unused", "too_many"} {
		t.Run(kind, func(t *testing.T) {
			d, s := testDraft(t)
			switch kind {
			case "missing":
				s = nil
			case "duplicate":
				s = append(s, s[0])
			case "unused":
				s = append(s, testSnapshot(t, 2, "仅测试：未引用资料。"))
			case "too_many":
				s = make([]sources.Snapshot, MaxDraftSnapshots+1)
			}
			if b, e := BindDraft(d, s); e == nil || b != nil {
				t.Fatal("证据集合没有精确匹配")
			}
		})
	}
}

func TestBindDraftGapOnlyAndLimits(t *testing.T) {
	d, s := testDraft(t)
	d.Cards = nil
	d.Gaps = []Gap{{"missing", "policy", "source_missing"}}
	a, e := BindDraft(d, nil)
	if e != nil || len(a.Cards()) != 0 {
		t.Fatal("明确缺口版本失败")
	}
	gaps := a.Gaps()
	if len(gaps) != 1 || gaps[0].Reason != "source_missing" {
		t.Fatal("缺口丢失")
	}
	gaps[0].Reason = "篡改"
	if a.Gaps()[0].Reason != "source_missing" {
		t.Fatal("缺口被外部修改")
	}
	d.Cards = []FactCard{}
	b, e := BindDraft(d, nil)
	if e != nil || a.Digest() != b.Digest() {
		t.Fatal("空数组编码不一致")
	}
	var none *BoundDraft
	if none.Digest() != "" || none.Canonical() != nil || none.Cards() != nil || none.Gaps() != nil || none.Identity() != (DraftIdentity{}) {
		t.Fatal("空结果不能安全读取")
	}
	d, s = testDraft(t)
	card := d.Cards[0]
	d.Cards = nil
	for i := 0; i < MaxCards; i++ {
		c := card
		c.Key = fmt.Sprintf("card_%02d", i)
		d.Cards = append(d.Cards, c)
		d.Gaps = append(d.Gaps, Gap{fmt.Sprintf("gap_%02d", i), "chain", "scope_unverified"})
	}
	if b, e := BindDraft(d, s); e != nil || len(b.Cards()) != MaxCards {
		t.Fatal("合法数量边界失败")
	}
	until := s[0].FetchedAt.Add(time.Hour)
	s[0].LicenseExpires = &until
	if _, e := BindDraft(d, s); e != nil {
		t.Fatal("合法许可时间失败")
	}
}

func TestBindDraftRejectsUnusableQuote(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		field      bool
	}{
		{"超长整句", strings.Repeat("甲", 1000) + "。", false},
		{"超长字段", strings.Repeat("甲", 201) + "。", true},
		{"上下文红线", "仅测试：建议买入该公司，产品处于送样阶段。", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := testDraft(t)
			s := testSnapshot(t, 1, tc.body)
			d.Cards[0].Statement = reference(s, s.Sentences[0])
			d.Cards[0].Subject = Field{State: "unknown"}
			d.Cards[0].Stage = Field{State: "unknown"}
			if tc.field {
				ref := reference(s, s.Sentences[0])
				ref.EndByte -= 3
				d.Cards[0].Subject = Field{State: "known", Ref: ref}
			}
			if b, e := BindDraft(d, []sources.Snapshot{s}); e == nil || b != nil {
				t.Fatal("不可用原文被放行")
			}
		})
	}
}

func TestRestoreDraftRebuildsAndRejectsTampering(t *testing.T) {
	d, s := testDraft(t)
	bound, e := BindDraft(d, s)
	if e != nil {
		t.Fatal(e)
	}
	if restored, e := RestoreDraft(bound.Canonical(), bound.Digest(), s); e != nil || restored.Digest() != bound.Digest() || restored.Cards()[0].Subject.Text != "测试主体甲" {
		t.Fatal("重读没有重建原文")
	}
	for _, tc := range []struct {
		name      string
		modify    func([]byte) []byte
		recompute bool
	}{
		{"改主体编号不重签", func(b []byte) []byte { return bytes.Replace(b, []byte(`"themeId":7`), []byte(`"themeId":8`), 1) }, false},
		{"重复键", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"themeId":7`), []byte(`"themeId":7,"themeId":7`), 1)
		}, true},
		{"未知字段", func(b []byte) []byte { return append([]byte(`{"extra":1,`), b[1:]...) }, true},
		{"大小写别名", func(b []byte) []byte { return bytes.Replace(b, []byte(`"themeId"`), []byte(`"ThemeId"`), 1) }, true},
		{"非规范空白", func(b []byte) []byte { return append(b, ' ') }, true},
		{"额外正文", func(b []byte) []byte { return append(b, []byte(`{}`)...) }, true},
		{"错误字段类型", func(b []byte) []byte { return bytes.Replace(b, []byte(`"themeId":7`), []byte(`"themeId":"7"`), 1) }, true},
		{"不支持版本", func(b []byte) []byte { return bytes.Replace(b, []byte(DraftVersion), []byte("other"), 1) }, true},
		{"篡改证据元数据", func(b []byte) []byte { return bytes.Replace(b, []byte("example.test"), []byte("example.invalid"), 1) }, true},
		{"记录大小超限", func(b []byte) []byte { return make([]byte, MaxDraftRecordBytes+1) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.modify(bound.Canonical())
			digest := bound.Digest()
			if tc.recompute {
				digest = task.StageDigest(raw)
			}
			if b, e := RestoreDraft(raw, digest, s); !errors.Is(e, ErrInvalidDraft) || b != nil {
				t.Fatal("损坏记录被恢复")
			}
		})
	}
	if b, e := RestoreDraft(bound.Canonical(), "", s); e == nil || b != nil {
		t.Fatal("缺预期摘要放行")
	}
	s[0].URL = "https://example.test/changed"
	if b, e := RestoreDraft(bound.Canonical(), bound.Digest(), s); e == nil || b != nil {
		t.Fatal("同编号原文的链接变化未发现")
	}
}

func TestBindDraftBodyBudget(t *testing.T) {
	d, _ := testDraft(t)
	card := d.Cards[0]
	d.Cards = nil
	card.Subject, card.Stage = Field{State: "unknown"}, Field{State: "unknown"}
	prefix := "仅测试。\n"
	body := prefix + strings.Repeat("x", MaxBodyBytes-len(prefix))
	var snaps []sources.Snapshot
	for i := int64(1); i <= 4; i++ {
		s := testSnapshot(t, i, body)
		snaps = append(snaps, s)
		c := card
		c.Key = fmt.Sprintf("card_%d", i)
		c.Statement = reference(s, s.Sentences[0])
		d.Cards = append(d.Cards, c)
	}
	if _, e := BindDraft(d, snaps); e != nil {
		t.Fatal("4MiB合法边界失败")
	}
	snaps = append(snaps, testSnapshot(t, 5, "仅测试第五份。"))
	if b, e := BindDraft(d, snaps); e == nil || b != nil {
		t.Fatal("总正文上限未生效")
	}
}

func TestBindDraftFieldCannotHideBlockedContext(t *testing.T) {
	d, s := testDraft(t)
	bad := testSnapshot(t, 2, "仅测试：建议买入该公司，产品处于送样阶段。")
	d.Cards[0].Stage = fieldFor(t, bad, "送样")
	if b, e := BindDraft(d, append(s, bad)); e == nil || b != nil {
		t.Fatal("截取中性字段绕过完整句守卫")
	}
}
