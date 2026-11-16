package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/materialpack"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func bindingFixture(t *testing.T) AgentMaterialBinding {
	t.Helper()
	now := time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	src := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试", LicenseRef: "test-only", LicenseExpires: &expires, Enabled: true}
	snap, err := sources.BuildSnapshot(src, "https://example.test/material", sources.MediaPlain, []byte("仅用于绑定测试的公开资料。"), now, 65536)
	if err != nil {
		t.Fatal(err)
	}
	snap.ID = 3
	approval := config.AILocalAgentMaterial{Version: 1, InstanceID: "test-instance", Revision: "test-v1", Scope: config.AgentMaterialScope, IssuedAtMS: now.Add(-time.Hour).UnixMilli(), ExpiresAtMS: expires.UnixMilli(), Members: []int64{1}, Use: config.AILocalAgentMaterialUse{DatasetDigest: strings.Repeat("a", 64), ApprovalRef: "local-eval:test", Purpose: config.AgentMaterialPurpose, Bindings: []config.AILocalAgentMaterialBinding{{SubjectType: "theme", SubjectID: 2, ScopeApproval: "local-eval:scope", Evidence: []config.AILocalAgentMaterialEvidence{{SnapshotID: 3, Hash: snap.Hash}}}}}, Limits: config.AILocalAgentMaterialLimits{MaxSources: 1, MaxEvidenceBytes: 65536}}
	pack, err := materialpack.New(materialpack.Scope{MemberID: 1, SubjectType: "theme", SubjectID: 2, SubjectName: "测试题材", ScopeApproval: "local-eval:scope", IssuedAtMS: approval.IssuedAtMS, ExpiresAtMS: approval.ExpiresAtMS}, approval, []sources.Source{src}, []sources.Snapshot{snap}, now)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := materialpack.MarshalManifest(pack)
	if err != nil {
		t.Fatal(err)
	}
	identity := AgentMaterialTaskIdentity{TaskID: 7, UserID: 1, TaskNo: strings.Repeat("b", 32)}
	req := agents.Request{RequestID: identity.RequestID(), MemberID: 1, SubjectType: "theme", SubjectID: 2, MaterialsDigest: pack.MaterialsDigest()}
	return AgentMaterialBinding{taskID: 7, userID: 1, taskNo: identity.TaskNo, record: agents.Record{ID: 5, ActorID: 9, Request: req, ProfileDigest: strings.Repeat("c", 64), State: agents.Submitting}, materialsDigest: pack.MaterialsDigest(), contentDigest: pack.ContentDigest(), manifest: raw}
}

func TestAgentMaterialBindingRestoreIdentity(t *testing.T) {
	b := bindingFixture(t)
	p, err := b.Pack()
	if err != nil || p.ContentDigest() != b.ContentDigest() {
		t.Fatal("有效绑定未恢复", err)
	}
	raw, err := json.Marshal(b)
	if err != nil || string(raw) != "{}" || strings.Contains(fmt.Sprintf("%+v %#v", b, b), "公开资料") {
		t.Fatal("绑定格式化泄露正文")
	}
	cp := p.Snapshots()
	cp[0].Sentences[0].Text = "修改副本"
	again, err := b.Pack()
	if err != nil || again.Snapshots()[0].Sentences[0].Text == "修改副本" {
		t.Fatal("恢复共享内存")
	}
	cases := map[string]func(*AgentMaterialBinding){
		"可信摘要错误": func(v *AgentMaterialBinding) { v.contentDigest = strings.Repeat("0", 64) },
		"声明摘要错误": func(v *AgentMaterialBinding) { v.materialsDigest = strings.Repeat("0", 64) },
		"任务错绑":   func(v *AgentMaterialBinding) { v.taskID++ },
		"任务号错绑":  func(v *AgentMaterialBinding) { v.taskNo = strings.Repeat("d", 32) },
		"会员错绑":   func(v *AgentMaterialBinding) { v.userID++ },
		"对象错绑":   func(v *AgentMaterialBinding) { v.record.Request.SubjectID++ },
		"请求错绑":   func(v *AgentMaterialBinding) { v.record.Request.RequestID = strings.Repeat("d", 64) },
		"正文篡改": func(v *AgentMaterialBinding) {
			v.manifest = []byte(strings.Replace(string(v.manifest), "公开资料", "篡改资料", 1))
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := b
			mutate(&v)
			if p, err := v.Pack(); p != nil || !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("篡改未拒绝", err)
			}
		})
	}
}

func TestAgentMaterialBindingRequestMatchesOriginalPlan(t *testing.T) {
	for _, id := range []int64{1, 9007199254740993, 9223372036854775807} {
		v := AgentMaterialTaskIdentity{TaskID: id, UserID: 1, TaskNo: strings.Repeat("a", 32), Owner: "test-owner-123456", Fence: 1}
		p := agenttask.Plan{TaskID: id, UserID: v.UserID, TaskNo: v.TaskNo}
		if !v.valid() || v.RequestID() != p.RequestID() {
			t.Fatal("原请求编码变化")
		}
		other := v
		other.Owner = "another-owner-123456"
		other.Fence++
		if other.RequestID() != v.RequestID() {
			t.Fatal("租约参与不可变请求身份")
		}
	}
}

func TestAgentMaterialBindingRejectsMissingDependencies(t *testing.T) {
	var tx *agentTx
	if _, _, err := tx.ReserveMaterialBinding(context.Background(), AgentMaterialTaskIdentity{}, agents.Profile{}, agents.Request{}, 0, nil); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("空事务未拒绝")
	}
	var repo *AIAgentRepo
	if _, err := repo.MaterialBindingByRequest(context.Background(), 1, "test-request"); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("空仓储未拒绝")
	}
	for _, v := range []AgentMaterialTaskIdentity{{}, {TaskID: 1, UserID: 1, TaskNo: strings.Repeat("a", 32), Owner: "test-owner-123456", Fence: 0}, {TaskID: 1, UserID: 9007199254740992, TaskNo: strings.Repeat("a", 32), Owner: "test-owner-123456", Fence: 1}} {
		if v.valid() {
			t.Fatal("非法身份通过")
		}
	}
}
