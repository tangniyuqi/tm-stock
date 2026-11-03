package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type dossierSourceSpec struct {
	id, source int64
	url, body  string
}

func dossierSourceBound(t *testing.T, version int64, specs []dossierSourceSpec, fieldOnly bool) *dossier.BoundDraft {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snaps := []sources.Snapshot{}
	u := dossier.Field{State: "unknown"}
	d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 7, Version: version, CreatedAt: base.Add(time.Hour).UnixMilli() + version, Cards: []dossier.FactCard{}, Gaps: []dossier.Gap{}}
	for _, spec := range specs {
		s, err := sources.BuildSnapshot(sources.Source{ID: spec.source, Domain: "example.test", Category: sources.CategoryOfficialList, Enabled: true, LicenseTag: "仅合成测试", LicenseRef: "test-fixture:sources"}, spec.url, sources.MediaPlain, []byte(spec.body), base.Add(time.Duration(spec.id)*time.Millisecond), 4096)
		if err != nil {
			t.Fatal(err)
		}
		s.ID = spec.id
		snaps = append(snaps, s)
		line := s.Sentences[0]
		ref := dossier.SentenceRef{SnapshotID: s.ID, Hash: s.Hash, Number: line.Number, StartByte: line.StartByte, EndByte: line.EndByte}
		if fieldOnly && len(d.Cards) > 0 {
			d.Cards[0].Subject = dossier.Field{State: "known", Ref: ref}
			continue
		}
		d.Cards = append(d.Cards, dossier.FactCard{Key: fmt.Sprintf("card_%d", s.ID), Section: "definition", Statement: ref, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u})
	}
	if len(specs) == 0 {
		d.Gaps = append(d.Gaps, dossier.Gap{Key: "missing", Section: "definition", Reason: "source_missing"})
	}
	b, err := dossier.BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDossierSourceRelationsProjection(t *testing.T) {
	first := dossierSourceSpec{1, 10001, "https://example.test/one", "仅合成测试：甲资料。"}
	second := dossierSourceSpec{2, 10002, "https://example.test/two", "仅合成测试：乙资料。"}
	for _, tc := range []struct {
		name  string
		edit  func(*dossierSourceSpec)
		kinds []string
	}{
		{"三个相同关系", func(s *dossierSourceSpec) { s.source = first.source; s.url = first.url; s.body = first.body }, []string{"same_url", "duplicate_body", "same_registered_source"}},
		{"同链接正文变化", func(s *dossierSourceSpec) { s.url = first.url }, []string{"same_url"}},
		{"不同链接同正文", func(s *dossierSourceSpec) { s.body = first.body }, []string{"duplicate_body"}},
		{"同登记来源不同文档", func(s *dossierSourceSpec) { s.source = first.source }, []string{"same_registered_source"}},
		{"不同存档不判独立", func(s *dossierSourceSpec) {}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := second
			tc.edit(&other)
			bound := dossierSourceBound(t, 1, []dossierSourceSpec{other, first}, false)
			view, err := dossierView([]*dossier.BoundDraft{bound})
			if err != nil {
				t.Fatal(err)
			}
			r := view.SourceRelations
			if r.Notice != dossierSourceNotice || r.Independence != "unverified" || r.ReprintChain != "unverified" || r.ExplicitCorrection != "unverified" || r.Groups == nil || len(r.Groups) != len(tc.kinds) {
				t.Fatal("关系状态或分组错误")
			}
			for i, g := range r.Groups {
				if g.Kind != tc.kinds[i] || !reflect.DeepEqual(g.SnapshotIDs, []int64{1, 2}) {
					t.Fatal("分组不稳定或串入其他快照")
				}
			}
			raw, marshalErr := json.Marshal(r)
			if marshalErr != nil {
				t.Fatal("来源关系响应编码失败")
			}
			if strings.Contains(string(raw), "10001") || strings.Contains(string(raw), bound.Origins()[0].Hash) || strings.Contains(string(raw), first.body) {
				t.Fatal("关系响应泄漏内部来源、摘要或正文")
			}
			if len(r.Groups) > 0 {
				r.Groups[0].SnapshotIDs[0] = 99
				again, e := dossierView([]*dossier.BoundDraft{bound})
				if e != nil || again.SourceRelations.Groups[0].SnapshotIDs[0] != 1 {
					t.Fatal("响应修改影响原档案")
				}
			}
		})
	}
	t.Run("多组顺序稳定", func(t *testing.T) {
		a, b := first, second
		c, d := first, second
		c.id, c.source, c.url = 3, 10003, "https://example.test/three"
		d.id, d.source, d.url = 4, 10004, "https://example.test/four"
		bound := dossierSourceBound(t, 1, []dossierSourceSpec{d, b, c, a}, false)
		for range 5 {
			v, e := dossierView([]*dossier.BoundDraft{bound})
			if e != nil || !reflect.DeepEqual(v.SourceRelations.Groups, []dto.AIDossierSourceGroup{{Kind: "duplicate_body", SnapshotIDs: []int64{1, 3}}, {Kind: "duplicate_body", SnapshotIDs: []int64{2, 4}}}) {
				t.Fatal("同正文多组次序不稳定")
			}
		}
	})
	t.Run("字段独有原文也纳入", func(t *testing.T) {
		other := second
		other.source, other.url, other.body = first.source, first.url, first.body
		v, e := dossierView([]*dossier.BoundDraft{dossierSourceBound(t, 1, []dossierSourceSpec{first, other}, true)})
		if e != nil || len(v.SourceRelations.Groups) != 3 || len(v.Versions[0].Cards) != 1 || v.Versions[0].Cards[0].Fields[0].Quote.SnapshotID != 2 {
			t.Fatal("字段出处遗漏或被当成额外事实卡")
		}
	})
	t.Run("两版相同快照去重", func(t *testing.T) {
		v, e := dossierView([]*dossier.BoundDraft{dossierSourceBound(t, 1, []dossierSourceSpec{first}, false), dossierSourceBound(t, 2, []dossierSourceSpec{first}, false)})
		if e != nil || len(v.SourceRelations.Groups) != 0 || len(v.Versions) != 2 {
			t.Fatal("跨版重复同一快照被计为多份资料")
		}
	})
	t.Run("同编号元数据冲突整份拒绝", func(t *testing.T) {
		other := first
		other.url = second.url
		v, e := dossierView([]*dossier.BoundDraft{dossierSourceBound(t, 1, []dossierSourceSpec{first}, false), dossierSourceBound(t, 2, []dossierSourceSpec{other}, false)})
		if !errors.Is(e, dossier.ErrInvalidDraft) || v.Versions != nil || v.SourceRelations.Groups != nil {
			t.Fatal("矛盾存档泄漏部分正文或关系")
		}
	})
	t.Run("仅缺口无关系", func(t *testing.T) {
		v, e := dossierView([]*dossier.BoundDraft{dossierSourceBound(t, 1, nil, false)})
		if e != nil || v.SourceRelations.Groups == nil || len(v.SourceRelations.Groups) != 0 || v.SourceRelations.ExplicitCorrection != "unverified" {
			t.Fatal("资料缺失被当成独立或更正结论")
		}
	})
}
