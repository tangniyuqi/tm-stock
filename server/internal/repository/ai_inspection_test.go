package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
)

func TestInspectionScanGuards(t *testing.T) {
	batch := []any{int64(1), "test_inspection_batch_0001", int64(8), int64(0), 10, 2, 3, 2, int64(3), strings.Repeat("0", 64), []byte(`[1,2,3]`), "2026-10-10 08:00:00.123000"}
	if b, found, err := scanInspectionBatch(auditScanStub{values: batch}); err != nil || !found || b.EligibleCount != 2 {
		t.Fatal(err)
	}
	for _, change := range []func([]any){func(v []any) { v[4] = 2001 }, func(v []any) { v[7] = 4 }, func(v []any) { v[10] = []byte(`null`) }, func(v []any) { v[10] = []byte(`[1,1,3]`) }, func(v []any) { v[10] = []byte(`[1,2,4]`) }, func(v []any) { v[8] = int64(4) }, func(v []any) { v[9] = "bad" }, func(v []any) { v[11] = "bad-time" }} {
		v := append([]any(nil), batch...)
		change(v)
		if _, _, err := scanInspectionBatch(auditScanStub{values: v}); err == nil {
			t.Fatal("损坏批次未拒绝")
		}
	}
	event := []any{int64(1), "test_inspection_decision_0001", int64(2), strings.Repeat("0", 64), int64(8), "confirm", "", int64(100), "2026-10-10 08:00:00.123000"}
	if e, found, err := scanInspectionEvent(auditScanStub{values: event}); err != nil || !found || e.MappingID != 100 {
		t.Fatal(err)
	}
	for _, change := range []func([]any){func(v []any) { v[4] = int64(0) }, func(v []any) { v[5] = "issue" }, func(v []any) { v[7] = int64(0) }, func(v []any) { v[8] = "bad-time" }} {
		v := append([]any(nil), event...)
		change(v)
		if _, _, err := scanInspectionEvent(auditScanStub{values: v}); err == nil {
			t.Fatal("损坏复核事件未拒绝")
		}
	}
	if _, found, err := scanInspectionBatch(auditScanStub{err: sql.ErrNoRows}); err != nil || found {
		t.Fatal(err)
	}
	if _, found, err := scanInspectionEvent(auditScanStub{err: sql.ErrNoRows}); err != nil || found {
		t.Fatal(err)
	}
	if _, _, err := scanInspectionEvent(auditScanStub{err: context.Canceled}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := NewAIInspectionRepo(nil).WithinInspectionTx(context.Background(), nil); !errors.Is(err, candidate.ErrInvalidInspection) {
		t.Fatal(err)
	}
}
