package materialpack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// MaxManifestBytes 是持久化编码的严格字节上限，独立于批准中的正文上限。
const MaxManifestBytes = 256 * 1024

// 许可期限使用零或一个时间元素，避免 null 与字段缺失形成不同口径。
type manifestSource struct {
	ID             int64
	Domain         string
	Category       string
	LicenseTag     string
	LicenseRef     string
	LicenseExpires []time.Time
	MinInterval    time.Duration
	Enabled        bool
}
type manifestSnapshot struct {
	ID             int64
	SourceID       int64
	SourceCategory string
	URL            string
	Hash           string
	Body           string
	Sentences      []sources.Sentence
	LicenseTag     string
	LicenseRef     string
	LicenseExpires []time.Time
	FetchedAt      time.Time
}
type manifest struct {
	Version         int                         `json:"version"`
	ValidatedAt     time.Time                   `json:"validated_at"`
	Scope           Scope                       `json:"scope"`
	Approval        config.AILocalAgentMaterial `json:"approval"`
	Sources         []manifestSource            `json:"sources"`
	Snapshots       []manifestSnapshot          `json:"snapshots"`
	MaterialsDigest string                      `json:"materials_digest"`
	ContentDigest   string                      `json:"content_digest"`
}

func invalidManifest() error {
	return fmt.Errorf("资料包持久化校验失败: %w", agents.ErrInvalid)
}

// MarshalManifest 只供可信内部仓储显式持久化，结果含正文，严禁 HTTP 输出或日志记录。
// 不实现 json.Marshaler，Pack 默认 JSON 仍为 {}。超过编码上限的包不能持久化。
func MarshalManifest(p *Pack) ([]byte, error) {
	if p == nil {
		return nil, invalidManifest()
	}
	m := manifest{Version: 1, ValidatedAt: p.validatedAt, Scope: p.scope, Approval: p.approval, Sources: make([]manifestSource, 0, len(p.sources)), Snapshots: make([]manifestSnapshot, 0, len(p.snapshots)), MaterialsDigest: p.materialsDigest, ContentDigest: p.contentDigest}
	for _, s := range p.sources {
		m.Sources = append(m.Sources, manifestSource{s.ID, s.Domain, s.Category, s.LicenseTag, s.LicenseRef, timeList(s.LicenseExpires), s.MinInterval, s.Enabled})
	}
	for _, s := range p.snapshots {
		m.Snapshots = append(m.Snapshots, manifestSnapshot{s.ID, s.SourceID, s.SourceCategory, s.URL, s.Hash, s.Body, s.Sentences, s.LicenseTag, s.LicenseRef, timeList(s.LicenseExpires), s.FetchedAt})
	}
	budget := MaxManifestBytes
	if !boundedValue(reflect.ValueOf(m), &budget) {
		return nil, invalidManifest()
	}
	raw, err := json.Marshal(m)
	if err != nil || len(raw) > MaxManifestBytes {
		return nil, invalidManifest()
	}
	if _, err = RestoreManifest(raw, p.contentDigest); err != nil {
		return nil, invalidManifest()
	}
	return raw, nil
}

// RestoreManifest 只核验可信仓储保存的历史身份，不授予当前使用权。
// expectedContentDigest 必须来自可信持久化身份，不能取自不可信 manifest 本身。
// 恢复不读取当前批准/来源；执行前调用层必须另行 Current/Recheck。
// validated_at 仅为满足原期限和采集时点的历史重验见证，不是可信审计时间。
// 既有 ContentDigest 不含此字段，同有效区间内改动可能不改变身份；仓储若需证明
// 原始序列化字节未变，必须另存并核验 manifest 字节摘要或施加不可变存储约束。
func RestoreManifest(raw []byte, expectedContentDigest string) (*Pack, error) {
	if len(raw) < 2 || len(raw) > MaxManifestBytes || !agents.ValidDigest(expectedContentDigest) || task.CheckStageJSON(raw) != nil || !exactManifestJSON(raw, reflect.TypeOf(manifest{})) {
		return nil, invalidManifest()
	}
	var m manifest
	if json.Unmarshal(raw, &m) != nil || m.Version != 1 || m.ContentDigest != expectedContentDigest || len(m.Sources) < 1 || len(m.Sources) > 8 || len(m.Snapshots) < 1 || len(m.Snapshots) > 8 {
		return nil, invalidManifest()
	}
	registry := make([]sources.Source, 0, len(m.Sources))
	for _, s := range m.Sources {
		if len(s.LicenseExpires) > 1 {
			return nil, invalidManifest()
		}
		registry = append(registry, sources.Source{ID: s.ID, Domain: s.Domain, Category: s.Category, LicenseTag: s.LicenseTag, LicenseRef: s.LicenseRef, LicenseExpires: timePointer(s.LicenseExpires), MinInterval: s.MinInterval, Enabled: s.Enabled})
	}
	snapshots := make([]sources.Snapshot, 0, len(m.Snapshots))
	for _, s := range m.Snapshots {
		if len(s.LicenseExpires) > 1 {
			return nil, invalidManifest()
		}
		snapshots = append(snapshots, sources.Snapshot{ID: s.ID, SourceID: s.SourceID, SourceCategory: s.SourceCategory, URL: s.URL, Hash: s.Hash, Body: s.Body, Sentences: s.Sentences, LicenseTag: s.LicenseTag, LicenseRef: s.LicenseRef, LicenseExpires: timePointer(s.LicenseExpires), FetchedAt: s.FetchedAt})
	}
	p, err := New(m.Scope, m.Approval, registry, snapshots, m.ValidatedAt)
	if err != nil || p.contentDigest != m.ContentDigest || p.materialsDigest != m.MaterialsDigest {
		return nil, invalidManifest()
	}
	// 拒绝未使用来源、非规范来源及顺序漂移，而非静默丢弃后认领旧身份。
	if !reflect.DeepEqual(p.sources, registry) || !reflect.DeepEqual(p.snapshots, snapshots) {
		return nil, invalidManifest()
	}
	return p, nil
}
func cloneApproval(in config.AILocalAgentMaterial) config.AILocalAgentMaterial {
	out := in
	out.Members = append([]int64(nil), in.Members...)
	out.Use.Bindings = append([]config.AILocalAgentMaterialBinding(nil), in.Use.Bindings...)
	for i := range out.Use.Bindings {
		out.Use.Bindings[i].Evidence = append([]config.AILocalAgentMaterialEvidence(nil), in.Use.Bindings[i].Evidence...)
	}
	return out
}
func timeList(t *time.Time) []time.Time {
	out := make([]time.Time, 0, 1)
	if t != nil {
		out = append(out, t.UTC())
	}
	return out
}
func timePointer(ts []time.Time) *time.Time {
	if len(ts) == 0 {
		return nil
	}
	t := ts[0]
	return &t
}

// CheckStageJSON 已拒绝重复键、大小写重复、非法 UTF-8、超深及尾随值；此处补齐精确键和必填闭集。
func exactManifestJSON(raw []byte, t reflect.Type) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false
	}
	if t == reflect.TypeOf(time.Time{}) {
		var s string
		return json.Unmarshal(raw, &s) == nil
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || len(obj) != t.NumField() {
			return false
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" {
				name = f.Name
			}
			value, ok := obj[name]
			if !ok || !exactManifestJSON(value, f.Type) {
				return false
			}
		}
	case reflect.Slice:
		var list []json.RawMessage
		if json.Unmarshal(raw, &list) != nil {
			return false
		}
		for _, v := range list {
			if !exactManifestJSON(v, t.Elem()) {
				return false
			}
		}
	default:
		return json.Unmarshal(raw, reflect.New(t).Interface()) == nil
	}
	return true
}

// 在编码分配之前限制字符串和元素总量；编码后的转义膨胀仍由严格字节上限兜底。
func boundedValue(v reflect.Value, budget *int) bool {
	*budget--
	if *budget < 0 {
		return false
	}
	if v.Type() == reflect.TypeOf(time.Time{}) {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return false
		}
		*budget -= v.Len()
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !boundedValue(v.Field(i), budget) {
				return false
			}
		}
	case reflect.Slice:
		if v.Len() > *budget {
			return false
		}
		for i := 0; i < v.Len(); i++ {
			if !boundedValue(v.Index(i), budget) {
				return false
			}
		}
	}
	return *budget >= 0
}
