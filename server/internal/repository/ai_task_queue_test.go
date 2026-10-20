package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestMemberQueueScansAndGuards(t *testing.T) {
	values := func() []any {
		return []any{int64(2), int64(1), 1, "theme", 2, "2026-10-18 01:02:03.456", "2026-10-18 01:12:03.456", sql.NullString{String: "2026-10-18 01:03:03.456", Valid: true}, true}
	}
	v, err := scanMemberQueueCandidate(taskScanStub{values: values()})
	if err != nil || v.ID != 2 || v.UserID != 1 || !v.HasAdmission || v.LeaseUntil == nil || v.NotBefore.UnixMilli() <= 0 {
		t.Fatal(v, err)
	}
	for _, index := range []int{5, 6, 7} {
		bad := values()
		if index == 7 {
			bad[index] = sql.NullString{String: "bad", Valid: true}
		} else {
			bad[index] = "bad"
		}
		if _, err := scanMemberQueueCandidate(taskScanStub{values: bad}); err == nil {
			t.Fatal("坏时点放行", index)
		}
	}
	absent := values()
	absent[7] = sql.NullString{}
	if v, err := scanMemberQueueCandidate(taskScanStub{values: absent}); err != nil || v.LeaseUntil != nil {
		t.Fatal(v, err)
	}
	if _, err := scanMemberQueueCandidate(taskScanStub{err: points.ErrInvariant}); !errors.Is(err, points.ErrInvariant) {
		t.Fatal(err)
	}
	for _, repo := range []*AITaskRepo{nil, {}} {
		if _, err := repo.MemberQueueRange(context.Background()); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
		if _, err := repo.MemberQueuePage(context.Background(), 0, 2, 1); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, v := range [][3]int64{{-1, 2, 1}, {2, 1, 1}, {0, 2, 0}, {0, 2, 102}} {
		if _, err := (&AITaskRepo{db: new(sql.DB)}).MemberQueuePage(context.Background(), v[0], v[1], int(v[2])); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
}
