package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/materialpack"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type syntheticAgentMaterialStore struct {
	scope   repository.AgentMaterialScope
	request repository.AgentMaterialRequest
	calls   int
	hook    func()
	err     error
}

func (s *syntheticAgentMaterialStore) Load(ctx context.Context, req repository.AgentMaterialRequest) (repository.AgentMaterialScope, error) {
	s.calls++
	s.request = req
	if s.hook != nil {
		s.hook()
	}
	return s.scope, s.err
}

type syntheticAgentMaterialIdentity struct {
	calls    int
	expected string
	err      error
	hook     func(int)
}

func (i *syntheticAgentMaterialIdentity) Verify(ctx context.Context, id string) error {
	i.calls++
	if i.hook != nil {
		i.hook(i.calls)
	}
	if id != i.expected {
		return agents.ErrInvalid
	}
	return i.err
}

type syntheticAgentMaterialFixture struct {
	svc      *AIAgentMaterialService
	store    *syntheticAgentMaterialStore
	identity *syntheticAgentMaterialIdentity
	approval config.AILocalAgentMaterial
	path     string
	runtime  *config.AILocalRuntime
}

func (f *syntheticAgentMaterialFixture) write(t *testing.T) {
	t.Helper()
	raw, err := json.Marshal(f.approval)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

// 全部为合成资料；批准使用真实临时文件Source，但仓储和实例核验是窄测试替身。
// 不访问数据库、模型、积分或真实资料，不能作为数据库持久化或执行器接线证据。
func newSyntheticAgentMaterialFixture(t *testing.T) *syntheticAgentMaterialFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	id := strings.Repeat("a", 32)
	source := sources.Source{ID: 1, Domain: "agent.example.test", Category: sources.CategoryOfficialList, LicenseTag: "合成测试许可", LicenseRef: "合成测试归档", Enabled: true}
	snapshot, err := sources.BuildSnapshot(source, "https://agent.example.test/synthetic", sources.MediaPlain, []byte("合成测试目录包含资料说明。"), now.Add(-time.Minute), 4096)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = 11
	binding := config.AILocalAgentMaterialBinding{SubjectType: "theme", SubjectID: 2, ScopeApproval: "local-eval:synthetic-scope", Evidence: []config.AILocalAgentMaterialEvidence{{SnapshotID: 11, Hash: snapshot.Hash}}}
	topic := binding
	topic.SubjectType = "topic"
	f := &syntheticAgentMaterialFixture{
		path:     filepath.Join(t.TempDir(), "synthetic-agent.json"),
		runtime:  &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: "tm_ai_local_" + id, Listen: "127.0.0.1:18081", PollIntervalMS: 1000, BatchSize: 1},
		approval: config.AILocalAgentMaterial{Version: 1, InstanceID: id, Revision: "v1", Scope: config.AgentMaterialScope, IssuedAtMS: now.Add(-time.Hour).UnixMilli(), ExpiresAtMS: now.Add(time.Hour).UnixMilli(), Members: []int64{1}, Use: config.AILocalAgentMaterialUse{DatasetDigest: strings.Repeat("b", 64), ApprovalRef: "local-eval:synthetic-agent", Purpose: config.AgentMaterialPurpose, Bindings: []config.AILocalAgentMaterialBinding{binding, topic}}, Limits: config.AILocalAgentMaterialLimits{MaxSources: 2, MaxEvidenceBytes: 4096}},
		store:    &syntheticAgentMaterialStore{scope: repository.AgentMaterialScope{SubjectName: "合成测试题材", Now: now, Sources: []sources.Source{source}, Snapshots: []sources.Snapshot{snapshot}}},
		identity: &syntheticAgentMaterialIdentity{expected: id},
	}
	f.write(t)
	approved, err := config.NewAILocalAgentMaterialSource(f.path, f.runtime)
	if err != nil {
		t.Fatal(err)
	}
	f.svc, err = NewAIAgentMaterialService(approved, f.identity, f.store)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestAIAgentMaterialPrepareSynthetic(t *testing.T) {
	for _, mode := range []string{"有效", "非名单会员", "未批准对象", "专题关闭", "错误用途", "实例失效", "装配中撤回", "装配中改写", "装配中实例失效", "仓储失败", "预先取消", "仓储中取消", "末次实例核验取消", "空上下文", "仓储时钟过期", "正文篡改"} {
		t.Run(mode, func(t *testing.T) {
			f := newSyntheticAgentMaterialFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var input context.Context = ctx
			member, kind, id := int64(1), "theme", int64(2)
			switch mode {
			case "非名单会员":
				member = 3
			case "未批准对象":
				id = 3
			case "专题关闭":
				kind = "topic"
			case "错误用途":
				f.approval.Use.Purpose = "bounded-material-qa"
				f.write(t)
			case "实例失效":
				f.identity.err = errors.New("合成敏感内部正文")
			case "装配中撤回":
				f.store.hook = func() {
					if err := os.Remove(f.path); err != nil {
						t.Fatal(err)
					}
				}
			case "装配中改写":
				f.store.hook = func() { f.approval.Revision = "v2"; f.write(t) }
			case "装配中实例失效":
				f.store.hook = func() { f.identity.err = errors.New("合成敏感内部正文") }
			case "仓储失败":
				f.store.err = errors.New("合成敏感内部正文")
			case "预先取消":
				cancel()
			case "仓储中取消":
				f.store.hook = cancel
			case "末次实例核验取消":
				f.identity.hook = func(n int) {
					if n == 2 {
						cancel()
					}
				}
			case "空上下文":
				input = nil
			case "仓储时钟过期":
				f.store.scope.Now = time.UnixMilli(f.approval.ExpiresAtMS)
			case "正文篡改":
				f.store.scope.Snapshots[0].Body = "合成敏感内部正文"
			}
			p, err := f.svc.Prepare(input, member, kind, id)
			if mode == "有效" {
				if err != nil || p == nil {
					t.Fatalf("合法合成包被拒绝：%v", err)
				}
				if f.identity.calls != 2 || f.store.calls != 1 {
					t.Fatal("未执行双实例核验与一次装配")
				}
				scope := p.Scope()
				req := f.store.request
				if scope.IssuedAtMS != f.approval.IssuedAtMS || scope.ExpiresAtMS != f.approval.ExpiresAtMS || req.MemberID != 1 || req.SubjectType != "theme" || req.SubjectID != 2 || req.ValidUntil.UnixMilli() != f.approval.ExpiresAtMS || req.MaxSources != 2 || req.MaxEvidenceBytes != 4096 || len(req.Evidence) != 1 || req.Evidence[0].Hash != f.approval.Use.Bindings[0].Evidence[0].Hash {
					t.Fatal("可信批准身份或固定时窗未透传")
				}
			} else {
				if p != nil || err == nil || strings.Contains(err.Error(), "合成敏感内部正文") {
					t.Fatal("未失败关闭或泄露内部正文")
				}
				if strings.Contains(mode, "取消") {
					if !errors.Is(err, context.Canceled) {
						t.Fatal("取消错误丢失")
					}
				} else if !errors.Is(err, agents.ErrInvalid) {
					t.Fatalf("未返回安全错误：%v", err)
				}
			}
			if mode == "非名单会员" || mode == "未批准对象" || mode == "专题关闭" || mode == "错误用途" || mode == "实例失效" || mode == "预先取消" || mode == "空上下文" {
				if f.store.calls != 0 {
					t.Fatal("未通过授权却读取仓储")
				}
			}
		})
	}
}

func TestAIAgentMaterialRecheckSynthetic(t *testing.T) {
	for _, mode := range []string{"有效且时钟推进", "名称变化", "链接变化", "采集时点变化", "当前来源变化", "身份变化", "正文变化", "撤回", "实例失效", "仓储失败", "取消", "空原包", "零值原包"} {
		t.Run(mode, func(t *testing.T) {
			f := newSyntheticAgentMaterialFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original, err := f.svc.Prepare(ctx, 1, "theme", 2)
			if err != nil {
				t.Fatal(err)
			}
			digest := original.ContentDigest()
			switch mode {
			case "有效且时钟推进":
				f.store.scope.Now = f.store.scope.Now.Add(time.Minute)
			case "名称变化":
				f.store.scope.SubjectName = "另一合成测试题材"
			case "链接变化":
				f.store.scope.Snapshots[0].URL = "https://agent.example.test/changed"
			case "采集时点变化":
				f.store.scope.Snapshots[0].FetchedAt = f.store.scope.Snapshots[0].FetchedAt.Add(-time.Second)
			case "当前来源变化":
				f.store.scope.Sources[0].MinInterval = time.Second
			case "身份变化":
				f.approval.Use.DatasetDigest = strings.Repeat("c", 64)
				f.write(t)
				f.svc.source, err = config.NewAILocalAgentMaterialSource(f.path, f.runtime)
				if err != nil {
					t.Fatal(err)
				}
			case "正文变化":
				f.store.scope.Snapshots[0].Body = "另一合成测试正文"
			case "撤回":
				if err := os.Remove(f.path); err != nil {
					t.Fatal(err)
				}
			case "实例失效":
				f.identity.err = agents.ErrInvalid
			case "仓储失败":
				f.store.err = errors.New("合成敏感内部正文")
			case "取消":
				cancel()
			case "空原包":
				original = nil
			case "零值原包":
				original = &materialpack.Pack{}
			}
			err = f.svc.Recheck(ctx, original)
			if mode == "有效且时钟推进" {
				if err != nil || f.store.calls != 2 || f.identity.calls != 4 {
					t.Fatalf("稳定复核失败：%v", err)
				}
			} else if err == nil {
				t.Fatal("变化后静默接受原包")
			}
			if err != nil && strings.Contains(err.Error(), "合成敏感内部正文") {
				t.Fatal("泄露仓储正文")
			}
			if original != nil && mode != "零值原包" && original.ContentDigest() != digest {
				t.Fatal("复核改写了原包")
			}
		})
	}
}

// 两个获批会员的包分别复核自身范围，不能把包当作当前调用者授权。
func TestAIAgentMaterialMemberBindingSynthetic(t *testing.T) {
	f := newSyntheticAgentMaterialFixture(t)
	f.approval.Members = []int64{1, 2}
	f.write(t)
	var err error
	f.svc.source, err = config.NewAILocalAgentMaterialSource(f.path, f.runtime)
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.svc.Prepare(context.Background(), 1, "theme", 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.Prepare(context.Background(), 2, "theme", 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentDigest() == second.ContentDigest() {
		t.Fatal("不同会员完整摘要未隔离")
	}
	for _, p := range []*materialpack.Pack{first, second} {
		if err := f.svc.Recheck(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		if f.store.request.MemberID != p.Scope().MemberID {
			t.Fatal("复核未绑定原包会员")
		}
	}
}

func TestAIAgentMaterialMissingDependenciesSynthetic(t *testing.T) {
	f := newSyntheticAgentMaterialFixture(t)
	for _, v := range []struct {
		source   *config.AILocalAgentMaterialSource
		identity localInstanceCheck
		store    agentMaterialStore
	}{{nil, f.identity, f.store}, {f.svc.source, nil, f.store}, {f.svc.source, f.identity, nil}, {f.svc.source, (*syntheticAgentMaterialIdentity)(nil), f.store}, {f.svc.source, f.identity, (*syntheticAgentMaterialStore)(nil)}} {
		if _, err := NewAIAgentMaterialService(v.source, v.identity, v.store); !errors.Is(err, agents.ErrInvalid) {
			t.Fatal("缺依赖仍构造")
		}
		broken := &AIAgentMaterialService{source: v.source, identity: v.identity, store: v.store}
		if p, err := broken.Prepare(context.Background(), 1, "theme", 2); p != nil || !errors.Is(err, agents.ErrInvalid) {
			t.Fatal("运行时空依赖未拒绝")
		}
	}
	var absent *AIAgentMaterialService
	if p, err := absent.Prepare(context.Background(), 1, "theme", 2); p != nil || !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("空服务未拒绝")
	}
}
