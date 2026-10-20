package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestTaskEventRecordScanAndGuards(t *testing.T) {
	values := []any{int64(9), int64(1), "created", 1, 0, "", "2026-10-16 01:02:03.456"}
	v, err := scanTaskEventRecord(taskScanStub{values: values})
	if err != nil || v.TaskID != 9 || v.CreatedAt.Location() != time.UTC || v.CreatedAt.Nanosecond() != 456000000 {
		t.Fatal(v, err)
	}
	fault := errors.New("fake event scan failure")
	if _, err := scanTaskEventRecord(taskScanStub{err: fault}); !errors.Is(err, fault) {
		t.Fatal(err)
	}
	values[6] = "bad"
	if _, err := scanTaskEventRecord(taskScanStub{values: values}); err == nil {
		t.Fatal("坏时点")
	}
	for _, tx := range []*aiTaskReadTx{nil, {}} {
		if _, err := tx.Events(context.Background(), 1, 9, 0, 3, 20); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, args := range [][5]int64{{0, 9, 0, 3, 20}, {1, 0, 0, 3, 20}, {1, 9, -1, 3, 20}, {1, 9, 4, 3, 20}, {1, 9, 0, 3, 0}, {1, 9, 0, 3, 102}} {
		if _, err := (&aiTaskReadTx{tx: new(sql.Tx)}).Events(context.Background(), args[0], args[1], args[2], args[3], int(args[4])); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
}
