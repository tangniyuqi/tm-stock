package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type pointsTestResult struct {
	count int64
	err   error
}

func (r pointsTestResult) LastInsertId() (int64, error) { return 0, nil }
func (r pointsTestResult) RowsAffected() (int64, error) { return r.count, r.err }

func TestPointsRepositoryGuards(t *testing.T) {
	var absent *PointsRepo
	for _, repo := range []*PointsRepo{absent, NewPointsRepo(nil)} {
		if _, err := repo.AuditUpperBound(context.Background()); !errors.Is(err, points.ErrInvalid) {
			t.Fatal("空仓储扫描上界未拒绝", err)
		}
		if _, err := repo.ListAuditUsers(context.Background(), 0, 1, 1); !errors.Is(err, points.ErrInvalid) {
			t.Fatal("空仓储扫描分页未拒绝", err)
		}
	}
	for _, repo := range []*PointsRepo{absent, NewPointsRepo(nil), NewPointsRepoWithIsolation(new(sql.DB), sql.LevelSerializable)} {
		if err := repo.WithinPointsTx(context.Background(), func(points.Tx) error { t.Fatal("非法配置不应执行动作"); return nil }); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if PointsTx(nil) != nil {
		t.Fatal("空 SQL 事务不应返回可用适配器")
	}
	fault := errors.New("测试数据库故障")
	if !errors.Is(onePointsRow(nil, fault), fault) {
		t.Fatal("未传播更新故障")
	}
	if !errors.Is(onePointsRow(pointsTestResult{err: fault}, nil), fault) {
		t.Fatal("未传播影响行数故障")
	}
	for _, n := range []int64{0, 2} {
		if !errors.Is(onePointsRow(pointsTestResult{count: n}, nil), points.ErrInvariant) {
			t.Fatal("条件更新未严格要求一行")
		}
	}
	if err := onePointsRow(pointsTestResult{count: 1}, nil); err != nil {
		t.Fatal(err)
	}
	if pointsDBError("测试", nil) != nil || !errors.Is(pointsDBError("测试", sql.ErrNoRows), points.ErrNotFound) || !errors.Is(pointsDBError("测试", &mysqldrv.MySQLError{Number: 1062}), points.ErrConflict) {
		t.Fatal("数据库错误分类错误")
	}
}

func TestPointsUTCTimeCodec(t *testing.T) {
	zone := time.FixedZone("测试东八区", 8*60*60)
	value := time.Date(2026, 10, 10, 9, 8, 7, 123000000, zone)
	raw := pointsUTC(value)
	if raw != "2026-10-10 01:08:07.123" {
		t.Fatal(raw)
	}
	decoded, err := parsePointsUTC(raw)
	if err != nil || !decoded.Equal(value) || decoded.Location() != time.UTC {
		t.Fatal(decoded, err)
	}
	for _, raw := range []string{"2026-10-10 01:08:07", "2026-10-10 01:08:07.000"} {
		if _, err := parsePointsUTC(raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"", "0000-00-00 00:00:00", "2026-13-10 01:08:07", "2026-10-10T01:08:07+08:00"} {
		if _, err := parsePointsUTC(raw); err == nil {
			t.Fatal("非法数据库时点未拒绝", raw)
		}
	}
}
