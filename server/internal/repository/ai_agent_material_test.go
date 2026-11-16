package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func agentMaterialRequestFixture() AgentMaterialRequest {
	return AgentMaterialRequest{MemberID: 1, SubjectType: "theme", SubjectID: 2, ValidUntil: time.Now().UTC().Truncate(time.Millisecond).Add(time.Hour), Evidence: []AgentMaterialEvidence{{SnapshotID: 3, Hash: strings.Repeat("a", 64)}}, MaxSources: 8, MaxEvidenceBytes: 65536}
}
func TestAgentMaterialRejectsParametersBeforeDatabase(t *testing.T) {
	cases := map[string]func(*AgentMaterialRequest){
		"会员零值":  func(v *AgentMaterialRequest) { v.MemberID = 0 },
		"会员溢出":  func(v *AgentMaterialRequest) { v.MemberID = conversation.MaxID + 1 },
		"专题未接线": func(v *AgentMaterialRequest) { v.SubjectType = "topic" },
		"类型空白":  func(v *AgentMaterialRequest) { v.SubjectType = "theme " },
		"对象零值":  func(v *AgentMaterialRequest) { v.SubjectID = 0 },
		"期限零值":  func(v *AgentMaterialRequest) { v.ValidUntil = time.Time{} },
		"期限精度":  func(v *AgentMaterialRequest) { v.ValidUntil = v.ValidUntil.Add(time.Nanosecond) },
		"空快照":   func(v *AgentMaterialRequest) { v.Evidence = nil },
		"编号溢出":  func(v *AgentMaterialRequest) { v.Evidence[0].SnapshotID = conversation.MaxID + 1 },
		"哈希大写":  func(v *AgentMaterialRequest) { v.Evidence[0].Hash = strings.Repeat("A", 64) },
		"哈希长度":  func(v *AgentMaterialRequest) { v.Evidence[0].Hash = "a" },
		"重复编号":  func(v *AgentMaterialRequest) { v.Evidence = append(v.Evidence, v.Evidence[0]) },
		"倒序编号": func(v *AgentMaterialRequest) {
			v.Evidence = append(v.Evidence, AgentMaterialEvidence{2, strings.Repeat("b", 64)})
		},
		"来源上限零": func(v *AgentMaterialRequest) { v.MaxSources = 0 },
		"来源上限大": func(v *AgentMaterialRequest) { v.MaxSources = 9 },
		"字节上限零": func(v *AgentMaterialRequest) { v.MaxEvidenceBytes = 0 },
		"字节上限大": func(v *AgentMaterialRequest) { v.MaxEvidenceBytes = (1 << 20) + 1 },
		"快照超限": func(v *AgentMaterialRequest) {
			v.MaxSources = 1
			v.Evidence = append(v.Evidence, AgentMaterialEvidence{4, strings.Repeat("b", 64)})
		},
	}
	// 零值数据库不能启动事务；若参数检查晚于数据库访问，本测试会直接失败。
	repo := NewAIAgentMaterialRepo(&sql.DB{})
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := agentMaterialRequestFixture()
			mutate(&v)
			if validAgentMaterialRequest(v) {
				t.Fatal("非法参数被接受")
			}
			if _, err := repo.Load(context.Background(), v); !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("参数未在访问数据库前拒绝", err)
			}
		})
	}
	if !validAgentMaterialRequest(agentMaterialRequestFixture()) {
		t.Fatal("合法参数被拒绝")
	}
	if _, err := repo.Load(nil, agentMaterialRequestFixture()); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("空上下文未拒绝")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Load(ctx, agentMaterialRequestFixture()); !errors.Is(err, context.Canceled) {
		t.Fatal("取消未传播")
	}
	var absent *AIAgentMaterialRepo
	for _, r := range []*AIAgentMaterialRepo{absent, NewAIAgentMaterialRepo(nil)} {
		if _, err := r.Load(context.Background(), agentMaterialRequestFixture()); !errors.Is(err, agents.ErrInvalid) {
			t.Fatal("空依赖未拒绝")
		}
	}
}
func TestAgentMaterialSnapshotCurrent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	until := now.Add(time.Hour)
	expires := now.Add(2 * time.Hour)
	source := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试", LicenseRef: "test-only", LicenseExpires: &expires, Enabled: true}
	snapshot, err := sources.BuildSnapshot(source, "https://example.test/material", sources.MediaPlain, []byte("仅测试资料。公开信息。"), now, 65536)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = 3
	ref := AgentMaterialEvidence{3, snapshot.Hash}
	if !agentMaterialSnapshotCurrent(snapshot, source, ref, now, until) {
		t.Fatal("有效快照未通过")
	}
	cases := map[string]func(*sources.Snapshot, *sources.Source){
		"来源停用": func(_ *sources.Snapshot, s *sources.Source) { s.Enabled = false },
		"来源编号": func(_ *sources.Snapshot, s *sources.Source) { s.ID++ },
		"引用变化": func(_ *sources.Snapshot, s *sources.Source) { s.LicenseRef = "changed" },
		"标签变化": func(_ *sources.Snapshot, s *sources.Source) { s.LicenseTag = "changed" },
		"类别变化": func(_ *sources.Snapshot, s *sources.Source) { s.Category = sources.CategoryAnnualReport },
		"域名变化": func(_ *sources.Snapshot, s *sources.Source) { s.Domain = "other.test" },
		"正文变化": func(s *sources.Snapshot, _ *sources.Source) { s.Body += "其他" },
		"编号变化": func(s *sources.Snapshot, _ *sources.Source) { s.ID++ },
		"未来采集": func(s *sources.Snapshot, _ *sources.Source) { s.FetchedAt = now.Add(time.Minute) },
		"偏移变化": func(s *sources.Snapshot, _ *sources.Source) {
			s.Sentences = append([]sources.Sentence(nil), s.Sentences...)
			s.Sentences[0].EndByte--
		},
		"期限不覆盖": func(s *sources.Snapshot, src *sources.Source) {
			v := now.Add(time.Minute)
			s.LicenseExpires = &v
			src.LicenseExpires = &v
		},
		"许可到期": func(s *sources.Snapshot, src *sources.Source) {
			v := now
			s.LicenseExpires = &v
			src.LicenseExpires = &v
		},
		"许可副本不同": func(_ *sources.Snapshot, src *sources.Source) { src.LicenseExpires = nil },
		"链接凭据":   func(s *sources.Snapshot, _ *sources.Source) { s.URL += "?token=secret" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s, src := snapshot, source
			mutate(&s, &src)
			if agentMaterialSnapshotCurrent(s, src, ref, now, until) {
				t.Fatal("变更快照或撤权来源未拒绝")
			}
		})
	}
}
