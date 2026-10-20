//go:build integration

package points_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func buildAuditCommand(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "points-audit.exe")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-p", "1", "-race", "-o", binary, "../../cmd/points-audit")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("构建真实对账命令失败：%s；%v", output, err)
	}
	return binary
}

func requireAuditCommand(t *testing.T, e *pointsTestEnv, binary string, code int, expectedEvents []string) {
	t.Helper()
	cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试连接解析失败")
	}
	cfg.DBName = e.database
	cmd := exec.CommandContext(e.ctx, binary, "-page-size", "1", "-round-timeout", "20s")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "TM_DB_DSN=") && !strings.HasPrefix(item, "TM_QUOTE_ENABLED=") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "TM_DB_DSN="+cfg.FormatDSN(), "TM_QUOTE_ENABLED=false")
	output, err := cmd.CombinedOutput()
	actualCode := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal("执行真实对账命令失败", err)
		}
		actualCode = exit.ExitCode()
	}
	if actualCode != code {
		t.Fatalf("对账命令退出码=%d，期望=%d；输出=%s", actualCode, code, output)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	var events []string
	for {
		var event struct {
			Event  string             `json:"event"`
			UserID int64              `json:"user_id"`
			Sweep  *points.AuditSweep `json:"sweep"`
			Failed bool               `json:"failed"`
		}
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal("命令输出不是 JSON 行", err)
		}
		events = append(events, event.Event)
		if event.Event == "points_audit_issue" && event.UserID != 3 {
			t.Fatal("命令漏报或误报用户", event)
		}
		if event.Event == "points_audit_round" && (event.Failed || event.Sweep == nil || !event.Sweep.Complete || event.Sweep.Checked != 4 || event.Sweep.Unhealthy != int64(code/2)) {
			t.Fatal("命令扫描汇总不正确", event)
		}
	}
	if !reflect.DeepEqual(events, expectedEvents) {
		t.Fatal("命令告警事件不完整", events)
	}
}

type boundedAuditScanner struct {
	points.AuditScanner
	afterUpper func() error
}

func (s boundedAuditScanner) AuditUpperBound(ctx context.Context) (int64, error) {
	upper, err := s.AuditScanner.AuditUpperBound(ctx)
	if err != nil {
		return upper, err
	}
	return upper, s.afterUpper()
}

func TestIntegrationPointsAuditSweep(t *testing.T) {
	binary := buildAuditCommand(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, repo, svc, now := pointsFixture(t, level)
			if upper, err := repo.AuditUpperBound(e.ctx); err != nil || upper != 0 {
				t.Fatal(upper, err)
			}
			if users, err := repo.ListAuditUsers(e.ctx, 0, 0, 1); err != nil || len(users) != 0 {
				t.Fatal(users, err)
			}
			grant := func(uid int64) error {
				g := pointGrant(now, 20)
				g.UserID = uid
				g.Key = fmt.Sprintf("test_audit_grant_%03d", uid)
				g.BizID = fmt.Sprintf("audit:grant:%d", uid)
				_, err := svc.Grant(e.ctx, g)
				return err
			}
			for _, uid := range []int64{1, 3, 10} {
				if err := grant(uid); err != nil {
					t.Fatal(err)
				}
			}
			if users, err := repo.ListAuditUsers(e.ctx, 1, 10, 1); err != nil || !reflect.DeepEqual(users, []int64{3}) {
				t.Fatal(users, err)
			}
			if users, err := repo.ListAuditUsers(e.ctx, 3, 10, 1000); err != nil || !reflect.DeepEqual(users, []int64{10}) {
				t.Fatal(users, err)
			}
			for _, args := range [][3]int64{{-1, 10, 1}, {4, 3, 1}, {0, 10, 0}, {0, 10, 1001}} {
				if _, err := repo.ListAuditUsers(e.ctx, args[0], args[1], int(args[2])); !errors.Is(err, points.ErrInvalid) {
					t.Fatal("非法分页未拒绝", err)
				}
			}
			noAlert := func(points.AuditReport) error { t.Fatal("健康账户被告警"); return nil }
			out, err := svc.ReconcileAll(e.ctx, repo, 2, noAlert)
			if err != nil || !out.Complete || out.Checked != 3 || out.Unhealthy != 0 || out.LastUserID != 10 {
				t.Fatal(out, err)
			}
			// 固定上界之后再创建高编号账户，不得让本轮无限追赶新增账户。
			out, err = svc.ReconcileAll(e.ctx, boundedAuditScanner{AuditScanner: repo, afterUpper: func() error { return grant(20) }}, 1, noAlert)
			if err != nil || !out.Complete || out.Checked != 3 || out.UpperUserID != 10 || out.LastUserID != 10 {
				t.Fatal(out, err)
			}
			out, err = svc.ReconcileAll(e.ctx, repo, 1, noAlert)
			if err != nil || !out.Complete || out.Checked != 4 || out.LastUserID != 20 {
				t.Fatal("下一轮漏掉新增账户", out, err)
			}
			requireAuditCommand(t, e, binary, 0, []string{"points_audit_round"})
			// 故意破坏快照但不改事件，真实周期扫描必须发现且不自动修账。
			if _, err := e.conn.ExecContext(e.ctx, "UPDATE tm_points_accounts SET avail=avail+1 WHERE user_id=3"); err != nil {
				t.Fatal(err)
			}
			var alerts []int64
			out, err = svc.ReconcileAll(e.ctx, repo, 2, func(r points.AuditReport) error { alerts = append(alerts, r.UserID); return nil })
			if err != nil || !out.Complete || out.Checked != 4 || out.Unhealthy != 1 || out.Issues == 0 || !reflect.DeepEqual(alerts, []int64{3}) || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=3") != 21 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != 4 {
				t.Fatal(out, err, alerts)
			}
			requireAuditCommand(t, e, binary, 2, []string{"points_audit_issue", "points_audit_round"})
			fault := errors.New("测试告警通道失败")
			out, err = svc.ReconcileAll(e.ctx, repo, 2, func(points.AuditReport) error { return fault })
			if !errors.Is(err, fault) || out.Complete || out.Checked != 2 || out.LastUserID != 1 {
				t.Fatal("告警失败误报完整", out, err)
			}
			// 第二个用户被并发写事务持锁：超时只能返回部分扫描，不能跳过后假报健康。
			lock, err := db.BeginTx(e.ctx, &sql.TxOptions{Isolation: level})
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback()
			if _, err := lock.ExecContext(e.ctx, "UPDATE tm_points_accounts SET avail=avail WHERE user_id=3"); err != nil {
				t.Fatal(err)
			}
			short, cancel := context.WithTimeout(e.ctx, 150*time.Millisecond)
			out, err = svc.ReconcileAll(short, repo, 2, func(points.AuditReport) error { t.Fatal("锁未释放不应读出审计"); return nil })
			cancel()
			if err == nil || out.Complete || out.Checked != 1 || out.LastUserID != 1 {
				t.Fatal("锁等待超时被跳过", out, err)
			}
			if err := lock.Rollback(); err != nil {
				t.Fatal(err)
			}
			out, err = svc.ReconcileAll(e.ctx, repo, 2, func(points.AuditReport) error { return nil })
			if err != nil || !out.Complete || out.Unhealthy != 1 || out.Checked != 4 {
				t.Fatal("超时后的下一轮无法完整重扫", out, err)
			}
			if _, err := e.conn.ExecContext(e.ctx, "ALTER TABLE tm_points_accounts MODIFY avail bigint unsigned NOT NULL DEFAULT 0"); err != nil {
				t.Fatal(err)
			}
			requireAuditCommand(t, e, binary, 1, []string{"points_audit_failure"})
		})
	}
}
