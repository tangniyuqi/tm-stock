// Package materialpack 只校验服务端已取得的题材资料，既不授予数据库读取权，也不调用模型。
// 调用方必须从可信服务端取得 Scope、当前来源登记及独立 Agent 批准；不得绑定客户端正文。
// 本包不复用问答批准、不制造问答证明，校验成功也不表示执行器已接线。
package materialpack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// Scope 是服务器核准的会员、对象名称与范围；不是客户端请求 DTO。
// 名称由可信对象记录提供，批准文件本身不声明名称，不能据此证明名称的数据库真实性。
type Scope struct {
	MemberID      int64
	SubjectType   string
	SubjectID     int64
	SubjectName   string
	ScopeApproval string
	IssuedAtMS    int64
	ExpiresAtMS   int64
}

// Pack 保存构造时的不可变副本；它不持有长期有效授权，发送前仍须重读并重新校验。
// 私有字段及脱敏格式化避免误序列化/日志输出正文；Snapshots 仅供服务端内部使用。
type Pack struct {
	scope           Scope
	approval        config.AILocalAgentMaterial
	sources         []sources.Source
	validatedAt     time.Time
	snapshots       []sources.Snapshot
	materialsDigest string
	contentDigest   string
}

func (Pack) String() string     { return "Agent资料包（正文已隐藏）" }
func (p Pack) GoString() string { return p.String() }

// Scope 返回值副本，不暴露资料正文。
func (p Pack) Scope() Scope { return p.scope }

// MaterialsDigest 是与原请求兼容的声明身份，不代表完整正文或当前许可有效。
func (p Pack) MaterialsDigest() string { return p.materialsDigest }

// ContentDigest 绑定可信范围、独立批准及完整快照内容；与输入排列顺序无关。
func (p Pack) ContentDigest() string { return p.contentDigest }

// Snapshots 返回深拷贝，仅供服务端内部使用；不得直接作为客户端响应或日志参数。
func (p Pack) Snapshots() []sources.Snapshot { return cloneSnapshots(p.snapshots) }

var refPattern = regexp.MustCompile(`^local-eval:[A-Za-z0-9_.:-]{1,110}$`)
var revisionPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)

// New 是无 I/O 的纯校验。approval 应由 AILocalAgentMaterialSource.Current 当前读取，
// 其文件可信性、实例绑定、撤回检查及数据库授权由调用层负责，本函数不能证明这些前置条件。
// now 必须为服务器时钟。所有失败统一包装 agents.ErrInvalid，不附带输入或下层错误正文。
func New(scope Scope, approval config.AILocalAgentMaterial, registry []sources.Source, snapshots []sources.Snapshot, now time.Time) (*Pack, error) {
	fail := func() (*Pack, error) { return nil, fmt.Errorf("校验Agent资料包失败: %w", agents.ErrInvalid) }
	if now.IsZero() || !validID(scope.MemberID) || !validID(scope.SubjectID) ||
		(scope.SubjectType != "theme" && scope.SubjectType != "topic") ||
		!utf8.ValidString(scope.SubjectName) || strings.TrimSpace(scope.SubjectName) == "" ||
		!refPattern.MatchString(scope.ScopeApproval) || !validPeriod(scope.IssuedAtMS, scope.ExpiresAtMS, now.UnixMilli()) ||
		!validApproval(approval, now.UnixMilli()) || scope.IssuedAtMS < approval.IssuedAtMS || scope.ExpiresAtMS > approval.ExpiresAtMS {
		return fail()
	}
	binding, ok := approval.Binding(scope.MemberID, scope.SubjectType, scope.SubjectID)
	if !ok || binding.ScopeApproval != scope.ScopeApproval || len(snapshots) != len(binding.Evidence) {
		return fail()
	}
	approved := make(map[int64]string, len(binding.Evidence))
	for _, e := range binding.Evidence {
		approved[e.SnapshotID] = e.Hash
	}
	registered := make(map[int64]sources.Source, len(registry))
	for _, source := range registry {
		if !validID(source.ID) {
			return fail()
		}
		if _, exists := registered[source.ID]; exists {
			return fail()
		}
		registered[source.ID] = source
	}
	out := cloneSnapshots(snapshots)
	seen := make(map[int64]bool, len(out))
	total := 0
	used := make(map[int64]sources.Source)
	for _, snapshot := range out {
		if seen[snapshot.ID] || approved[snapshot.ID] != snapshot.Hash || snapshot.Validate() != nil {
			return fail()
		}
		seen[snapshot.ID] = true
		source, exists := registered[snapshot.SourceID]
		if !exists || !source.HasValidAuthorization(now) || source.Category != snapshot.SourceCategory ||
			source.LicenseTag != snapshot.LicenseTag || source.LicenseRef != snapshot.LicenseRef ||
			!sameTime(source.LicenseExpires, snapshot.LicenseExpires) ||
			(source.LicenseExpires != nil && source.LicenseExpires.Before(time.UnixMilli(scope.ExpiresAtMS))) ||
			snapshot.FetchedAt.After(now) || snapshot.FetchedAt.UnixMilli() <= 0 {
			return fail()
		}
		canonical, err := sources.ValidateURL(source.Domain, snapshot.URL)
		if err != nil || canonical != snapshot.URL {
			return fail()
		}
		// 先减后加防溢出；按 UTF-8 正文字节总量而非字符数执行批准上限。
		if len(snapshot.Body) > approval.Limits.MaxEvidenceBytes-total {
			return fail()
		}
		total += len(snapshot.Body)
		// 仅把实际使用来源纳入完整身份，排除无关登记及输入顺序影响。
		source.Domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(source.Domain)), ".")
		if source.LicenseExpires != nil {
			v := source.LicenseExpires.UTC()
			source.LicenseExpires = &v
		}
		used[source.ID] = source
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	declaration, err := approval.MaterialsDigest(binding)
	if err != nil {
		return fail()
	}
	// 完整快照含 URL、正文、句子索引、许可和采集时点；范围名称与有效期也参与摘要。
	currentSources := make([]sources.Source, 0, len(used))
	for _, source := range used {
		currentSources = append(currentSources, source)
	}
	sort.Slice(currentSources, func(i, j int) bool { return currentSources[i].ID < currentSources[j].ID })
	raw, err := json.Marshal([]any{"tm-stock-agent-material-content-v1", scope, approval, currentSources, out})
	if err != nil {
		return fail()
	}
	sum := sha256.Sum256(raw)
	return &Pack{scope: scope, approval: cloneApproval(approval), sources: currentSources, validatedAt: now.UTC(), snapshots: out, materialsDigest: declaration, contentDigest: hex.EncodeToString(sum[:])}, nil
}

func validID(id int64) bool { return id > 0 && id <= 9007199254740991 }
func validPeriod(issued, expires, now int64) bool {
	return issued > 0 && issued <= now && expires > now && expires > issued
}
func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

// 独立检查公开结构，避免直接构造配置绕过用途、闭集或限制；不替代配置来源的实例鉴权。
func validApproval(c config.AILocalAgentMaterial, now int64) bool {
	if c.Version != 1 || c.Scope != config.AgentMaterialScope || strings.TrimSpace(c.InstanceID) == "" ||
		!revisionPattern.MatchString(c.Revision) || !validPeriod(c.IssuedAtMS, c.ExpiresAtMS, now) ||
		c.Use.Purpose != config.AgentMaterialPurpose || !agents.ValidDigest(c.Use.DatasetDigest) || !refPattern.MatchString(c.Use.ApprovalRef) ||
		c.Limits.MaxSources < 1 || c.Limits.MaxSources > 8 || c.Limits.MaxEvidenceBytes < 1 || c.Limits.MaxEvidenceBytes > 1<<20 ||
		len(c.Members) == 0 || len(c.Members) > 16 || len(c.Use.Bindings) == 0 || len(c.Use.Bindings) > 16 {
		return false
	}
	for i, m := range c.Members {
		if !validID(m) || i > 0 && m <= c.Members[i-1] {
			return false
		}
	}
	for i, b := range c.Use.Bindings {
		if (b.SubjectType != "theme" && b.SubjectType != "topic") || !validID(b.SubjectID) || !refPattern.MatchString(b.ScopeApproval) || len(b.Evidence) == 0 || len(b.Evidence) > c.Limits.MaxSources {
			return false
		}
		if i > 0 {
			p := c.Use.Bindings[i-1]
			if b.SubjectType < p.SubjectType || b.SubjectType == p.SubjectType && b.SubjectID <= p.SubjectID {
				return false
			}
		}
		for j, e := range b.Evidence {
			if !validID(e.SnapshotID) || !agents.ValidDigest(e.Hash) || j > 0 && e.SnapshotID <= b.Evidence[j-1].SnapshotID {
				return false
			}
		}
	}
	return true
}

func cloneSnapshots(in []sources.Snapshot) []sources.Snapshot {
	out := make([]sources.Snapshot, len(in))
	for i, s := range in {
		out[i] = s
		out[i].Sentences = append([]sources.Sentence(nil), s.Sentences...)
		out[i].FetchedAt = s.FetchedAt.UTC()
		if s.LicenseExpires != nil {
			v := s.LicenseExpires.UTC()
			out[i].LicenseExpires = &v
		}
	}
	return out
}
