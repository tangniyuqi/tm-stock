package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestExpiryScansAndGuards(t *testing.T) {
	v, err := scanExpiryRange(taskScanStub{values: []any{int64(9), "2026-10-16 01:02:03.456"}})
	if err != nil || v.UpperID != 9 || v.AsOf.Location() != time.UTC || v.AsOf.Nanosecond() != 456000000 {
		t.Fatal(v, err)
	}
	c, err := scanExpiryCandidate(taskScanStub{values: []any{int64(9), int64(2), 1, "2026-10-16 01:02:03.456"}})
	if err != nil || c.ID != 9 || c.UserID != 2 || c.Deadline != v.AsOf {
		t.Fatal(c, err)
	}
	fault := errors.New("fake expiry scan failure")
	for _, row := range []taskScanStub{{err: fault}, {values: []any{int64(9), "bad"}}} {
		if _, err := scanExpiryRange(row); err == nil {
			t.Fatal("范围坏值未拒绝")
		}
	}
	for _, row := range []taskScanStub{{err: fault}, {values: []any{int64(9), int64(2), 1, "bad"}}} {
		if _, err := scanExpiryCandidate(row); err == nil {
			t.Fatal("任务坏值未拒绝")
		}
	}
	for _, r := range []*AITaskRepo{nil, {}} {
		if _, err := r.ExpiryRange(context.Background()); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
		if _, err := r.ExpiryPage(context.Background(), 0, 9, 1); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, a := range [][3]int64{{-1, 9, 1}, {2, 1, 1}, {0, 9, 0}, {0, 9, 1001}} {
		if _, err := (&AITaskRepo{db: new(sql.DB)}).ExpiryPage(context.Background(), a[0], a[1], int(a[2])); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
}
