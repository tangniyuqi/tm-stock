//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 一次测试进程只读一份迁移副本，防止多轮运行时混入工作区正在编辑的新 DDL。
// 修改代码/迁移后仍须重新启动整套验证，旧进程通过不证明新代码通过。
var persistentTaskMigration = sync.OnceValues(func() (string, error) {
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261011_create_tm_ai_tasks.sql"))
	if err != nil {
		return "", err
	}
	script := strings.ReplaceAll(string(data), "DELIMITER $$", "")
	script = strings.ReplaceAll(script, "DELIMITER ;", "")
	script = strings.ReplaceAll(script, "END$$", "END;")
	return script, nil
})

func persistentTaskFixture(t *testing.T, level sql.IsolationLevel) (*pointsTestEnv, *sql.DB, *repository.AITaskRepo, *service.AITaskService, *service.PointsService, time.Time) {
	t.Helper()
	e, db, _, ledger, now := pointsFixture(t, level)
	script, err := persistentTaskMigration()
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(script)
	e.mustExec(script)
	budgetDDL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261015_create_tm_ai_task_budgets.sql"))
	if err != nil {
		t.Fatal(err)
	}
	budgetSQL := strings.ReplaceAll(string(budgetDDL), "DELIMITER $$", "")
	budgetSQL = strings.ReplaceAll(budgetSQL, "DELIMITER ;", "")
	budgetSQL = strings.ReplaceAll(budgetSQL, "END$$", "END;")
	e.mustExec(budgetSQL)
	e.mustExec(budgetSQL)
	stageDDL, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261015_create_tm_ai_task_steps.sql"))
	if err != nil {
		t.Fatal(err)
	}
	stageSQL := strings.ReplaceAll(string(stageDDL), "DELIMITER $$", "")
	stageSQL = strings.ReplaceAll(stageSQL, "DELIMITER ;", "")
	stageSQL = strings.ReplaceAll(stageSQL, "END$$", "END;")
	e.mustExec(stageSQL)
	e.mustExec(stageSQL)
	repo := repository.NewAITaskRepoWithIsolation(db, level)
	s, err := service.NewAITaskService(repo, ledger)
	if err != nil {
		t.Fatal(err)
	}
	return e, db, repo, s, ledger, now
}
func taskRequest(number int) task.CreateRequest {
	return task.CreateRequest{UserID: 1, Key: fmt.Sprintf("test_ai_task_%06d", number), AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}
}
func taskBudget() task.Budget {
	return task.Budget{MaxAttempts: 3, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 30000}
}
func waitTaskTime(t *testing.T, e *pointsTestEnv, until time.Time) {
	t.Helper()
	timer := time.NewTimer(max(time.Until(until)+30*time.Millisecond, 0))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-e.ctx.Done():
		t.Fatal(e.ctx.Err())
	}
}

func TestIntegrationAITaskRoundTrip(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, repo, s, ledger, now := persistentTaskFixture(t, level)
			if _, err := ledger.Grant(e.ctx, pointGrant(now, 20)); err != nil {
				t.Fatal(err)
			}
			r := taskRequest(1)
			price := points.Price{Version: "v1", Points: 7}
			created, err := s.Create(e.ctx, r, price, taskBudget())
			if err != nil {
				t.Fatal(err)
			}
			if e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 13 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks") != 1 {
				t.Fatal("任务与冻结不一致")
			}
			requirePointAudit(t, e, ledger)
			if again, err := s.Create(e.ctx, r, points.Price{}, task.Budget{}); err != nil || again != created {
				t.Fatal("变更配置后的创建重放改变原结果", again, err)
			}
			r.SubjectID++
			if _, err := s.Create(e.ctx, r, price, taskBudget()); !errors.Is(err, points.ErrIdempotency) {
				t.Fatal(err)
			}
			r.SubjectID--
			owner := "worker_integration_001"
			v, err := s.Claim(e.ctx, 1, created.TaskID, owner, time.Second)
			if err != nil || v.Fence != 1 || v.Attempt != 1 {
				t.Fatal(v, err)
			}
			if _, err := s.Claim(e.ctx, 1, v.ID, "worker_integration_002", time.Second); !errors.Is(err, task.ErrNotReady) {
				t.Fatal(err)
			}
			v, err = s.Heartbeat(e.ctx, 1, v.ID, owner, 1, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			waitTaskTime(t, e, *v.LeaseUntil)
			v, err = s.Claim(e.ctx, 1, v.ID, "worker_integration_002", time.Second)
			if err != nil || v.Fence != 2 || v.Attempt != 2 {
				t.Fatal("租约到期后接管失败", v, err)
			}
			if _, err := s.Heartbeat(e.ctx, 1, v.ID, owner, 1, time.Second); !errors.Is(err, task.ErrLease) {
				t.Fatal("旧 worker 心跳写入", err)
			}
			if _, err := s.FailOrRetry(e.ctx, 1, v.ID, owner, 1, "internal_failure", 0); !errors.Is(err, task.ErrLease) {
				t.Fatal("旧 worker 终结任务", err)
			}
			owner = "worker_integration_002"
			v, err = s.FailOrRetry(e.ctx, 1, v.ID, owner, 2, "provider_unavailable", time.Second)
			if err != nil || v.Status != task.Queued {
				t.Fatal(v, err)
			}
			if _, err := s.Claim(e.ctx, 1, v.ID, owner, time.Second); !errors.Is(err, task.ErrNotReady) {
				t.Fatal("未到退避时点就执行", err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='freeze'") != 1 {
				t.Fatal("重试重复冻结")
			}
			waitTaskTime(t, e, v.NotBefore)
			v, err = s.Claim(e.ctx, 1, v.ID, owner, time.Minute)
			if err != nil || v.Fence != 3 || v.Attempt != 3 {
				t.Fatal(v, err)
			}
			v, err = s.Cancel(e.ctx, 1, v.ID)
			if err != nil || v.Status != task.Running || !v.CancelRequested {
				t.Fatal(v, err)
			}
			if _, err := s.Heartbeat(e.ctx, 1, v.ID, owner, 3, time.Minute); !errors.Is(err, task.ErrCancelled) {
				t.Fatal(err)
			}
			v, err = s.FailOrRetry(e.ctx, 1, v.ID, owner, 3, "provider_unavailable", time.Second)
			if err != nil || v.Status != task.Cancelled {
				t.Fatal(v, err)
			}
			count := e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events")
			if again, err := s.Cancel(e.ctx, 1, v.ID); err != nil || !reflect.DeepEqual(again, v) {
				t.Fatal("取消终态不稳定", again, err)
			}
			if again, err := s.Create(e.ctx, r, points.Price{}, task.Budget{}); err != nil || again != created {
				t.Fatal("终态后创建重放变化", again, err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != count || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("取消重放产生事件或漏释放")
			}
			g := pointGrant(now, 20)
			g.UserID = 2
			g.Key = "test_task_user2_grant"
			g.BizID = "task:user2:grant"
			if _, err := ledger.Grant(e.ctx, g); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Cancel(e.ctx, 2, v.ID); !errors.Is(err, points.ErrNotFound) {
				t.Fatal("可终结其他用户任务", err)
			}
			for _, query := range []string{"UPDATE tm_ai_tasks SET status=status WHERE id=?", "DELETE FROM tm_ai_tasks WHERE id=?", "UPDATE tm_ai_task_events SET seq=seq WHERE task_id=?", "DELETE FROM tm_ai_task_events WHERE task_id=?"} {
				if _, err := e.conn.ExecContext(e.ctx, query, v.ID); err == nil {
					t.Fatal("终态或事件可改写", query)
				}
			}
			b := taskBudget()
			b.MaxDurationMS = 1000
			topicRequest := taskRequest(2)
			topicRequest.SubjectType = task.Topic
			expiring, err := s.Create(e.ctx, topicRequest, price, b)
			if err != nil {
				t.Fatal(err)
			}
			candidates, err := repo.ClaimCandidates(e.ctx, 100)
			if err != nil || len(candidates) != 1 || candidates[0].ID != expiring.TaskID {
				t.Fatal(candidates, err)
			}
			waitTaskTime(t, e, candidates[0].Deadline)
			v, err = s.Expire(e.ctx, 1, expiring.TaskID)
			if err != nil || v.Status != task.Expired {
				t.Fatal(v, err)
			}
			if e.queryInt("SELECT state FROM tm_points_holds WHERE id=?", expiring.HoldID) != points.Expired {
				t.Fatal("过期任务冻结未按过期释放")
			}
			requirePointAudit(t, e, ledger)
			if rows, err := repo.ClaimCandidates(e.ctx, 100); err != nil || len(rows) != 0 {
				t.Fatal(rows, err)
			}
		})
	}
}

type abortTaskStore struct {
	db     *sql.DB
	level  sql.IsolationLevel
	after  error
	commit bool
}

type borrowedTaskStore struct{ tx *sql.Tx }

func (s borrowedTaskStore) WithinTaskTx(_ context.Context, fn func(task.Tx) error) error {
	return fn(repository.AITaskTx(s.tx))
}

func (s abortTaskStore) WithinTaskTx(ctx context.Context, fn func(task.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: s.level})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(repository.AITaskTx(tx)); err != nil {
		return err
	}
	if s.commit {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return s.after
}

func TestIntegrationAITaskRollbackAndGuards(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, db, _, s, ledger, now := persistentTaskFixture(t, level)
			if _, err := ledger.Grant(e.ctx, pointGrant(now, 20)); err != nil {
				t.Fatal(err)
			}
			fault := errors.New("测试父事务失败")
			parent, err := service.NewAITaskService(abortTaskStore{db: db, level: level, after: fault}, ledger)
			if err != nil {
				t.Fatal(err)
			}
			price := points.Price{Version: "v1", Points: 7}
			r := taskRequest(1)
			if v, err := parent.Create(e.ctx, r, price, taskBudget()); !errors.Is(err, fault) || v != (task.Created{}) {
				t.Fatal(v, err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 0 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 {
				t.Fatal("父事务创建失败留下冻结/任务")
			}
			if _, err := s.Create(e.ctx, r, points.Price{Version: "v2", Points: 7}, taskBudget()); !errors.Is(err, points.ErrPriceChanged) {
				t.Fatal(err)
			}
			e.mustExec("CREATE TRIGGER tm_test_task_event_fail BEFORE INSERT ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test task event failure'")
			if _, err := s.Create(e.ctx, r, price, taskBudget()); err == nil {
				t.Fatal("事件写入失败仍成功")
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 0 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 20 {
				t.Fatal("创建事件失败未整体回滚")
			}
			e.mustExec("DROP TRIGGER tm_test_task_event_fail")
			// 固定父事务旧 RR 快照，再由其他连接提交任务；创建重放必须当前读，不能重复冻结。
			oldSnapshot, err := db.BeginTx(e.ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
			if err != nil {
				t.Fatal(err)
			}
			defer oldSnapshot.Rollback()
			var initial int
			if err := oldSnapshot.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM tm_ai_tasks").Scan(&initial); err != nil || initial != 0 {
				t.Fatal(initial, err)
			}
			unknown, err := service.NewAITaskService(abortTaskStore{db: db, level: level, after: fault, commit: true}, ledger)
			if err != nil {
				t.Fatal(err)
			}
			if v, err := unknown.Create(e.ctx, r, price, taskBudget()); !errors.Is(err, fault) || v != (task.Created{}) {
				t.Fatal(v, err)
			}
			v, err := s.Create(e.ctx, r, price, taskBudget())
			if err != nil || v.TaskID <= 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 1 {
				t.Fatal("提交未知结果后原键核查重复冻结", v, err)
			}
			borrowed, err := service.NewAITaskService(borrowedTaskStore{tx: oldSnapshot}, ledger)
			if err != nil {
				t.Fatal(err)
			}
			if again, err := borrowed.Create(e.ctx, r, price, taskBudget()); err != nil || again != v {
				t.Fatal("父事务旧快照漏读既有任务", again, err)
			}
			if err := oldSnapshot.Commit(); err != nil {
				t.Fatal(err)
			}
			if _, err := parent.Cancel(e.ctx, 1, v.TaskID); !errors.Is(err, fault) {
				t.Fatal(err)
			}
			if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", v.TaskID) != task.Queued || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
				t.Fatal("父事务取消失败未回滚")
			}
			e.mustExec("CREATE TRIGGER tm_test_task_event_fail BEFORE INSERT ON tm_ai_task_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test task event failure'")
			if _, err := s.Cancel(e.ctx, 1, v.TaskID); err == nil {
				t.Fatal("终态事件失败仍成功")
			}
			if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id=?", v.TaskID) != task.Queued || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'") != 0 {
				t.Fatal("终态事件失败留下已释放积分")
			}
			e.mustExec("DROP TRIGGER tm_test_task_event_fail")
			if _, err := s.Cancel(e.ctx, 1, v.TaskID); err != nil {
				t.Fatal(err)
			}
			// 每个破坏探针都先构造同形正例并回滚，防止唯一键或基础故障冒充 CHECK/FK 拒绝。
			for _, mode := range []string{"budget", "price", "state", "owner", "event"} {
				for _, negative := range []bool{false, true} {
					tx, err := db.BeginTx(e.ctx, &sql.TxOptions{Isolation: level})
					if err != nil {
						t.Fatal(err)
					}
					ptx := repository.PointsTx(tx)
					freeze, err := ledger.FreezeInTx(e.ctx, ptx, points.FreezeRequest{UserID: 1, Key: "test_task_schema_probe", BizType: "ai_task", BizID: "schema:probe", PriceVersion: "v1", ExpiresAt: now.Add(time.Hour)}, price)
					if err != nil {
						tx.Rollback()
						t.Fatal(err)
					}
					next := task.Task{UserID: 1, No: strings.Repeat("b", 32), Key: "test_task_schema_001", Hash: strings.Repeat("b", 64), AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, HoldID: freeze.HoldID, PriceVersion: "v1", PricePoints: 7, Budget: taskBudget(), Status: task.Queued, NotBefore: now, Deadline: now.Add(time.Hour), EventSeq: 1}
					if negative {
						switch mode {
						case "budget":
							next.Budget.MaxCalls = 0
						case "price":
							next.PricePoints = 0
						case "state":
							next.Status = task.Running
						case "owner":
							next.UserID = 2
						}
					}
					var taskID int64
					taskID, err = repository.AITaskTx(tx).InsertTask(e.ctx, next)
					if mode == "event" && err == nil {
						event := task.Event{TaskID: taskID, Seq: 1, Type: "created", Status: task.Queued}
						if negative {
							event.Status = task.Succeeded
						}
						err = repository.AITaskTx(tx).InsertEvent(e.ctx, event)
					}
					if rollback := tx.Rollback(); rollback != nil {
						t.Fatal(rollback)
					}
					if !negative && err != nil {
						t.Fatal("任务结构正例失败", mode, err)
					}
					if negative {
						var me *mysqldrv.MySQLError
						want := uint16(3819)
						if mode == "owner" {
							want = 1452
						}
						if !errors.As(err, &me) || me.Number != want {
							t.Fatal("未由目标约束拒绝", mode, err)
						}
					}
				}
			}
			requirePointAudit(t, e, ledger)
		})
	}
}

func TestIntegrationAITaskConcurrency(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, s, ledger, now := persistentTaskFixture(t, level)
			if _, err := ledger.Grant(e.ctx, pointGrant(now, 100)); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			created := make(chan task.Created, 40)
			errs := make(chan error, 40)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					v, err := s.Create(e.ctx, taskRequest(1), points.Price{Version: "v1", Points: 7}, taskBudget())
					created <- v
					errs <- err
				}()
			}
			wg.Wait()
			close(created)
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			var first task.Created
			for v := range created {
				if first.TaskID == 0 {
					first = v
				}
				if v != first {
					t.Fatal("并发重放返回不同收据")
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 1 {
				t.Fatal("同键并发重复任务/冻结")
			}
			claimed := make(chan task.Task, 40)
			errs = make(chan error, 40)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					v, err := s.Claim(e.ctx, 1, first.TaskID, fmt.Sprintf("test_worker_%06d", i), time.Minute)
					claimed <- v
					errs <- err
				}(i)
			}
			wg.Wait()
			close(claimed)
			close(errs)
			wins := 0
			for err := range errs {
				if err == nil {
					wins++
				} else if !errors.Is(err, task.ErrNotReady) {
					t.Fatal(err)
				}
			}
			var winner task.Task
			for v := range claimed {
				if v.ID > 0 {
					winner = v
				}
			}
			if wins != 1 || winner.Fence != 1 || winner.Attempt != 1 {
				t.Fatal("重复抢占", wins, winner)
			}
			errs = make(chan error, 40)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); _, err := s.Cancel(e.ctx, 1, first.TaskID); errs <- err }()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_task_events") != 3 {
				t.Fatal("并发取消重复进度事件")
			}
			errs = make(chan error, 40)
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := s.FailOrRetry(e.ctx, 1, winner.ID, winner.LeaseOwner, winner.Fence, "internal_failure", 0)
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			wins = 0
			for err := range errs {
				if err == nil {
					wins++
				} else if !errors.Is(err, task.ErrLease) {
					t.Fatal(err)
				}
			}
			if wins != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'") != 1 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 100 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 {
				t.Fatal("并发取消确认重复释放", wins)
			}
			requirePointAudit(t, e, ledger)
		})
	}
}

func TestIntegrationAITaskConcurrentCreates(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, _, _, s, ledger, now := persistentTaskFixture(t, level)
			if _, err := ledger.Grant(e.ctx, pointGrant(now, 100)); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			results := make(chan error, 200)
			for i := 0; i < 200; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, err := s.Create(e.ctx, taskRequest(i), points.Price{Version: "v1", Points: 7}, taskBudget())
					results <- err
				}(i)
			}
			wg.Wait()
			close(results)
			wins := 0
			for err := range results {
				if err == nil {
					wins++
				} else if !errors.Is(err, points.ErrInsufficient) {
					t.Fatal(err)
				}
			}
			if wins != 14 || e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks") != 14 || e.queryInt("SELECT COUNT(*) FROM tm_points_holds") != 14 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 98 || e.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 2 {
				t.Fatal("任务创建并发超扣或留下孤立冻结", wins)
			}
			requirePointAudit(t, e, ledger)
		})
	}
}
