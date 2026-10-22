//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type reportEnv struct {
	*pointsTestEnv
	db                   *sql.DB
	repo                 *repository.AITaskRepo
	tasks                *service.AITaskService
	ledger               *service.PointsService
	svc                  *service.AIReportService
	policy               *finalreport.Policy
	r                    finalreport.Request
	v                    task.Task
	snap                 sources.Snapshot
	source               sources.Source
	calls                atomic.Int32
	hook                 func(context.Context) error
	entailment, semantic finalreport.Verdict
	deepPolicy           *finalreport.Policy
}

func reportDDL(t *testing.T, e *pointsTestEnv, name string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(b), "DELIMITER $$", "")
	s = strings.ReplaceAll(s, "DELIMITER ;", "")
	s = strings.ReplaceAll(s, "END$$", "END;")
	e.mustExec(s)
	e.mustExec(s)
}

func newReportEnv(t *testing.T, level sql.IsolationLevel, lease time.Duration) *reportEnv {
	return newReportEnvWithStage(t, level, lease, true)
}

func newReportEnvWithStage(t *testing.T, level sql.IsolationLevel, lease time.Duration, withStage bool) *reportEnv {
	return newReportEnvWithBudget(t, level, lease, withStage, taskBudget())
}

func newReportEnvWithBudget(t *testing.T, level sql.IsolationLevel, lease time.Duration, withStage bool, budget task.Budget) *reportEnv {
	t.Helper()
	e, db, repo, tasks, ledger, now := persistentTaskFixture(t, level)
	reportDDL(t, e, "20261010_create_ai_source_snapshots.sql")
	reportDDL(t, e, "20261016_create_tm_ai_report_commits.sql")
	reportDDL(t, e, "20261018_create_tm_ai_task_admissions.sql")
	e.mustExec(`CREATE TABLE addon_quant_theme(id bigint UNSIGNED PRIMARY KEY,name varchar(100),status tinyint,deleted_at datetime(3));INSERT INTO addon_quant_theme VALUES(1,'示例题材',1,NULL)`)
	x := &reportEnv{pointsTestEnv: e, db: db, repo: repo, tasks: tasks, ledger: ledger, entailment: finalreport.Pass, semantic: finalreport.Pass}
	if _, err := ledger.Grant(e.ctx, pointGrant(now, 20)); err != nil {
		t.Fatal(err)
	}
	src := repository.NewAISourceRepo(db)
	var err error
	x.source, err = src.RegisterSource(e.ctx, sources.Source{Domain: "evidence.example.test", Category: sources.CategoryAnnualReport, LicenseTag: "test-only", LicenseRef: "test-only-private-license"})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.SetSourceEnabled(e.ctx, x.source.ID, true); err != nil {
		t.Fatal(err)
	}
	x.source.Enabled = true
	snap, err := sources.BuildSnapshot(x.source, "https://evidence.example.test/example", sources.MediaPlain, []byte("示例题材是本测试资料的主题。示例题材的事项发生于资料记载日期。示例题材包含资料所述环节。示例题材的公司信息见本段。"), now, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	x.snap, err = src.SaveSnapshot(e.ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	cfg := finalreport.Config{Ref: "test-only-policy", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{x.source.Domain}}
	x.policy, err = finalreport.NewPolicy(cfg, func(ctx context.Context, in finalreport.EntailmentInput) (finalreport.Verdict, error) {
		x.calls.Add(1)
		return x.entailment, nil
	}, func(ctx context.Context, text string) (finalreport.Verdict, error) {
		x.calls.Add(1)
		if x.hook != nil {
			if err := x.hook(ctx); err != nil {
				return finalreport.Uncertain, err
			}
		}
		return x.semantic, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	budget.MaxDurationMS = 300000
	created, err := tasks.Create(e.ctx, taskRequest(99001), points.Price{Version: "v1", Points: 7}, budget)
	if err != nil {
		t.Fatal(err)
	}
	x.v, err = tasks.Claim(e.ctx, 1, created.TaskID, "worker_report_test_001", lease)
	if err != nil {
		t.Fatal(err)
	}
	x.svc, err = service.NewAIReportService(repo, ledger, x.policy, true)
	if err != nil {
		t.Fatal(err)
	}
	if !withStage {
		return x
	}
	p := finalreport.Proposal{TaskNo: x.v.No, ThemeID: 1, ThemeName: "示例题材", Report: render.Report{SchemaVer: 1, Disclosure: render.Disclosure{AIGenerated: true, ModelName: cfg.ModelName, FilingNo: cfg.FilingNo}}}
	for i, key := range []render.SectionKey{render.SecDefinition, render.SecTimeline, render.SecSegment, render.SecCompanies} {
		q := x.snap.Sentences[i]
		node, err := render.QuoteNode(render.Quote{SnapshotID: x.snap.ID, Start: q.StartByte, End: q.EndByte, Text: q.Text, SourceType: "年报", URL: x.snap.URL, FetchedAt: x.snap.FetchedAt.UnixMilli(), ContentHash: x.snap.Hash}, render.ProvAIExtractedQuote)
		if err != nil {
			t.Fatal(err)
		}
		title, _ := render.SectionTitle(key)
		p.Report.Sections = append(p.Report.Sections, render.Section{Key: key, Title: title, AILabel: true, Nodes: []render.Node{node}})
	}
	x.r = finalreport.Request{TaskNo: x.v.No, Stage: task.StageRequest{UserID: 1, TaskID: x.v.ID, Owner: x.v.LeaseOwner, Fence: x.v.Fence, OperationKey: "report_test_operation", InputDigest: task.StageDigest([]byte("test-only-report-input")), StepNo: 1, Stage: "judge"}}
	b, err := service.NewAITaskBudgetService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Reserve(e.ctx, task.ReserveRequest{UserID: 1, TaskID: x.v.ID, Owner: x.v.LeaseOwner, Fence: x.v.Fence, Key: x.r.Stage.OperationKey, InputDigest: x.r.Stage.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
		t.Fatal(err)
	}
	stage, err := service.NewAITaskStageService(repo, map[string]task.StagePolicy{"judge": x.policy.StagePolicy()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Capture(e.ctx, x.r.Stage, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Complete(e.ctx, x.r.Stage); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestIntegrationAIReportLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newReportEnv(t, level, time.Minute)
			if err := repository.NewAIReportSchemaVerifier(x.db).Verify(x.ctx); err != nil {
				t.Fatal(err)
			}
			x.hook = func(ctx context.Context) error {
				tx, err := x.db.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				defer tx.Rollback()
				var id int64
				return tx.QueryRowContext(ctx, `SELECT id FROM tm_ai_tasks WHERE id=? FOR UPDATE NOWAIT`, x.v.ID).Scan(&id)
			}
			got, err := x.svc.Finalize(x.ctx, x.r)
			if err != nil || got.Status != task.Succeeded || got.ReportDigest == "" {
				t.Fatal(got, err)
			}
			if x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3 AND payload IS NOT NULL") != 1 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 || x.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 13 || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("报告、任务、锁价结算未闭环")
			}
			requirePointAudit(t, x.pointsTestEnv, x.ledger)
			if err := repository.NewAISourceRepo(x.db).SetSourceEnabled(x.ctx, x.source.ID, false); err != nil {
				t.Fatal(err)
			}
			calls := x.calls.Load()
			again, err := x.svc.Finalize(x.ctx, x.r)
			if err != nil || got != again || calls != x.calls.Load() {
				t.Fatal("重放重复校验/结算", again, err)
			}
			bad := x.r
			bad.Stage.OperationKey = "report_other_operation"
			if _, err := x.svc.Finalize(x.ctx, bad); err == nil {
				t.Fatal("异操作键重放")
			}
		})
	}
}

func TestIntegrationAIReportBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"关闭", "语义不明", "蕴含拒绝", "来源撤回", "授权到期", "授权副本变更", "题材改名", "取消", "旧围栏", "未知预留"} {
				t.Run(name, func(t *testing.T) {
					x := newReportEnv(t, level, time.Minute)
					switch name {
					case "关闭":
						var err error
						x.svc, err = service.NewAIReportService(x.repo, x.ledger, nil, false)
						if err != nil {
							t.Fatal(err)
						}
					case "语义不明":
						x.semantic = finalreport.Uncertain
					case "蕴含拒绝":
						x.entailment = finalreport.Block
					case "来源撤回":
						x.hook = func(ctx context.Context) error {
							return repository.NewAISourceRepo(x.db).SetSourceEnabled(ctx, x.source.ID, false)
						}
					case "授权到期":
						x.hook = func(ctx context.Context) error {
							_, err := x.db.ExecContext(ctx, `UPDATE tm_ai_sources SET license_expires_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE id=?`, x.source.ID)
							return err
						}
					case "授权副本变更":
						x.hook = func(ctx context.Context) error {
							_, err := x.db.ExecContext(ctx, `UPDATE tm_ai_sources SET license_ref='test-only-changed-license' WHERE id=?`, x.source.ID)
							return err
						}
					case "题材改名":
						x.hook = func(ctx context.Context) error {
							_, err := x.db.ExecContext(ctx, `UPDATE addon_quant_theme SET name='已改名示例' WHERE id=1`)
							return err
						}
					case "取消":
						x.hook = func(ctx context.Context) error { _, err := x.tasks.Cancel(ctx, 1, x.v.ID); return err }
					case "旧围栏":
						x.r.Stage.Fence++
					case "未知预留":
						b, err := service.NewAITaskBudgetService(x.repo)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := b.Reserve(x.ctx, task.ReserveRequest{UserID: 1, TaskID: x.v.ID, Owner: x.v.LeaseOwner, Fence: x.v.Fence, Key: "report_unknown_operation", InputDigest: strings.Repeat("f", 64), Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
							t.Fatal(err)
						}
					}
					got, err := x.svc.Finalize(x.ctx, x.r)
					if name == "关闭" || name == "旧围栏" || name == "未知预留" {
						if err == nil || got != (finalreport.Receipt{}) || x.calls.Load() != 0 || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 {
							t.Fatal("关闭/失效/未知仍终结", got, err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					want := task.Failed
					if name == "取消" {
						want = task.Cancelled
					}
					if got.Status != want || got.ReportDigest != "" || x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE payload IS NOT NULL") != 0 || x.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 0 {
						t.Fatal("拒绝/取消未整体释放", got)
					}
					requirePointAudit(t, x.pointsTestEnv, x.ledger)
					if want == task.Failed {
						calls := x.calls.Load()
						x.hook = nil
						x.entailment = finalreport.Pass
						x.semantic = finalreport.Pass
						again, err := x.svc.Finalize(x.ctx, x.r)
						if err != nil || again != got || x.calls.Load() != calls {
							t.Fatal("拒绝重放重新校验/扣费", again, err)
						}
					}
				})
			}
		})
	}
}

type reportCommitUnknown struct {
	store finalreport.Store
	n     int
}

func (s *reportCommitUnknown) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	s.n++
	if err := s.store.WithinReportTx(ctx, fn); err != nil {
		return err
	}
	if s.n == 3 {
		return errors.New("test-only:真实提交后回执不明")
	}
	return nil
}

func TestIntegrationAIReportAtomicity(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"报告收据故障", "成功事件故障", "账务流水故障", "提交回执未知"} {
				t.Run(name, func(t *testing.T) {
					x := newReportEnv(t, level, time.Minute)
					switch name {
					case "报告收据故障":
						x.mustExec(`CREATE TRIGGER test_report_abort BEFORE INSERT ON tm_ai_report_commits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only report failure'`)
					case "成功事件故障":
						x.mustExec(`CREATE TRIGGER test_report_event_abort BEFORE INSERT ON tm_ai_task_events FOR EACH ROW BEGIN IF NEW.type='succeeded' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only event failure'; END IF; END`)
					case "账务流水故障":
						x.mustExec(`CREATE TRIGGER test_report_points_abort BEFORE INSERT ON tm_points_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only entry failure'`)
					case "提交回执未知":
						var err error
						x.svc, err = service.NewAIReportService(&reportCommitUnknown{store: x.repo}, x.ledger, x.policy, true)
						if err != nil {
							t.Fatal(err)
						}
					}
					got, err := x.svc.Finalize(x.ctx, x.r)
					if err == nil || got != (finalreport.Receipt{}) {
						t.Fatal("故障返回候选成功", got, err)
					}
					if name == "提交回执未知" {
						again, err := x.svc.Finalize(x.ctx, x.r)
						if err != nil || again.Status != task.Succeeded || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 || x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 1 {
							t.Fatal("提交后未知重复结算", again, err)
						}
					} else {
						if x.queryInt("SELECT status FROM tm_ai_tasks") != task.Running || x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 0 || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || x.queryInt("SELECT state FROM tm_points_holds") != points.Holding {
							t.Fatal("事务故障留下半成品")
						}
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newReportEnv(t, level, time.Minute)
			start := make(chan struct{})
			errs := make(chan error, 40)
			receipts := make(chan finalreport.Receipt, 40)
			var wg sync.WaitGroup
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); <-start; r, err := x.svc.Finalize(x.ctx, x.r); errs <- err; receipts <- r }()
			}
			close(start)
			wg.Wait()
			close(errs)
			close(receipts)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			var first finalreport.Receipt
			for r := range receipts {
				if first.TaskID == 0 {
					first = r
				}
				if r != first || r.Status != task.Succeeded {
					t.Fatal("并发收据不稳定")
				}
			}
			if x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 1 || x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != 1 || x.queryInt("SELECT COUNT(*) FROM tm_ai_task_events WHERE type='succeeded'") != 1 {
				t.Fatal("重复报告/结算/成功事件")
			}
			requirePointAudit(t, x.pointsTestEnv, x.ledger)
		})
	}
}

func TestIntegrationAIReportSchema(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for name, ddl := range map[string]string{
				"缺插入保护": "DROP TRIGGER tm_ai_report_insert", "缺修改保护": "DROP TRIGGER tm_ai_report_no_update", "缺删除保护": "DROP TRIGGER tm_ai_report_no_delete",
				"缺CHECK": "ALTER TABLE tm_ai_report_commits DROP CHECK chk_tm_ai_report", "缺事件外键": "ALTER TABLE tm_ai_report_commits DROP FOREIGN KEY fk_tm_ai_report_event",
				"错误正文空值": "ALTER TABLE tm_ai_report_commits MODIFY payload mediumblob NOT NULL", "受限账号": "",
			} {
				t.Run(name, func(t *testing.T) {
					x := newReportEnv(t, level, time.Minute)
					verifier := repository.NewAIReportSchemaVerifier(x.db)
					if err := verifier.Verify(x.ctx); err != nil {
						t.Fatal("破坏前自检基线失败", err)
					}
					if ddl != "" {
						x.mustExec(ddl)
						if err := verifier.Verify(x.ctx); err == nil {
							t.Fatal("报告保护缺失仍放行")
						}
						if x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 {
							t.Fatal("失败自检留下报告探针")
						}
						return
					}
					cfg, err := mysqldrv.ParseDSN(os.Getenv("TM_TEST_DSN"))
					if err != nil {
						t.Fatal(err)
					}
					cfg.DBName = x.database
					var random [16]byte
					if _, err := rand.Read(random[:]); err != nil {
						t.Fatal(err)
					}
					user := "report_it_" + hex.EncodeToString(random[:6])
					cfg.User = user
					cfg.Passwd = hex.EncodeToString(random[:])
					x.mustExec("CREATE USER '" + user + "'@'%' IDENTIFIED BY '" + cfg.Passwd + "'")
					t.Cleanup(func() { x.mustExec("DROP USER '" + user + "'@'%'") })
					for _, table := range append(append([]string(nil), taskProbeTables...), "tm_ai_report_commits", "tm_ai_task_admissions") {
						x.mustExec("GRANT SELECT, INSERT, UPDATE, DELETE ON `" + x.database + "`." + table + " TO '" + user + "'@'%'")
					}
					for _, table := range []string{"tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies", "addon_quant_theme"} {
						x.mustExec("GRANT SELECT ON `" + x.database + "`." + table + " TO '" + user + "'@'%'")
					}
					db, err := sql.Open("mysql", cfg.FormatDSN())
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					if _, err := db.ExecContext(x.ctx, "CREATE TABLE report_forbidden_ddl(id INT)"); err == nil {
						t.Fatal("报告账号不应有DDL权限")
					}
					if err := repository.NewAIReportSchemaVerifier(db).Verify(x.ctx); err != nil {
						t.Fatal("受限账号自检失败", err)
					}
					svc, err := service.NewAIReportService(repository.NewAITaskRepoWithIsolation(db, level), x.ledger, x.policy, true)
					if err != nil {
						t.Fatal(err)
					}
					if got, err := svc.Finalize(x.ctx, x.r); err != nil || got.Status != task.Succeeded {
						t.Fatal("仅SELECT来源权限不能终检", got, err)
					}
				})
			}
		})
	}
}

type reportLockStore struct {
	store    finalreport.Store
	db       *sql.DB
	sourceID int64
	n        int
}

func (s *reportLockStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	s.n++
	return s.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		return fn(&reportLockTx{Tx: tx, db: s.db, sourceID: s.sourceID, check: s.n == 3})
	})
}

type reportLockTx struct {
	finalreport.Tx
	db       *sql.DB
	sourceID int64
	check    bool
}

func (t *reportLockTx) ReportEvidence(ctx context.Context, ids []int64, themeID int64) ([]sources.Snapshot, string, error) {
	snaps, name, err := t.Tx.ReportEvidence(ctx, ids, themeID)
	if err != nil || !t.check {
		return snaps, name, err
	}
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM tm_ai_sources WHERE id=? FOR UPDATE NOWAIT`, t.sourceID).Scan(&id)
	var locked *mysqldrv.MySQLError
	if !errors.As(err, &locked) || locked.Number != 3572 {
		return nil, "", errors.New("test-only:最终来源共享锁未阻止撤回")
	}
	return snaps, name, nil
}

func TestIntegrationAIReportRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, name := range []string{"租约接管", "来源提交锁", "取消成功竞争"} {
				t.Run(name, func(t *testing.T) {
					lease := time.Minute
					if name == "租约接管" {
						lease = time.Second
					}
					x := newReportEnv(t, level, lease)
					switch name {
					case "租约接管":
						var next task.Task
						x.hook = func(ctx context.Context) error {
							waitTaskTime(t, x.pointsTestEnv, *x.v.LeaseUntil)
							var err error
							next, err = x.tasks.Claim(ctx, 1, x.v.ID, "worker_report_takeover", time.Minute)
							return err
						}
						got, err := x.svc.Finalize(x.ctx, x.r)
						if !errors.Is(err, task.ErrLease) || got != (finalreport.Receipt{}) || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits") != 0 {
							t.Fatal("旧围栏仍提交报告", got, err)
						}
						x.hook = nil
						x.r.Stage.Owner = next.LeaseOwner
						x.r.Stage.Fence = next.Fence
						if got, err := x.svc.Finalize(x.ctx, x.r); err != nil || got.Status != task.Succeeded {
							t.Fatal("新租约不能复用已捕获报告", got, err)
						}
					case "来源提交锁":
						svc, err := service.NewAIReportService(&reportLockStore{store: x.repo, db: x.db, sourceID: x.source.ID}, x.ledger, x.policy, true)
						if err != nil {
							t.Fatal(err)
						}
						if got, err := svc.Finalize(x.ctx, x.r); err != nil || got.Status != task.Succeeded {
							t.Fatal(got, err)
						}
						if err := repository.NewAISourceRepo(x.db).SetSourceEnabled(x.ctx, x.source.ID, false); err != nil {
							t.Fatal("提交后未释放来源锁", err)
						}
					case "取消成功竞争":
						var report finalreport.Receipt
						var cancelTask task.Task
						var reportErr, cancelErr error
						var wg sync.WaitGroup
						wg.Add(2)
						go func() { defer wg.Done(); report, reportErr = x.svc.Finalize(x.ctx, x.r) }()
						go func() { defer wg.Done(); cancelTask, cancelErr = x.tasks.Cancel(x.ctx, 1, x.v.ID) }()
						wg.Wait()
						if reportErr != nil || cancelErr != nil || report.Status != task.Succeeded && report.Status != task.Cancelled || cancelTask.Status != task.Running && cancelTask.Status != report.Status {
							t.Fatal("取消/成功竞态无稳定终态", report, reportErr, cancelErr)
						}
						settled := x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'")
						released := x.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'")
						if settled+released != 1 || x.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || report.Status == task.Cancelled && x.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE payload IS NOT NULL") != 0 {
							t.Fatal("取消/成功双终结")
						}
					}
					requirePointAudit(t, x.pointsTestEnv, x.ledger)
				})
			}
		})
	}
}
