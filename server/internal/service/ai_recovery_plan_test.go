package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type recoveryThemeFixture struct {
	name string
	err  error
}

func (r *recoveryThemeFixture) ThemeForRecovery(context.Context, int64) (string, error) {
	return r.name, r.err
}

func TestAIExtractionRecoveryPlannerBindsCurrentInputs(t *testing.T) {
	snapshot, err := sources.BuildSnapshot(sources.Source{ID: 3, Domain: "exchange.example", Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "fake-recovery-test", LicenseRef: "fake-approval"}, "https://exchange.example/test", sources.MediaPlain, []byte("星河科技主营算力服务。"), time.Now().UTC(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = 42
	reader := &fakeExtractionSnapshotReader{snapshot: snapshot}
	themes := &recoveryThemeFixture{name: "算力"}
	specs := []ExtractionRecoverySpec{{SnapshotID: 42, OperationKey: "recover_operation_0001"}}
	p, err := NewAIExtractionRecoveryPlanner(reader, themes, specs)
	if err != nil {
		t.Fatal(err)
	}
	specs[0].SnapshotID = 99
	v := task.Task{ID: 1, UserID: 2, No: "test-task", SubjectType: task.Theme, SubjectID: 3}
	plan, err := p.Plan(context.Background(), v)
	if err != nil || plan.Validate(v) != nil || len(plan.Steps) != 1 {
		t.Fatal(err)
	}
	if err := plan.Steps[0].Recheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	themes.name = "数据中心"
	if err := plan.Steps[0].Recheck(context.Background()); !errors.Is(err, task.ErrStagePolicy) {
		t.Fatal(err)
	}
	themes.err = errPointsInjected
	if err := plan.Steps[0].Recheck(context.Background()); !errors.Is(err, errPointsInjected) {
		t.Fatal(err)
	}
	themes.err = nil
	themes.name = "算力"
	reader.err = sources.ErrUnauthorized
	if err := plan.Steps[0].Recheck(context.Background()); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := p.Plan(context.Background(), v); err == nil {
		t.Fatal("撤回授权仍构造计划")
	}
	v.SubjectType = task.Topic
	if _, err := p.Plan(context.Background(), v); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("专题被猜成题材", err)
	}
	for _, bad := range [][]ExtractionRecoverySpec{nil, {{SnapshotID: 0, OperationKey: "recover_operation_0001"}}, {{SnapshotID: 42, OperationKey: "short"}}, {{SnapshotID: 42, OperationKey: "recover_operation_0001"}, {SnapshotID: 43, OperationKey: "recover_operation_0001"}}} {
		if _, err := NewAIExtractionRecoveryPlanner(reader, themes, bad); err == nil {
			t.Fatal("无效计划配置")
		}
	}
	if _, err := NewAIExtractionRecoveryPlanner(nil, themes, specs); err == nil {
		t.Fatal("缺快照依赖")
	}
	var absent *AIExtractionRecoveryPlanner
	if _, err := absent.Plan(context.Background(), v); err == nil {
		t.Fatal("空计划器")
	}
}

func TestAITaskInspectIsInternalReadOnly(t *testing.T) {
	s, st := existingTask(t)
	before := st.v
	got, err := s.Inspect(context.Background(), before.UserID, before.ID)
	if err != nil || got.ID != before.ID || st.v.EventSeq != before.EventSeq {
		t.Fatal(got, err)
	}
	if _, err := s.Inspect(context.Background(), 0, before.ID); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	st.commitError = errPointsInjected
	if got, err := s.Inspect(context.Background(), before.UserID, before.ID); err == nil || got != (task.Task{}) {
		t.Fatal(got, err)
	}
}
