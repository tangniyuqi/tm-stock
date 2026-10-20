package service

import (
	"context"
	"errors"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"strings"
	"testing"
	"time"
)

type bindingTestStore struct {
	*scriptedTaskStore
	record    task.AdmissionRecord
	marked    bool
	readError error
}

func (s *bindingTestStore) WithinAdmissionTx(ctx context.Context, fn func(task.AdmissionReadTx) error) error {
	return s.WithinTaskTx(ctx, func(task.Tx) error { return fn(s) })
}
func (s *bindingTestStore) ReadAdmission(ctx context.Context, v task.Task) (task.AdmissionRecord, bool, error) {
	if s.readError != nil {
		return task.AdmissionRecord{}, false, s.readError
	}
	if !s.marked {
		return task.AdmissionRecord{}, false, ctx.Err()
	}
	if err := s.record.Bind(v); err != nil {
		return task.AdmissionRecord{}, false, err
	}
	return s.record, true, nil
}
func bindingFixture(t *testing.T) (*bindingTestStore, AIL1ReportPlan, task.Task) {
	t.Helper()
	p, v, _ := l1PlanFixture(t)
	_, store, _, _, _ := taskFixture(t)
	at := p.Snapshots[0].FetchedAt.Add(time.Hour)
	v.Deadline = at.Add(time.Duration(v.Budget.MaxDurationMS) * time.Millisecond)
	v.Hash = strings.Repeat("b", 64)
	d, err := p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	a := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: v.UserID, AnalysisType: 1, SubjectType: task.Theme, SubjectID: v.SubjectID}, Price: points.Price{Version: v.PriceVersion, Points: v.PricePoints}, Budget: v.Budget, SubjectName: p.ThemeName, ModelName: p.Model.Model, FilingNo: p.Model.FilingNo, ProfileDigest: p.Model.ProfileDigest, ExecutionDigest: d, ApprovalRef: p.Model.ApprovalRef, QualityRef: p.Model.QualityRef, HardBudgetRef: p.Model.HardBudgetRef, PriceRef: "test-only-price", SourceRef: "test-only-source"}
	for _, s := range p.Snapshots {
		a.Materials = append(a.Materials, task.AdmissionMaterial{SnapshotID: s.ID, Digest: s.Hash, Domain: p.Policy.AllowedDomains[0], Category: s.SourceCategory, URL: s.URL, CollectedAt: s.FetchedAt})
	}
	_, hash, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	r := task.AdmissionRecord{TaskID: v.ID, UserID: v.UserID, TaskNo: v.No, RequestHash: v.Hash, Digest: hash, Plan: a, CreatedAt: at}
	if err := r.Bind(v); err != nil {
		t.Fatal(err)
	}
	store.v = v
	store.exists = true
	store.p.account.UserID = v.UserID
	return &bindingTestStore{scriptedTaskStore: store, record: r, marked: true}, p, v
}
func TestAIL1AdmissionBindingRejectsChangedExecution(t *testing.T) {
	for _, name := range []string{"same", "actor", "symbol", "agent_profile", "units", "model", "quality", "hard_budget", "policy", "theme", "source", "body", "license", "material_count", "missing", "corrupt", "commit", "legacy", "task_price", "task_budget", "scope"} {
		t.Run(name, func(t *testing.T) {
			s, p, v := bindingFixture(t)
			switch name {
			case "actor":
				p.Agent.ActorID++
			case "symbol":
				p.Agent.Symbol = "600002"
			case "agent_profile":
				p.Agent.ProfileDigest = strings.Repeat("f", 64)
			case "units":
				p.Agent.Units.Tokens++
			case "model":
				p.Model.Model = "其他测试模型"
			case "quality":
				p.Model.QualityRef = "changed"
			case "hard_budget":
				p.Model.HardBudgetRef = "changed"
			case "policy":
				p.Policy.Ref = "changed"
			case "theme":
				p.ThemeName = "其他示例题材"
			case "source":
				p.Snapshots[0].SourceID++
			case "body":
				p.Snapshots[0].Body += "不同测试正文"
			case "license":
				p.Snapshots[0].LicenseRef = "other-test-license"
			case "material_count":
				p.Snapshots = nil
			case "missing":
				s.readError = points.ErrInvariant
			case "corrupt":
				s.record.Digest = "wrong"
			case "commit":
				s.commitError = errPointsInjected
			case "legacy":
				s.marked = false
			case "task_price":
				s.v.PricePoints++
			case "task_budget":
				s.v.Budget.MaxTokens++
			case "scope":
				s.v.SubjectID++
			}
			err := checkAdmittedL1(context.Background(), s, v, p)
			if name == "same" || name == "legacy" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("换计划或未确认提交被放行")
			}
		})
	}
	s, p, v := bindingFixture(t)
	if guardTaskAgentAdmission(context.Background(), nil, nil, v) == nil {
		t.Fatal("空单Agent准入仓储")
	}
	if err := guardTaskAgentAdmission(context.Background(), s, nil, v); !errors.Is(err, task.ErrAdmissionUnavailable) {
		t.Fatal("单Agent绕过持久完整计划", err)
	}
	if err := guardTaskAgentAdmission(context.Background(), s, l1AgentPlanner{}, v); err != nil {
		t.Fatal(err)
	}
	if checkAdmittedL1(context.Background(), nil, v, p) == nil {
		t.Fatal("nil仓储")
	}
	c, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(checkAdmittedL1(c, s, v, p), context.Canceled) {
		t.Fatal("取消未生效")
	}
	if checkOriginalAdmission(context.Background(), s.scriptedTaskStore, v) == nil {
		t.Fatal("缺少读取能力")
	}
}
func TestAIL1AdmissionDigestIgnoresVolatileIdentityAndCopiesInputs(t *testing.T) {
	_, p, _ := bindingFixture(t)
	at := p.Snapshots[0].FetchedAt.Add(time.Hour)
	p.Snapshots[0].LicenseExpires = &at
	q := cloneL1Plan(p)
	if q.Snapshots[0].LicenseExpires == p.Snapshots[0].LicenseExpires {
		t.Fatal("共享可变时点")
	}
	a, err := p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	q.Agent.UserID++
	q.Agent.TaskID++
	q.Agent.TaskNo = "new-task-no"
	q.Agent.OperationKey = "new-derived-operation-key"
	q.Snapshots[0].FetchedAt = q.Snapshots[0].FetchedAt.In(time.FixedZone("测试东八区", 8*3600))
	b, err := q.AdmissionDigest()
	if err != nil || a != b {
		t.Fatal("可变任务身份或等价时区改变批准语义")
	}
	q.Policy.AllowedDomains[0] = "changed.example.test"
	if q.Policy.AllowedDomains[0] == p.Policy.AllowedDomains[0] {
		t.Fatal("共享域名数组")
	}
	q.Snapshots[0].Sentences[0].Text = "其他测试句子"
	if q.Snapshots[0].Sentences[0].Text == p.Snapshots[0].Sentences[0].Text {
		t.Fatal("共享句子数组")
	}
	q = cloneL1Plan(p)
	q.Snapshots[0].FetchedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := q.AdmissionDigest(); err == nil {
		t.Fatal("无效JSON时点未拒绝")
	}
}
func TestAITaskMemberAdmissionRecordFailureNeverReturnsReceipt(t *testing.T) {
	for _, name := range []string{"missing", "digest", "plan", "future"} {
		t.Run(name, func(t *testing.T) {
			s, st, c, in, key := admissionFixture(t)
			if name == "future" {
				c.p.Materials[0].CollectedAt = st.points.now.Add(time.Second)
				if out, err := s.Create(context.Background(), 1, key, in); err == nil || out.TaskID != 0 {
					t.Fatal("未来证据计划未拒绝")
				}
				return
			}
			if _, err := s.Create(context.Background(), 1, key, in); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "missing":
				st.admission = nil
			case "digest":
				st.marker = strings.Repeat("f", 64)
			case "plan":
				st.admission.Plan.QualityRef = "changed"
			}
			if out, err := s.Receipt(context.Background(), 1, key); err == nil || out.TaskID != 0 {
				t.Fatal("损坏准入仍返回收据")
			}
			if out, err := s.Create(context.Background(), 1, key, in); err == nil || out.TaskID != 0 {
				t.Fatal("原键补造准入或返回候选收据")
			}
		})
	}
}
