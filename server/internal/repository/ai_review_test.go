package repository

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
)

// 仅覆盖扫描/错误处理边界，不能替代真实数据库的事务与约束测试。
type auditScanStub struct {
	values []any
	err    error
}

func (s auditScanStub) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	for i, value := range s.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

func TestScanReviewAuditRejectsBrokenState(t *testing.T) {
	valid := []any{int64(1), "test_request_00001", int64(2), strings.Repeat("0", 64), int64(7), "approve", "", sql.NullInt64{Int64: 100, Valid: true}, "2026-10-10 08:00:00.123000"}
	event, found, err := scanAudit(auditScanStub{values: valid})
	if err != nil || !found || event.MappingID != 100 || event.ActorID != 7 {
		t.Fatalf("有效审计未读出: %+v %v", event, err)
	}
	for _, tc := range []struct {
		name   string
		change func([]any)
	}{
		{"坏时间", func(v []any) { v[8] = "bad-time" }},
		{"无身份", func(v []any) { v[4] = int64(0) }},
		{"通过无归属", func(v []any) { v[7] = sql.NullInt64{} }},
		{"驳回带零归属", func(v []any) { v[5] = "reject"; v[6] = "证据不足"; v[7] = sql.NullInt64{Int64: 0, Valid: true} }},
		{"坏决定", func(v []any) { v[5] = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := append([]any(nil), valid...)
			tc.change(values)
			if _, _, err := scanAudit(auditScanStub{values: values}); err == nil {
				t.Fatal("损坏审计未被拒绝")
			}
		})
	}
	if _, found, err := scanAudit(auditScanStub{err: sql.ErrNoRows}); err != nil || found {
		t.Fatal("未找到的审计处理有误")
	}
	if _, _, err := scanAudit(auditScanStub{err: context.Canceled}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAIReviewRepositoryGuardsAndDuplicateConflict(t *testing.T) {
	var repo *AIReviewRepo
	if err := repo.WithinReviewTx(context.Background(), nil); err == nil {
		t.Fatal("空事务未拒绝")
	}
	if _, err := repo.ListReviewQueue(context.Background(), 0, 20); err == nil {
		t.Fatal("空仓储未拒绝")
	}
	if err := reviewWriteError("测试", &mysqldrv.MySQLError{Number: 1062}); !errors.Is(err, candidate.ErrConflict) {
		t.Fatal(err)
	}
	if err := reviewWriteError("测试", context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
