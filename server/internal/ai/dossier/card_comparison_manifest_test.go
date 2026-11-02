package dossier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDossierComparisonManifestRoundTrip(t *testing.T) {
	d, snaps := comparisonDraft(t, nil)
	p := comparisonPreparation(t, comparisonConfig(), d, snaps)
	m, err := p.Manifest()
	if err != nil || !m.Valid() || m.AnalysisType() != task.DeepAnalysisType || m.Identity() != p.base.identity || m.DraftDigest() != p.base.draftDigest {
		t.Fatal("清单未绑定比较定位", err)
	}
	for _, private := range []string{snaps[0].Body, snaps[0].LicenseRef, supportTaskFixture().No, "userId", "inputDigest", "response"} {
		if bytes.Contains(m.Canonical(), []byte(private)) {
			t.Fatal("清单保存了正文、许可或执行身份")
		}
	}
	bound, err := BindDraft(d, snaps)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := m.Rebuild(context.Background(), bound, snaps)
	if err != nil || rebuilt.digest != p.digest {
		t.Fatal("重取原文不能恢复原组件", err)
	}
	one, err := p.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	two, err := rebuilt.BindTask(context.Background(), supportTaskFixture())
	if err != nil || one.InputDigest() != two.InputDigest() || one.OperationKey(supportTaskFixture(), 5) != two.OperationKey(supportTaskFixture(), 5) {
		t.Fatal("重建改变原输入或操作键", err)
	}
	canonical := m.Canonical()
	canonical[0] = 'x'
	if m.Canonical()[0] != '{' || strings.Contains(fmt.Sprintf("%+v %#v", *m, *m), m.DraftDigest()) {
		t.Fatal("清单副本/隐藏无效")
	}
	for _, change := range []func(*cardComparisonManifestDocument){
		func(d *cardComparisonManifestDocument) { d.Config.Focus = "definition" },
		func(d *cardComparisonManifestDocument) { d.Config.Model.Version = "test-only-v2" },
		func(d *cardComparisonManifestDocument) { d.Draft.Version++ },
	} {
		doc := m.doc
		change(&doc)
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		changed, err := DecodeCardComparisonManifest(raw, m.ComponentDigest())
		if err != nil {
			t.Fatal("合法配置变化应在重建时核组件", err)
		}
		if _, err := changed.Rebuild(context.Background(), bound, snaps); !errors.Is(err, ErrCardComparison) {
			t.Fatal("新版本/策略冒用原组件", err)
		}
	}
	badSnaps := slices.Clone(snaps)
	badSnaps[0].Body += "篡改"
	if _, err := m.Rebuild(context.Background(), bound, badSnaps); !errors.Is(err, ErrCardComparison) {
		t.Fatal("只凭清单不回核正文", err)
	}
	if _, err := m.Rebuild(nil, bound, snaps); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Rebuild(canceled, bound, snaps); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDossierComparisonManifestRejects(t *testing.T) {
	d, snaps := comparisonDraft(t, nil)
	m, err := comparisonPreparation(t, comparisonConfig(), d, snaps).Manifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*cardComparisonManifestDocument)
	}{
		{"旧协议", func(d *cardComparisonManifestDocument) { d.Protocol = SupportManifestVersion }},
		{"轻类型", func(d *cardComparisonManifestDocument) { d.AnalysisType = task.LightAnalysisType }},
		{"零题材", func(d *cardComparisonManifestDocument) { d.Draft.ThemeID = 0 }},
		{"零版本", func(d *cardComparisonManifestDocument) { d.Draft.Version = 0 }},
		{"时点越界", func(d *cardComparisonManifestDocument) { d.Draft.CreatedAt = 253402300800000 }},
		{"档案摘要", func(d *cardComparisonManifestDocument) { d.DraftDigest = "bad" }},
		{"空题材名", func(d *cardComparisonManifestDocument) { d.ThemeName = "" }},
		{"缺卡", func(d *cardComparisonManifestDocument) { d.Keys = d.Keys[:1] }},
		{"重复卡", func(d *cardComparisonManifestDocument) { d.Keys[1] = d.Keys[0] }},
		{"卡顺序", func(d *cardComparisonManifestDocument) { slices.Reverse(d.Keys) }},
		{"非法卡键", func(d *cardComparisonManifestDocument) { d.Keys[0] = "../other" }},
		{"错误步位", func(d *cardComparisonManifestDocument) { d.Config.ReviewStep = 4 }},
		{"未知焦点", func(d *cardComparisonManifestDocument) { d.Config.Focus = "auto" }},
		{"错组件", func(d *cardComparisonManifestDocument) { d.ComponentDigest = strings.Repeat("a", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := m.doc
			doc.Keys = slices.Clone(m.doc.Keys)
			tc.change(&doc)
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeCardComparisonManifest(raw, m.ComponentDigest()); !errors.Is(err, ErrCardComparison) {
				t.Fatal("无效清单被接受", err)
			}
		})
	}
	for _, raw := range [][]byte{nil, []byte("null"), []byte(strings.Repeat("x", MaxSupportManifestBytes+1)), append([]byte(" "), m.Canonical()...), bytes.Replace(m.Canonical(), []byte(`"protocol":`), []byte(`"Protocol":`), 1), append(bytes.TrimSuffix(m.Canonical(), []byte("}")), []byte(`,"extra":true}`)...)} {
		if _, err := DecodeCardComparisonManifest(raw, m.ComponentDigest()); !errors.Is(err, ErrCardComparison) {
			t.Fatal("畸形或非规范清单被接受", err)
		}
	}
	var empty *CardComparisonManifest
	if empty.Valid() || empty.Canonical() != nil || empty.Identity() != (DraftIdentity{}) || empty.DraftDigest() != "" || empty.ComponentDigest() != "" || empty.AnalysisType() != 0 {
		t.Fatal("空清单提供身份")
	}
	if _, err := empty.Rebuild(context.Background(), nil, nil); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	if _, err := empty.Registration(time.Now()); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
	if _, err := (*CardComparisonPreparation)(nil).Manifest(); !errors.Is(err, ErrCardComparison) {
		t.Fatal(err)
	}
}

func TestDossierComparisonManifestRegistration(t *testing.T) {
	d, snaps := comparisonDraft(t, nil)
	p := comparisonPreparation(t, comparisonConfig(), d, snaps)
	m, err := p.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	at := time.UnixMilli(m.Identity().CreatedAt).Add(time.Minute)
	r, err := m.Registration(at)
	if err != nil {
		t.Fatal(err)
	}
	if digest, err := r.ValidatedDraftDigest(); err != nil || digest != m.DraftDigest() {
		t.Fatal("通用仓储未核新记录", err)
	}
	if _, err := r.Manifest(); !errors.Is(err, ErrSupportReview) {
		t.Fatal("旧解码器接受新清单", err)
	}
	old, err := p.base.Manifest(task.DeepAnalysisType)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := old.Registration(at)
	if err != nil {
		t.Fatal(err)
	}
	if digest, err := legacy.ValidatedDraftDigest(); err != nil || digest != old.DraftDigest() {
		t.Fatal("旧记录兼容失败", err)
	}
	if _, err := legacy.CardComparisonManifest(); !errors.Is(err, ErrCardComparison) {
		t.Fatal("新解码器接受旧清单", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*SupportRegistration)
	}{
		{"时间早于档案", func(r *SupportRegistration) { r.CreatedAt = time.UnixMilli(r.Identity.CreatedAt - 1) }},
		{"非毫秒时点", func(r *SupportRegistration) { r.CreatedAt = r.CreatedAt.Add(time.Nanosecond) }},
		{"记录身份", func(r *SupportRegistration) { r.Identity.Version++ }},
		{"记录类型", func(r *SupportRegistration) { r.AnalysisType = task.LightAnalysisType }},
		{"记录哈希", func(r *SupportRegistration) { r.ManifestDigest = strings.Repeat("a", 64) }},
		{"未知协议", func(r *SupportRegistration) {
			r.Canonical = bytes.Replace(r.Canonical, []byte(CardComparisonManifestVersion), []byte("test-only-unknown"), 1)
			r.ManifestDigest = task.StageDigest(r.Canonical)
		}},
		{"坏JSON", func(r *SupportRegistration) { r.Canonical = []byte("{") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := r
			bad.Canonical = bytes.Clone(r.Canonical)
			tc.change(&bad)
			if _, err := bad.ValidatedDraftDigest(); err == nil {
				t.Fatal("仓储元数据验证放行篡改记录")
			}
		})
	}
}
