package materialpack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type fixture struct {
	scope     Scope
	approval  config.AILocalAgentMaterial
	registry  []sources.Source
	snapshots []sources.Snapshot
	now       time.Time
}

// 固定测试资料，不发网络请求、不使用生产批准或模型。
func newFixture() fixture {
	now := time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	body := "仅供测试的公开资料。"
	sum := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(sum[:])
	s := sources.Snapshot{ID: 11, SourceID: 1, SourceCategory: sources.CategoryAnnouncement, URL: "https://example.com/test", Hash: hash, Body: body, Sentences: []sources.Sentence{{Number: 1, StartByte: 0, EndByte: len(body), Text: body}}, LicenseTag: "测试许可", LicenseRef: "测试归档", LicenseExpires: &expires, FetchedAt: now.Add(-time.Minute)}
	return fixture{
		scope:    Scope{MemberID: 1, SubjectType: "theme", SubjectID: 2, SubjectName: "测试题材", ScopeApproval: "local-eval:scope", IssuedAtMS: now.Add(-time.Hour).UnixMilli(), ExpiresAtMS: expires.UnixMilli()},
		approval: config.AILocalAgentMaterial{Version: 1, InstanceID: "test-instance", Revision: "v1", Scope: config.AgentMaterialScope, IssuedAtMS: now.Add(-time.Hour).UnixMilli(), ExpiresAtMS: expires.UnixMilli(), Members: []int64{1}, Use: config.AILocalAgentMaterialUse{DatasetDigest: strings.Repeat("a", 64), ApprovalRef: "local-eval:agent", Purpose: config.AgentMaterialPurpose, Bindings: []config.AILocalAgentMaterialBinding{{SubjectType: "theme", SubjectID: 2, ScopeApproval: "local-eval:scope", Evidence: []config.AILocalAgentMaterialEvidence{{SnapshotID: 11, Hash: hash}}}}}, Limits: config.AILocalAgentMaterialLimits{MaxSources: 2, MaxEvidenceBytes: 1024}},
		registry: []sources.Source{{ID: 1, Domain: "example.com", Category: sources.CategoryAnnouncement, LicenseTag: s.LicenseTag, LicenseRef: s.LicenseRef, LicenseExpires: &expires, Enabled: true}}, snapshots: []sources.Snapshot{s}, now: now,
	}
}
func (f fixture) build() (*Pack, error) {
	return New(f.scope, f.approval, f.registry, f.snapshots, f.now)
}

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*fixture)
		valid  bool
	}{
		{"合法题材", func(f *fixture) {}, true},
		{"合法专题", func(f *fixture) { f.scope.SubjectType = "topic"; f.approval.Use.Bindings[0].SubjectType = "topic" }, true},
		{"大小恰好", func(f *fixture) { f.approval.Limits.MaxEvidenceBytes = len(f.snapshots[0].Body) }, true},
		{"无到期许可", func(f *fixture) { f.registry[0].LicenseExpires = nil; f.snapshots[0].LicenseExpires = nil }, true},
		{"会员不符", func(f *fixture) { f.scope.MemberID = 3 }, false},
		{"对象不符", func(f *fixture) { f.scope.SubjectID = 3 }, false},
		{"空名称", func(f *fixture) { f.scope.SubjectName = " " }, false},
		{"错误对象类型", func(f *fixture) { f.scope.SubjectType = "stock" }, false},
		{"不安全编号", func(f *fixture) { f.scope.MemberID = 9007199254740992 }, false},
		{"范围不符", func(f *fixture) { f.scope.ScopeApproval = "local-eval:other" }, false},
		{"范围到期边界", func(f *fixture) { f.scope.ExpiresAtMS = f.now.UnixMilli() }, false},
		{"范围尚未生效", func(f *fixture) { f.scope.IssuedAtMS = f.now.Add(time.Second).UnixMilli() }, false},
		{"批准到期边界", func(f *fixture) { f.approval.ExpiresAtMS = f.now.UnixMilli() }, false},
		{"批准尚未生效", func(f *fixture) { f.approval.IssuedAtMS = f.now.Add(time.Second).UnixMilli() }, false},
		{"问答用途拒绝", func(f *fixture) { f.approval.Use.Purpose = "bounded-material-qa" }, false},
		{"错误批准范围", func(f *fixture) { f.approval.Scope = "qa" }, false},
		{"错误批准版本", func(f *fixture) { f.approval.Version = 2 }, false},
		{"空实例", func(f *fixture) { f.approval.InstanceID = "" }, false},
		{"重复会员", func(f *fixture) { f.approval.Members = []int64{1, 1} }, false},
		{"范围早于批准", func(f *fixture) { f.scope.IssuedAtMS-- }, false},
		{"范围晚于批准", func(f *fixture) { f.scope.ExpiresAtMS++ }, false},
		{"许可未覆盖范围", func(f *fixture) {
			v := f.now.Add(time.Minute)
			f.registry[0].LicenseExpires = &v
			f.snapshots[0].LicenseExpires = &v
		}, false},
		{"范围许可等于边界", func(f *fixture) {}, true},
		{"会员数量超限", func(f *fixture) {
			for i := int64(2); i <= 17; i++ {
				f.approval.Members = append(f.approval.Members, i)
			}
		}, false},
		{"重复绑定", func(f *fixture) {
			f.approval.Use.Bindings = append(f.approval.Use.Bindings, f.approval.Use.Bindings[0])
		}, false},
		{"重复批准快照", func(f *fixture) { b := &f.approval.Use.Bindings[0]; b.Evidence = append(b.Evidence, b.Evidence[0]) }, false},
		{"缺失快照", func(f *fixture) { f.snapshots = nil }, false},
		{"额外快照", func(f *fixture) { f.snapshots = append(f.snapshots, f.snapshots[0]) }, false},
		{"替换快照", func(f *fixture) { f.snapshots[0].ID = 12 }, false},
		{"批准哈希不符", func(f *fixture) { f.approval.Use.Bindings[0].Evidence[0].Hash = strings.Repeat("b", 64) }, false},
		{"正文哈希不符", func(f *fixture) { f.snapshots[0].Body = "篡改" }, false},
		{"句子偏移无效", func(f *fixture) { f.snapshots[0].Sentences[0].EndByte-- }, false},
		{"来源缺失", func(f *fixture) { f.registry = nil }, false},
		{"来源重复", func(f *fixture) { f.registry = append(f.registry, f.registry[0]) }, false},
		{"来源停用", func(f *fixture) { f.registry[0].Enabled = false }, false},
		{"许可缺失", func(f *fixture) { f.registry[0].LicenseRef = "" }, false},
		{"许可到期边界", func(f *fixture) { v := f.now; f.registry[0].LicenseExpires = &v; f.snapshots[0].LicenseExpires = &v }, false},
		{"许可引用漂移", func(f *fixture) { f.registry[0].LicenseRef = "其他许可" }, false},
		{"许可标签漂移", func(f *fixture) { f.registry[0].LicenseTag = "其他标签" }, false},
		{"许可期限漂移", func(f *fixture) { f.registry[0].LicenseExpires = nil }, false},
		{"来源类别漂移", func(f *fixture) { f.registry[0].Category = sources.CategoryAnnualReport }, false},
		{"跨域URL", func(f *fixture) { f.snapshots[0].URL = "https://other.example/test" }, false},
		{"凭据URL", func(f *fixture) { f.snapshots[0].URL = "https://example.com/test?token=secret" }, false},
		{"非规范URL", func(f *fixture) { f.snapshots[0].URL += "#fragment" }, false},
		{"未来采集", func(f *fixture) { f.snapshots[0].FetchedAt = f.now.Add(time.Second) }, false},
		{"空采集时点", func(f *fixture) { f.snapshots[0].FetchedAt = time.Time{} }, false},
		{"总正文超限", func(f *fixture) { f.approval.Limits.MaxEvidenceBytes = len(f.snapshots[0].Body) - 1 }, false},
		{"快照上限非法", func(f *fixture) { f.approval.Limits.MaxSources = 9 }, false},
		{"空当前时钟", func(f *fixture) { f.now = time.Time{} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			tc.mutate(&f)
			p, err := f.build()
			if tc.valid {
				if err != nil || p == nil {
					t.Fatalf("合法资料被拒绝: %v", err)
				}
			} else if p != nil || !errors.Is(err, agents.ErrInvalid) {
				t.Fatalf("未统一拒绝: %v", err)
			}
		})
	}
}

func TestImmutableAndHidden(t *testing.T) {
	f := newFixture()
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	original := p.Snapshots()
	digest := p.ContentDigest()
	f.snapshots[0].Body = "修改输入"
	f.snapshots[0].Sentences[0].Text = "修改输入"
	*f.snapshots[0].LicenseExpires = f.now
	copy := p.Snapshots()
	copy[0].Body = "修改副本"
	copy[0].Sentences[0].Text = "修改副本"
	*copy[0].LicenseExpires = f.now
	scope := p.Scope()
	scope.SubjectName = "修改名称"
	if !reflect.DeepEqual(original, p.Snapshots()) || digest != p.ContentDigest() || p.Scope().SubjectName == scope.SubjectName {
		t.Fatal("资料包受外部修改影响")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "{}" {
		t.Fatal("序列化意外暴露字段")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if strings.Contains(fmt.Sprintf(format, p), original[0].Body) {
			t.Fatal("格式化泄露正文")
		}
	}
}

func TestCurrentSourceDigest(t *testing.T) {
	f := newFixture()
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	f.registry[0].MinInterval = time.Second
	q, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	if p.ContentDigest() == q.ContentDigest() {
		t.Fatal("当前来源登记变化未绑定完整摘要")
	}
	f.registry[0].MinInterval = 0
	f.registry[0].Domain = "EXAMPLE.COM."
	f.registry = append([]sources.Source{{ID: 2}}, f.registry...)
	q, err = f.build()
	if err != nil {
		t.Fatal(err)
	}
	if p.ContentDigest() != q.ContentDigest() {
		t.Fatal("域名规范化或无关来源影响摘要")
	}
}

func TestDigests(t *testing.T) {
	f := newFixture()
	second := cloneSnapshots(f.snapshots)[0]
	second.ID = 12
	f.snapshots = append(f.snapshots, second)
	f.approval.Use.Bindings[0].Evidence = append(f.approval.Use.Bindings[0].Evidence, config.AILocalAgentMaterialEvidence{SnapshotID: 12, Hash: second.Hash})
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := f.approval.MaterialsDigest(f.approval.Use.Bindings[0])
	if err != nil {
		t.Fatal(err)
	}
	if p.MaterialsDigest() != declaration || !agents.ValidDigest(p.ContentDigest()) {
		t.Fatal("摘要格式或声明身份不符")
	}
	f.snapshots[0], f.snapshots[1] = f.snapshots[1], f.snapshots[0]
	q, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	if p.ContentDigest() != q.ContentDigest() {
		t.Fatal("排列顺序改变身份")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*fixture)
	}{
		{"名称", func(f *fixture) { f.scope.SubjectName = "另一测试名称" }},
		{"会员", func(f *fixture) { f.scope.MemberID = 2; f.approval.Members = []int64{1, 2} }},
		{"URL", func(f *fixture) { f.snapshots[0].URL = "https://example.com/other" }},
		{"采集时点", func(f *fixture) { f.snapshots[0].FetchedAt = f.now.Add(-2 * time.Minute) }},
		{"许可", func(f *fixture) {
			f.registry[0].LicenseRef = "另一许可"
			f.snapshots[0].LicenseRef = "另一许可"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newFixture()
			before, err := g.build()
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&g)
			after, err := g.build()
			if err != nil {
				t.Fatal(err)
			}
			if before.ContentDigest() == after.ContentDigest() {
				t.Fatal("完整内容变化未改变摘要")
			}
		})
	}
	// 两条快照总量而不是单条大小必须受限。
	f.approval.Limits.MaxEvidenceBytes = len(second.Body)
	if _, err := f.build(); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("未限制总量")
	}
	f.approval.Limits.MaxEvidenceBytes = 1024
	f.snapshots[1] = f.snapshots[0]
	if _, err := f.build(); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("未拒绝同数量重复快照")
	}
}
