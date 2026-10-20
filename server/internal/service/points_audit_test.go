package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type auditScanStub struct {
	upper             int64
	pages             [][]int64
	upperErr, pageErr error
	calls             int
	limits            []int64
}

func (s *auditScanStub) AuditUpperBound(context.Context) (int64, error) { return s.upper, s.upperErr }
func (s *auditScanStub) ListAuditUsers(_ context.Context, after, upper int64, limit int) ([]int64, error) {
	s.calls++
	s.limits = append(s.limits, after, upper, int64(limit))
	if len(s.pages) == 0 {
		return nil, s.pageErr
	}
	page := s.pages[0]
	s.pages = s.pages[1:]
	return page, nil
}

type auditStoreStub struct {
	points.Tx
	seen               []int64
	bad, fail          int64
	readErr, commitErr error
	ctx                context.Context
}

func (s *auditStoreStub) WithinPointsTx(ctx context.Context, fn func(points.Tx) error) error {
	s.ctx = ctx
	if err := fn(s); err != nil {
		return err
	}
	return s.commitErr
}
func (s *auditStoreStub) ReadAudit(ctx context.Context, uid int64) (points.AuditRows, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 30*time.Second {
		return points.AuditRows{}, points.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return points.AuditRows{}, err
	}
	s.seen = append(s.seen, uid)
	if uid == s.fail {
		return points.AuditRows{}, s.readErr
	}
	a := points.Account{UserID: uid}
	if uid == s.bad {
		a.Avail = 1
	}
	return points.AuditRows{Account: a}, nil
}

func TestPointsAuditSweepPagingAndAlerts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	{
		store := &auditStoreStub{bad: 4}
		svc, err := NewPointsService(store)
		if err != nil {
			t.Fatal(err)
		}
		scan := &auditScanStub{upper: 9, pages: [][]int64{{1, 4}, {9}}}
		var alerts []int64
		out, err := svc.ReconcileAll(ctx, scan, 2, func(report points.AuditReport) error {
			if report.Healthy() {
				t.Fatal("健康账户不应告警")
			}
			alerts = append(alerts, report.UserID)
			return nil
		})
		if err != nil || !out.Complete || out.Checked != 3 || out.Unhealthy != 1 || out.Issues == 0 || out.LastUserID != 9 || out.UpperUserID != scan.upper || !reflect.DeepEqual(store.seen, []int64{1, 4, 9}) || !reflect.DeepEqual(alerts, []int64{4}) {
			t.Fatalf("分页或异常阻止了其余账户对账：%+v %v", out, err)
		}
		if !reflect.DeepEqual(scan.limits[:6], []int64{0, scan.upper, 2, 4, scan.upper, 2}) {
			t.Fatal("上界未固定或游标未推进", scan.limits)
		}
		if !errors.Is(store.ctx.Err(), context.Canceled) {
			t.Fatal("用户事务退出未取消上下文")
		}
	}
	store := &auditStoreStub{}
	svc, _ := NewPointsService(store)
	out, err := svc.ReconcileAll(ctx, &auditScanStub{}, 1, func(points.AuditReport) error { t.Fatal("空库不应告警"); return nil })
	if err != nil || !out.Complete || out.Checked != 0 || len(store.seen) != 0 {
		t.Fatal(out, err)
	}
}

func TestPointsAuditSweepFailureAndBadCursors(t *testing.T) {
	fault := errors.New("测试分页故障")
	for _, tc := range []struct {
		name          string
		scan          auditScanStub
		store         auditStoreStub
		notifyErr     error
		want          error
		checked, last int64
	}{
		{name: "上界读取失败", scan: auditScanStub{upperErr: fault}, want: fault},
		{name: "负上界", scan: auditScanStub{upper: -1}, want: points.ErrInvariant},
		{name: "分页失败", scan: auditScanStub{upper: 2, pageErr: fault}, want: fault},
		{name: "上界前空页", scan: auditScanStub{upper: 2}, want: points.ErrInvariant},
		{name: "后页上界丢失", scan: auditScanStub{upper: 2, pages: [][]int64{{1}}}, want: points.ErrInvariant, checked: 1, last: 1},
		{name: "零用户", scan: auditScanStub{upper: 2, pages: [][]int64{{0}}}, want: points.ErrInvariant},
		{name: "越上界", scan: auditScanStub{upper: 2, pages: [][]int64{{3}}}, want: points.ErrInvariant},
		{name: "重复游标", scan: auditScanStub{upper: 2, pages: [][]int64{{1, 1}}}, want: points.ErrInvariant},
		{name: "逆序", scan: auditScanStub{upper: 2, pages: [][]int64{{2, 1}}}, want: points.ErrInvariant},
		{name: "超出页长", scan: auditScanStub{upper: 3, pages: [][]int64{{1, 2, 3}}}, want: points.ErrInvariant},
		{name: "跨页回退", scan: auditScanStub{upper: 3, pages: [][]int64{{1}, {1}}}, want: points.ErrInvariant, checked: 1, last: 1},
		{name: "对账访问失败", scan: auditScanStub{upper: 2, pages: [][]int64{{1, 2}}}, store: auditStoreStub{fail: 2, readErr: fault}, want: fault, checked: 1, last: 1},
		{name: "提交失败", scan: auditScanStub{upper: 1, pages: [][]int64{{1}}}, store: auditStoreStub{commitErr: fault}, want: fault},
		{name: "告警失败", scan: auditScanStub{upper: 2, pages: [][]int64{{1, 2}}}, store: auditStoreStub{bad: 1}, notifyErr: fault, want: fault, checked: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			svc, _ := NewPointsService(&tc.store)
			out, err := svc.ReconcileAll(ctx, &tc.scan, 2, func(points.AuditReport) error { return tc.notifyErr })
			if !errors.Is(err, tc.want) || out.Complete || out.Checked != tc.checked || out.LastUserID != tc.last {
				t.Fatal(out, err)
			}
		})
	}
}

func TestPointsAuditSweepGuardsAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store := &auditStoreStub{}
	svc, _ := NewPointsService(store)
	scan := &auditScanStub{upper: 2, pages: [][]int64{{1}, {2}}}
	notify := func(points.AuditReport) error { return nil }
	for _, size := range []int{0, 1001} {
		if _, err := svc.ReconcileAll(ctx, scan, size, notify); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	var absent *PointsService
	for _, fn := range []func() (points.AuditSweep, error){
		func() (points.AuditSweep, error) { return absent.ReconcileAll(ctx, scan, 1, notify) },
		func() (points.AuditSweep, error) { return svc.ReconcileAll(ctx, nil, 1, notify) },
		func() (points.AuditSweep, error) { return svc.ReconcileAll(ctx, scan, 1, nil) },
		func() (points.AuditSweep, error) { return svc.ReconcileAll(context.Background(), scan, 1, notify) },
	} {
		if _, err := fn(); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
	store.bad = 1
	out, err := svc.ReconcileAll(ctx, scan, 1, func(points.AuditReport) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) || out.Complete || out.Checked != 1 || scan.calls != 1 {
		t.Fatal(out, err)
	}
	out, err = svc.ReconcileAll(ctx, scan, 1, notify)
	if !errors.Is(err, context.Canceled) || out.Complete {
		t.Fatal(out, err)
	}
}
