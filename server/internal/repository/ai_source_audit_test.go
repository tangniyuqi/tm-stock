package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestScanSourceAuditRejectsMalformedHistory(t *testing.T) {
	source := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport, MinInterval: time.Second}
	after, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	valid := []any{int64(1), int64(7), "register", int64(1), []byte(nil), after, int64(0), "", "", "2026-10-10 08:00:00.123000"}
	if e, err := scanSourceAudit(auditScanStub{values: valid}); err != nil || e.CreatedAt.UnixMilli() != 1791619200123 || e.Before != nil {
		t.Fatalf("审计读取失败: %+v %v", e, err)
	}
	for _, change := range []func([]any){
		func(v []any) { v[0] = int64(0) }, func(v []any) { v[1] = int64(0) },
		func(v []any) { v[2] = "unknown" }, func(v []any) { v[3] = int64(2) },
		func(v []any) { v[4] = []byte("{") }, func(v []any) { v[4] = []byte("null") },
		func(v []any) { v[4] = []byte(strings.Repeat("x", 8193)) }, func(v []any) { v[5] = []byte("{") },
		func(v []any) { v[5] = []byte("null") }, func(v []any) { v[5] = []byte(strings.Repeat("x", 8193)) },
		func(v []any) { v[9] = "bad-time" }, func(v []any) { v[9] = "0001-01-01 00:00:00.000000" },
	} {
		v := append([]any(nil), valid...)
		change(v)
		if _, err := scanSourceAudit(auditScanStub{values: v}); err == nil {
			t.Fatal("损坏历史被接受")
		}
	}
	if _, err := scanSourceAudit(auditScanStub{err: context.Canceled}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	enabled := source
	enabled.Enabled = true
	enabledJSON, _ := json.Marshal(enabled)
	state := append([]any(nil), valid...)
	state[2] = "disable"
	state[4] = enabledJSON
	if e, err := scanSourceAudit(auditScanStub{values: state}); err != nil || e.Before == nil || !e.Before.Enabled {
		t.Fatal(err)
	}
	if err := NewAISourceRepo(nil).WithinSourceTx(context.Background(), nil); !errors.Is(err, sources.ErrInvalidSourceAudit) {
		t.Fatal(err)
	}
	if _, err := NewAISourceRepo(nil).ListSourceAudits(context.Background(), 1, 0, 10); !errors.Is(err, sources.ErrInvalidSourceAudit) {
		t.Fatal(err)
	}
}
