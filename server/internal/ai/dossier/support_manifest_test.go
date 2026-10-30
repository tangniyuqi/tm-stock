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
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func manifestFixture(t *testing.T) (*SupportManifest, *SupportPreparation, *BoundDraft, []sources.Snapshot) {
	t.Helper()
	d, snapshots := testDraft(t)
	bound, err := BindDraft(d, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareSupport(context.Background(), supportConfig(), "仅测试专题", bound, snapshots, []string{"progress_a"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepared.Manifest(2)
	if err != nil {
		t.Fatal(err)
	}
	return m, prepared, bound, snapshots
}

func TestDossierManifestRoundTrip(t *testing.T) {
	m, prepared, bound, snapshots := manifestFixture(t)
	decoded, err := DecodeSupportManifest(m.Canonical(), m.ComponentDigest())
	if err != nil || !decoded.Valid() || decoded.Identity() != bound.Identity() || decoded.DraftDigest() != bound.Digest() || decoded.AnalysisType() != 2 {
		t.Fatal("清单往返错误", err)
	}
	rebuilt, err := decoded.Rebuild(context.Background(), bound, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	first, err := prepared.BindTask(context.Background(), supportTaskFixture())
	if err != nil {
		t.Fatal(err)
	}
	again, err := rebuilt.BindTask(context.Background(), supportTaskFixture())
	if err != nil || first.InputDigest() != again.InputDigest() || first.OperationKey(supportTaskFixture(), 3) != again.OperationKey(supportTaskFixture(), 3) {
		t.Fatal("重建改变原键", err)
	}
	light, err := prepared.Manifest(1)
	if err != nil || light.ComponentDigest() == m.ComponentDigest() {
		t.Fatal("轻深清单混用")
	}
	for _, private := range []string{snapshots[0].Body, snapshots[0].LicenseRef, `"TaskID"`, `"UserID"`, `"response"`, `"Prompt"`} {
		if private != "" && bytes.Contains(m.Canonical(), []byte(private)) {
			t.Fatal("清单复制正文/许可/执行身份")
		}
	}
	raw := m.Canonical()
	raw[0] = 'x'
	if m.Canonical()[0] != '{' {
		t.Fatal("清单被外部修改")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", m, m), "progress_a") {
		t.Fatal("格式化泄漏")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := m.Rebuild(ctx, bound, snapshots); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("取消仍重建")
	}
	if got, err := m.Rebuild(nil, bound, snapshots); got != nil || err == nil {
		t.Fatal("空上下文仍重建")
	}
	if got, err := prepared.Manifest(0); got != nil || err == nil {
		t.Fatal("错类型清单")
	}
	var none *SupportManifest
	if none.Valid() || none.Canonical() != nil || none.Identity() != (DraftIdentity{}) || none.DraftDigest() != "" || none.ComponentDigest() != "" || none.AnalysisType() != 0 {
		t.Fatal("空清单有定位")
	}
	if got, err := none.Rebuild(context.Background(), bound, snapshots); got != nil || err == nil {
		t.Fatal("空清单重建")
	}
	if _, err := none.Registration(time.Now()); err == nil {
		t.Fatal("空清单登记")
	}
}

func TestDossierManifestRejectsMalformed(t *testing.T) {
	m, _, _, _ := manifestFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"协议", func(d map[string]any) { d["protocol"] = "unknown" }},
		{"类型", func(d map[string]any) { d["analysisType"] = 0 }},
		{"摘要", func(d map[string]any) { d["componentDigest"] = strings.Repeat("f", 64) }},
		{"档案摘要", func(d map[string]any) { d["draftDigest"] = "wrong" }},
		{"题材名", func(d map[string]any) { d["themeName"] = "" }},
		{"无键", func(d map[string]any) { d["keys"] = []any{} }},
		{"多键", func(d map[string]any) { d["keys"] = []any{"a", "b", "c", "d", "e"} }},
		{"重键", func(d map[string]any) { d["keys"] = []any{"a", "a"} }},
		{"逆序", func(d map[string]any) { d["keys"] = []any{"b", "a"} }},
		{"坏键", func(d map[string]any) { d["keys"] = []any{"../a"} }},
		{"无配置", func(d map[string]any) { d["config"] = nil }},
		{"跨安全题材", func(d map[string]any) { d["draft"].(map[string]any)["ThemeID"] = float64(1 << 54) }},
		{"零版本", func(d map[string]any) { d["draft"].(map[string]any)["Version"] = 0 }},
		{"零时点", func(d map[string]any) { d["draft"].(map[string]any)["CreatedAt"] = 0 }},
		{"超时点", func(d map[string]any) { d["draft"].(map[string]any)["CreatedAt"] = float64(253402300800000) }},
		{"未知字段", func(d map[string]any) { d["extra"] = "x" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(m.Canonical(), &doc); err != nil {
				t.Fatal(err)
			}
			tc.mutate(doc)
			// 映射编码也会改变顺序；另用规范文档重编码，使拒绝不只依赖字节顺序。
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if tc.name != "未知字段" {
				var typed supportManifestDocument
				if json.Unmarshal(raw, &typed) == nil {
					raw, err = json.Marshal(typed)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if got, err := DecodeSupportManifest(raw, m.ComponentDigest()); got != nil || err == nil {
				t.Fatal("非法清单通过")
			}
		})
	}
	for i, raw := range [][]byte{nil, []byte(`[]`), append(m.Canonical(), ' '), []byte(strings.Repeat("x", MaxSupportManifestBytes+1)), bytes.Replace(m.Canonical(), []byte(`"protocol":`), []byte(`"Protocol":`), 1), bytes.Replace(m.Canonical(), []byte(`"protocol":`), []byte(`"protocol":null,"protocol":`), 1)} {
		t.Run(fmt.Sprint("raw_", i), func(t *testing.T) {
			if got, err := DecodeSupportManifest(raw, m.ComponentDigest()); got != nil || err == nil {
				t.Fatal("非规范清单通过")
			}
		})
	}
}

func TestDossierManifestRebuildRejectsChangedScope(t *testing.T) {
	m, _, bound, snapshots := manifestFixture(t)
	for _, mode := range []string{"field_set", "model_strategy", "component", "identity", "draft_digest", "missing_source", "changed_body", "nil_draft"} {
		t.Run(mode, func(t *testing.T) {
			doc := m.doc
			doc.Keys = slices.Clone(doc.Keys)
			b := bound
			snaps := append([]sources.Snapshot(nil), snapshots...)
			switch mode {
			case "field_set":
				doc.Keys = []string{"other_card"}
			case "model_strategy":
				doc.Config.Version = "test-only-other"
			case "component":
				doc.ComponentDigest = strings.Repeat("e", 64)
			case "identity":
				doc.Draft.Version++
			case "draft_digest":
				doc.DraftDigest = strings.Repeat("e", 64)
			case "missing_source":
				snaps = nil
			case "changed_body":
				snaps[0].Body += "改变的测试原文"
			case "nil_draft":
				b = nil
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := DecodeSupportManifest(raw, doc.ComponentDigest)
			if err != nil {
				t.Fatal("未取得格式有效的恶意清单", err)
			}
			if got, err := changed.Rebuild(context.Background(), b, snaps); got != nil || err == nil {
				t.Fatal("篡改清单仍重建")
			}
		})
	}
}
func TestDossierManifestRegistration(t *testing.T) {
	m, _, _, _ := manifestFixture(t)
	at := time.UnixMilli(m.Identity().CreatedAt + 1000)
	r, err := m.Registration(at)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.Manifest(); err != nil || got.ComponentDigest() != m.ComponentDigest() {
		t.Fatal("登记往返失败", err)
	}
	for _, mode := range []string{"before_draft", "precision", "future_year", "manifest_digest", "identity", "analysis_type", "canonical"} {
		t.Run(mode, func(t *testing.T) {
			bad := r
			bad.Canonical = bytes.Clone(r.Canonical)
			switch mode {
			case "before_draft":
				bad.CreatedAt = time.UnixMilli(m.Identity().CreatedAt - 1)
			case "precision":
				bad.CreatedAt = bad.CreatedAt.Add(time.Nanosecond)
			case "future_year":
				bad.CreatedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
			case "manifest_digest":
				bad.ManifestDigest = strings.Repeat("f", 64)
			case "identity":
				bad.Identity.Version++
			case "analysis_type":
				bad.AnalysisType = 1
			case "canonical":
				bad.Canonical = append(bad.Canonical, ' ')
				bad.ManifestDigest = task.StageDigest(bad.Canonical)
			}
			if got, err := bad.Manifest(); got != nil || err == nil {
				t.Fatal("篡改登记通过")
			}
		})
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", r, r), "progress_a") {
		t.Fatal("登记格式化泄漏")
	}
}
