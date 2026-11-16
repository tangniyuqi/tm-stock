package materialpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestManifestRoundTripHistorical(t *testing.T) {
	for _, noExpiry := range []bool{false, true} {
		f := newFixture()
		// 固定过去的测试证据，当前时间不参与恢复。
		f.now = f.now.AddDate(-2, 0, 0)
		f.scope.IssuedAtMS = f.now.Add(-time.Hour).UnixMilli()
		f.scope.ExpiresAtMS = f.now.Add(time.Hour).UnixMilli()
		f.approval.IssuedAtMS = f.scope.IssuedAtMS
		f.approval.ExpiresAtMS = f.scope.ExpiresAtMS
		expires := f.now.Add(time.Hour)
		f.registry[0].LicenseExpires = &expires
		f.snapshots[0].LicenseExpires = &expires
		f.snapshots[0].FetchedAt = f.now.Add(-time.Minute)
		if noExpiry {
			f.registry[0].LicenseExpires = nil
			f.snapshots[0].LicenseExpires = nil
		}
		p, err := f.build()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := MarshalManifest(p)
		if err != nil {
			t.Fatal(err)
		}
		q, err := RestoreManifest(raw, p.ContentDigest())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p, q) {
			t.Fatal("完整历史资料恢复不一致")
		}
		again, err := MarshalManifest(q)
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatal("持久化往返不稳定")
		}
		hidden, err := json.Marshal(q)
		if err != nil || string(hidden) != "{}" {
			t.Fatal("默认JSON泄露正文")
		}
		if _, err := New(q.scope, q.approval, q.sources, q.snapshots, f.now.Add(2*time.Hour)); !errors.Is(err, agents.ErrInvalid) {
			t.Fatal("历史恢复错误地赋予当前授权")
		}
	}
}

func TestManifestClosedJSON(t *testing.T) {
	f := newFixture()
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	cases := map[string]string{
		"未知字段":   strings.Replace(s, `"version":1`, `"extra":1,"version":1`, 1),
		"重复字段":   strings.Replace(s, `"version":1`, `"version":1,"version":1`, 1),
		"大小写重复":  strings.Replace(s, `"version":1`, `"Version":1,"version":1`, 1),
		"大小写别名":  strings.Replace(s, `"version":1`, `"Version":1`, 1),
		"缺失":     strings.Replace(s, `"version":1,`, "", 1),
		"空值":     strings.Replace(s, `"version":1`, `"version":null`, 1),
		"错误版本":   strings.Replace(s, `"version":1`, `"version":2`, 1),
		"类型错误":   strings.Replace(s, `"version":1`, `"version":"1"`, 1),
		"嵌套别名":   strings.Replace(s, `"MemberID":1`, `"memberid":1`, 1),
		"嵌套重复":   strings.Replace(s, `"MemberID":1`, `"MemberID":1,"MemberID":1`, 1),
		"嵌套缺失":   strings.Replace(s, `"Enabled":true`, `"unknown":true`, 1),
		"数组空值":   strings.Replace(s, `"members":[1]`, `"members":[null]`, 1),
		"正文空值":   strings.Replace(s, `"Body":"仅供测试的公开资料。"`, `"Body":null`, 1),
		"跨会员":    strings.Replace(s, `"MemberID":1`, `"MemberID":2`, 1),
		"正文篡改":   strings.Replace(s, "仅供测试的公开资料。", "篡改测试正文。", 1),
		"来源篡改":   strings.Replace(s, `"MinInterval":0`, `"MinInterval":1`, 1),
		"时点过期":   strings.Replace(s, `"validated_at":"2026-11-16T08:00:00Z"`, `"validated_at":"2026-11-16T09:00:00Z"`, 1),
		"时点早于采集": strings.Replace(s, `"validated_at":"2026-11-16T08:00:00Z"`, `"validated_at":"2026-11-16T07:00:00Z"`, 1),
		"尾随":     s + "{}",
		"非法UTF8": s + string([]byte{255}),
		"超限":     s + strings.Repeat(" ", MaxManifestBytes),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			if bad == s {
				t.Fatal("测试未替换字段")
			}
			q, err := RestoreManifest([]byte(bad), p.ContentDigest())
			if q != nil || !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("错误manifest未拒绝")
			}
		})
	}
	for _, digest := range []string{"", strings.Repeat("0", 64), strings.ToUpper(p.ContentDigest())} {
		if q, err := RestoreManifest(raw, digest); q != nil || !errors.Is(err, agents.ErrInvalid) {
			t.Fatal("错误可信摘要未拒绝")
		}
	}
	// 仅核验带尾部空格的输入长度边界，不代表编码器输出边界。
	padded := append(append([]byte(nil), raw...), bytes.Repeat([]byte(" "), MaxManifestBytes-len(raw))...)
	if _, err := RestoreManifest(padded, p.ContentDigest()); err != nil {
		t.Fatal("恰好大小上限被拒绝")
	}
}

func TestManifestDeepCopiesAndDigestCompatibility(t *testing.T) {
	f := newFixture()
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	// 按改动前的编码表达式独立取证，保证两种摘要的字节语义未改变。
	old, err := json.Marshal([]any{"tm-stock-agent-material-content-v1", f.scope, f.approval, f.registry, cloneSnapshots(f.snapshots)})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(old)
	if p.ContentDigest() != hex.EncodeToString(sum[:]) {
		t.Fatal("旧完整摘要语义漂移")
	}
	before, err := MarshalManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	f.approval.Members[0] = 2
	f.approval.Use.Bindings[0].ScopeApproval = "local-eval:changed"
	f.approval.Use.Bindings[0].Evidence[0].Hash = strings.Repeat("b", 64)
	f.registry[0].Domain = "changed.example"
	*f.registry[0].LicenseExpires = f.now
	f.snapshots[0].Sentences[0].Text = "篡改"
	after, err := MarshalManifest(p)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("输入slice或pointer修改影响持久化")
	}
	q, err := RestoreManifest(before, p.ContentDigest())
	if err != nil {
		t.Fatal(err)
	}
	cp := q.Snapshots()
	cp[0].Sentences[0].Text = "修改返回副本"
	*cp[0].LicenseExpires = f.now
	after, err = MarshalManifest(q)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("恢复后的快照共享内存")
	}
	g := newFixture()
	g.scope.MemberID = 2
	g.approval.Members = []int64{1, 2}
	other, err := g.build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreManifest(before, other.ContentDigest()); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("另一会员身份认领旧包")
	}
}

func TestManifestHistoricalTimeWitnessBoundary(t *testing.T) {
	f := newFixture()
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	// 同有效区间内的见证不是原始创建时间证明；原字节完整性须由仓储另行绑定。
	changed := bytes.Replace(raw, []byte("2026-11-16T08:00:00Z"), []byte("2026-11-16T08:01:00Z"), 1)
	q, err := RestoreManifest(changed, p.ContentDigest())
	if err != nil || q.ContentDigest() != p.ContentDigest() {
		t.Fatal("历史重验见证边界漂移")
	}
	if bytes.Equal(raw, changed) {
		t.Fatal("未实际修改见证")
	}
}

func TestManifestCanonicalCollections(t *testing.T) {
	f := newFixture()
	secondSource := f.registry[0]
	secondSource.ID = 2
	second := cloneSnapshots(f.snapshots)[0]
	second.ID = 12
	second.SourceID = 2
	f.registry = append(f.registry, secondSource)
	f.snapshots = append(f.snapshots, second)
	f.approval.Use.Bindings[0].Evidence = append(f.approval.Use.Bindings[0].Evidence, config.AILocalAgentMaterialEvidence{SnapshotID: 12, Hash: second.Hash})
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	f.registry[0], f.registry[1] = f.registry[1], f.registry[0]
	f.snapshots[0], f.snapshots[1] = f.snapshots[1], f.snapshots[0]
	q, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := MarshalManifest(q)
	if err != nil || !bytes.Equal(raw, reordered) {
		t.Fatal("构造输入顺序改变规范manifest")
	}
	cases := map[string]func(*manifest){
		"来源乱序":   func(m *manifest) { m.Sources[0], m.Sources[1] = m.Sources[1], m.Sources[0] },
		"快照乱序":   func(m *manifest) { m.Snapshots[0], m.Snapshots[1] = m.Snapshots[1], m.Snapshots[0] },
		"多余未用来源": func(m *manifest) { s := m.Sources[0]; s.ID = 3; m.Sources = append(m.Sources, s) },
		"来源双期限": func(m *manifest) {
			m.Sources[0].LicenseExpires = append(m.Sources[0].LicenseExpires, m.Sources[0].LicenseExpires[0])
		},
		"快照双期限": func(m *manifest) {
			m.Snapshots[0].LicenseExpires = append(m.Snapshots[0].LicenseExpires, m.Snapshots[0].LicenseExpires[0])
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var m manifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			mutate(&m)
			bad, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if q, err := RestoreManifest(bad, p.ContentDigest()); q != nil || !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("非规范集合未拒绝")
			}
		})
	}
}

func TestManifestEscapedDuplicate(t *testing.T) {
	f := newFixture()
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	bad := bytes.Replace(raw, []byte(`"MemberID":1`), []byte(`"MemberID":1,"\u004demberID":1`), 1)
	if bytes.Equal(raw, bad) {
		t.Fatal("未修改嵌套字段")
	}
	if q, err := RestoreManifest(bad, p.ContentDigest()); q != nil || !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("转义重复字段未拒绝")
	}
}

func TestManifestEscapeExpansionRejected(t *testing.T) {
	f := newFixture()
	f.scope.SubjectName = strings.Repeat("<", 50000)
	p, err := f.build()
	if err != nil {
		t.Fatal(err)
	}
	// 原字符串预算明显低于上限，JSON HTML转义膨胀后才超过上限。
	budget := MaxManifestBytes
	if !boundedValue(reflect.ValueOf(p.scope), &budget) {
		t.Fatal("未进入转义膨胀边界")
	}
	escaped, err := json.Marshal(p.scope)
	if err != nil || len(escaped) <= MaxManifestBytes {
		t.Fatal("测试未制造转义膨胀")
	}
	if b, err := MarshalManifest(p); b != nil || !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("转义膨胀超限未拒绝")
	}
}

func TestManifestLegalLargePackRejected(t *testing.T) {
	f := newFixture()
	body := strings.Repeat("测", 100000)
	sum := sha256.Sum256([]byte(body))
	hash := hex.EncodeToString(sum[:])
	f.snapshots[0].Body = body
	f.snapshots[0].Hash = hash
	f.snapshots[0].Sentences = []sources.Sentence{{Number: 1, StartByte: 0, EndByte: len(body), Text: body}}
	f.approval.Limits.MaxEvidenceBytes = 1 << 20
	f.approval.Use.Bindings[0].Evidence[0].Hash = hash
	p, err := f.build()
	if err != nil {
		t.Fatal("合法大包构造失败", err)
	}
	if b, err := MarshalManifest(p); b != nil || !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("未执行独立持久化大小上限")
	}
	for _, p := range []*Pack{nil, {}} {
		if b, err := MarshalManifest(p); b != nil || !errors.Is(err, agents.ErrInvalid) {
			t.Fatal("空资料包未拒绝")
		}
	}
}
