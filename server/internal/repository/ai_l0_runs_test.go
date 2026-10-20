package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func TestScanL0RunRejectsBrokenTerminalState(t *testing.T) {
	valid := []any{int64(1), "test_run_request_0001", int64(2), int64(7), "test-vendor", "test-model", "test-written-approval", "running", []byte(nil), "", "2026-10-10 08:00:00.123000", sql.NullString{}}
	if record, err := scanL0Run(auditScanStub{values: valid}); err != nil || record.State != "running" {
		t.Fatalf("有效运行未读出: %+v %v", record, err)
	}
	result := pipeline.ExtractionResult{Attempts: 1, Model: "test-model", Usage: provider.Usage{Total: 30, Prompt: 20, Completion: 10}}
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	succeeded := append([]any(nil), valid...)
	succeeded[7] = "succeeded"
	succeeded[8] = body
	succeeded[11] = sql.NullString{String: "2026-10-10 08:00:01.123000", Valid: true}
	if _, err := scanL0Run(auditScanStub{values: succeeded}); err != nil {
		t.Fatal(err)
	}
	failed := append([]any(nil), succeeded...)
	failed[7] = "failed"
	failed[8] = []byte(nil)
	failed[9] = "extraction_failed"
	if _, err := scanL0Run(auditScanStub{values: failed}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func([]any)
	}{
		{"空身份", func(v []any) { v[3] = int64(0) }},
		{"超长批准", func(v []any) { v[6] = string(make([]byte, 513)) }},
		{"坏请求编号", func(v []any) { v[1] = "bad" }},
		{"错终态", func(v []any) { v[7] = "unknown" }},
		{"空成功内容", func(v []any) { v[8] = []byte(`null`) }},
		{"损坏成功内容", func(v []any) { v[8] = []byte(`{`) }},
		{"错模型", func(v []any) { v[8] = []byte(`{"Attempts":1,"Model":"other"}`) }},
		{"负用量", func(v []any) { v[8] = []byte(`{"Attempts":1,"Model":"test-model","Usage":{"Cached":-1}}`) }},
		{"坏失败码", func(v []any) { v[7] = "failed"; v[8] = []byte(nil); v[9] = "sensitive detail" }},
		{"无结束时间", func(v []any) { v[11] = sql.NullString{} }},
		{"运行带结果", func(v []any) { v[7] = "running"; v[11] = sql.NullString{} }},
		{"坏时间", func(v []any) { v[10] = "bad-time" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := append([]any(nil), succeeded...)
			tc.change(v)
			if _, err := scanL0Run(auditScanStub{values: v}); err == nil {
				t.Fatal("损坏运行记录未拒绝")
			}
		})
	}
	if _, err := scanL0Run(auditScanStub{err: context.Canceled}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestL0RunWriteGuardsBeforeDatabaseAccess(t *testing.T) {
	repo := NewAIL0RunRepo(nil)
	for _, failure := range []string{"sensitive detail", "other"} {
		if _, err := repo.FinishRun(context.Background(), 1, pipeline.ExtractionResult{}, failure); !errors.Is(err, pipeline.ErrInvalidRun) {
			t.Fatal(err)
		}
	}
	if _, err := repo.FinishRun(context.Background(), 0, pipeline.ExtractionResult{}, ""); !errors.Is(err, pipeline.ErrInvalidRun) {
		t.Fatal(err)
	}
	if _, _, err := repo.BeginRun(context.Background(), pipeline.RunRequest{SnapshotID: 2, RequestID: "test_run_request_0001"}, 7, pipeline.RunPolicy{Provider: "test", Model: "test", ApprovalRef: "\n"}); !errors.Is(err, pipeline.ErrInvalidRun) {
		t.Fatal(err)
	}
	if err := repo.AppendCallEvent(context.Background(), pipeline.CallEvent{RunID: 1, Attempt: 3, Phase: "start", Payload: []byte(`{}`)}); !errors.Is(err, pipeline.ErrInvalidRun) {
		t.Fatal(err)
	}
}
