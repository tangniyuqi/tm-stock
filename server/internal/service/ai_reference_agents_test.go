package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

type referenceExecutorStub struct {
	agentExecutorStub
	checks, submitted   int
	checkErr, submitErr error
	seen                agents.Request
}

func (e *referenceExecutorStub) ValidateReference(_ context.Context, r agents.Request) error {
	e.checks++
	if !r.Valid() || !r.IsReference() {
		return agents.ErrInvalid
	}
	return e.checkErr
}
func (e *referenceExecutorStub) SubmitReference(_ context.Context, r agents.Request) (agents.Acceptance, error) {
	e.submitted++
	e.seen = r
	return agents.Acceptance{RemoteID: "reference_remote_01"}, e.submitErr
}

func newReferenceService(t *testing.T, store *agentMemoryStore, executor agents.Executor) *AIAgentService {
	t.Helper()
	s, err := NewAIAgentService(store, executor, agents.Profile{Digest: strings.Repeat("b", 64), MaxSubmissions: 2})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func referenceServiceRequest() agents.Request {
	return agents.Request{RequestID: "reference_request_01", MemberID: 1, SubjectType: "theme", SubjectID: 1001, MaterialsDigest: strings.Repeat("a", 64)}
}

func TestReferenceAgentRejectsBeforeReservation(t *testing.T) {
	t.Run("旧执行器不接受题材", func(t *testing.T) {
		store, executor := agentTestStore(), &agentExecutorStub{}
		_, err := newReferenceService(t, store, executor).Submit(context.Background(), 7, referenceServiceRequest())
		if !errors.Is(err, agents.ErrDisabled) || len(store.records) != 0 || executor.submits.Load() != 0 {
			t.Fatal("旧股票接口被用于资料对象")
		}
	})
	t.Run("准入撤回不预留", func(t *testing.T) {
		store, executor := agentTestStore(), &referenceExecutorStub{checkErr: agents.ErrDisabled}
		_, err := newReferenceService(t, store, executor).Submit(context.Background(), 7, referenceServiceRequest())
		if !errors.Is(err, agents.ErrDisabled) || len(store.records) != 0 || len(store.counts) != 0 || executor.submitted != 0 {
			t.Fatal("准入拒绝仍产生执行副作用")
		}
	})
}

func TestReferenceAgentReplayAndScopeConflict(t *testing.T) {
	store, executor := agentTestStore(), &referenceExecutorStub{}
	s, req := newReferenceService(t, store, executor), referenceServiceRequest()
	first, err := s.Submit(context.Background(), 7, req)
	if err != nil || first.State != agents.Accepted || first.Request != req || executor.seen != req || executor.submitted != 1 || executor.submits.Load() != 0 {
		t.Fatal("显式对象未由专用执行器提交")
	}
	second, err := s.Submit(context.Background(), 7, req)
	if err != nil || first != second || executor.submitted != 1 {
		t.Fatal("同键同体重复提交")
	}
	for _, change := range []func(*agents.Request){func(r *agents.Request) { r.MemberID++ }, func(r *agents.Request) { r.SubjectID++ }, func(r *agents.Request) { r.MaterialsDigest = strings.Repeat("c", 64) }} {
		altered := req
		change(&altered)
		if _, err := s.Submit(context.Background(), 7, altered); !errors.Is(err, agents.ErrConflict) || executor.submitted != 1 {
			t.Fatal("原键被不同资料范围复用")
		}
	}
}

func TestReferenceAgentUnknownPreservesOriginalKey(t *testing.T) {
	store, executor := agentTestStore(), &referenceExecutorStub{submitErr: errors.New("仅测试传输失败")}
	s, req := newReferenceService(t, store, executor), referenceServiceRequest()
	for i := 0; i < 2; i++ {
		r, err := s.Submit(context.Background(), 7, req)
		if !errors.Is(err, agents.ErrUncertain) || r.State != agents.SubmissionUnknown {
			t.Fatal("未知提交被当作可重提")
		}
	}
	executor.checkErr = agents.ErrDisabled
	r, err := s.Show(context.Background(), 7, req.RequestID)
	if err != nil || r.State != agents.SubmissionUnknown || executor.submitted != 1 {
		t.Fatal("撤权后丢失原键核查或重复调用")
	}
}
