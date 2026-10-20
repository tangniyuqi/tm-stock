package repository

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type taskScanStub struct {
	values []any
	err    error
}

func (s taskScanStub) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	for i, value := range s.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}
func taskScanValues() []any {
	return []any{int64(1), int64(2), strings.Repeat("a", 32), "test_task_create_0001", strings.Repeat("a", 64), 1, task.Theme, int64(1), int64(3), "v1", int64(7), []byte(`{"max_attempts":2,"max_calls":4,"max_tokens":1000,"max_pages":3,"max_duration_ms":1000}`), task.Running, 1, 0, "2026-10-11 00:00:00.000", "2026-10-11 00:00:30.000", sql.NullString{String: "worker_test_00001", Valid: true}, sql.NullString{String: "2026-10-11 00:00:10.000", Valid: true}, int64(1), int64(2), false, ""}
}

func TestAITaskScanAndRepositoryGuards(t *testing.T) {
	fault := errors.New("测试任务读取失败")
	if _, err := scanAITask(taskScanStub{err: fault}); !errors.Is(err, fault) {
		t.Fatal(err)
	}
	for _, index := range []int{11, 15, 16, 18} {
		values := taskScanValues()
		if index == 11 {
			values[index] = []byte("bad")
		} else if index == 18 {
			values[index] = sql.NullString{String: "bad", Valid: true}
		} else {
			values[index] = "bad"
		}
		if _, err := scanAITask(taskScanStub{values: values}); err == nil {
			t.Fatal("损坏任务副本放行", index)
		}
	}
	values := taskScanValues()
	values[11] = []byte(`{}`)
	if _, err := scanAITask(taskScanStub{values: values}); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	v, err := scanAITask(taskScanStub{values: taskScanValues()})
	if err != nil || v.ID != 1 || v.UserID != 2 || v.LeaseUntil == nil || v.Fence != 1 {
		t.Fatal(v, err)
	}
	owner, until := taskLeaseValues(v)
	if owner != v.LeaseOwner || until != "2026-10-11 00:00:10.000" {
		t.Fatal(owner, until)
	}
	values = taskScanValues()
	values[17], values[18] = sql.NullString{}, sql.NullString{}
	v, err = scanAITask(taskScanStub{values: values})
	if err != nil || v.LeaseUntil != nil || v.LeaseOwner != "" {
		t.Fatal(v, err)
	}
	if owner, until := taskLeaseValues(v); owner != nil || until != nil {
		t.Fatal(owner, until)
	}
	var absent *AITaskRepo
	for _, repo := range []*AITaskRepo{absent, NewAITaskRepo(nil), NewAITaskRepoWithIsolation(new(sql.DB), sql.LevelSerializable)} {
		if err := repo.WithinTaskTx(context.Background(), func(task.Tx) error { t.Fatal("无效仓储执行事务"); return nil }); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, repo := range []*AITaskRepo{absent, NewAITaskRepo(nil)} {
		if _, err := repo.ClaimCandidates(context.Background(), 1); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, limit := range []int{0, 101} {
		if _, err := NewAITaskRepo(new(sql.DB)).ClaimCandidates(context.Background(), limit); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if AITaskTx(nil) != nil {
		t.Fatal("空 SQL 事务可用")
	}
}
