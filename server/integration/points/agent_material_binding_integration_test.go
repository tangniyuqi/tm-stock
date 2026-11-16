//go:build integration

package points_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/materialpack"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅组合真实资料服务与真实任务租约；不装配执行器、不调用模型。
type materialBindingFixture struct {
	*pointsTestEnv
	db           *sql.DB
	repo         *repository.AIAgentRepo
	svc          *service.AIAgentMaterialService
	pack         *materialpack.Pack
	identity     repository.AgentMaterialTaskIdentity
	profile      agents.Profile
	request      agents.Request
	approvalPath string
	baseline     []int
}

func materialBindingOpen(t *testing.T, e *pointsTestEnv) *sql.DB {
	t.Helper()
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("解析测试连接失败")
	}
	cfg.DBName, cfg.MultiStatements = e.database, true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("打开隔离测试连接失败")
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("关闭隔离测试连接失败")
		}
	})
	return db
}

func newMaterialBindingFixture(t *testing.T) *materialBindingFixture {
	t.Helper()
	return newMaterialBindingTimedFixture(t, 300000, time.Minute)
}

func newMaterialBindingTimedFixture(t *testing.T, durationMS int64, lease time.Duration) *materialBindingFixture {
	t.Helper()
	e, request, snap := agentMaterialServiceFixture(t)
	db := materialBindingOpen(t, e)
	reportDDL(t, e, "20261015_create_tm_ai_agent_runs.sql")
	reportDDL(t, e, "20261104_add_tm_ai_agent_reference_scope.sql")
	reportDDL(t, e, "20261116_create_tm_ai_agent_material_bindings.sql")
	reportDDL(t, e, "20261116_create_tm_ai_agent_material_bindings.sql")
	id := strings.TrimPrefix(e.database, "tm_ai_local_")
	runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: e.database, Listen: "127.0.0.1:18081", PollIntervalMS: 1000, BatchSize: 1}
	approval := config.AILocalAgentMaterial{Version: 1, InstanceID: id, Revision: "v1", Scope: config.AgentMaterialScope, IssuedAtMS: snap.FetchedAt.UnixMilli(), ExpiresAtMS: request.ValidUntil.UnixMilli(), Members: []int64{1, 2}, Use: config.AILocalAgentMaterialUse{DatasetDigest: strings.Repeat("b", 64), ApprovalRef: "local-eval:synthetic-binding", Purpose: config.AgentMaterialPurpose, Bindings: []config.AILocalAgentMaterialBinding{{SubjectType: request.SubjectType, SubjectID: request.SubjectID, ScopeApproval: "local-eval:synthetic-theme", Evidence: []config.AILocalAgentMaterialEvidence{{SnapshotID: snap.ID, Hash: snap.Hash}}}}}, Limits: config.AILocalAgentMaterialLimits{MaxSources: 1, MaxEvidenceBytes: 4096}}
	raw, err := json.Marshal(approval)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "synthetic-binding-approval.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	source, err := config.NewAILocalAgentMaterialSource(path, runtime)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewAIAgentMaterialService(source, repository.NewAILocalInstanceVerifier(db), repository.NewAIAgentMaterialRepo(db))
	if err != nil {
		t.Fatal(err)
	}
	pack, err := svc.Prepare(e.ctx, 1, "theme", 7)
	if err != nil {
		t.Fatal(err)
	}
	agentMaterialAssertNoWrites(t, e)
	ledger, err := service.NewPointsService(repository.NewPointsRepoWithIsolation(db, sql.LevelReadCommitted))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ledger.Grant(e.ctx, pointGrant(snap.FetchedAt, 100)); err != nil {
		t.Fatal(err)
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepoWithIsolation(db, sql.LevelReadCommitted), ledger)
	if err != nil {
		t.Fatal(err)
	}
	tr := taskRequest(1)
	tr.SubjectID = 7
	budget := taskBudget()
	budget.MaxDurationMS = durationMS
	created, err := tasks.Create(e.ctx, tr, points.Price{Version: "v1", Points: 7}, budget)
	if err != nil {
		t.Fatal(err)
	}
	leased, err := tasks.Claim(e.ctx, 1, created.TaskID, "worker_material_binding_01", lease)
	if err != nil {
		t.Fatal(err)
	}
	identity := repository.AgentMaterialTaskIdentity{TaskID: leased.ID, UserID: leased.UserID, TaskNo: leased.No, Owner: leased.LeaseOwner, Fence: leased.Fence}
	f := &materialBindingFixture{pointsTestEnv: e, db: db, repo: repository.NewAIAgentRepo(db), svc: svc, pack: pack, identity: identity, profile: agents.Profile{Digest: strings.Repeat("a", 64), MaxSubmissions: 8}, request: agents.Request{RequestID: identity.RequestID(), MemberID: 1, SubjectType: "theme", SubjectID: 7, MaterialsDigest: pack.MaterialsDigest()}, approvalPath: path}
	f.baseline = append([]int(nil), taskProbeCounts(e)...)
	return f
}

func (f *materialBindingFixture) reserve(pack *materialpack.Pack, after error) (repository.AgentMaterialBinding, bool, error) {
	var b repository.AgentMaterialBinding
	var fresh bool
	err := f.repo.WithinAgentTx(f.ctx, func(tx agents.Tx) error {
		var err error
		b, fresh, err = tx.(repository.AgentMaterialBindingTx).ReserveMaterialBinding(f.ctx, f.identity, f.profile, f.request, 7, pack)
		if err != nil {
			return err
		}
		return after
	})
	return b, fresh, err
}
func (f *materialBindingFixture) unchanged(t *testing.T) {
	t.Helper()
	if !reflect.DeepEqual(f.baseline, taskProbeCounts(f.pointsTestEnv)) || f.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 || f.queryInt("SELECT avail FROM tm_points_accounts WHERE user_id=1") != 93 {
		t.Fatal("绑定操作污染任务、积分或结算")
	}
}
func (f *materialBindingFixture) counts(t *testing.T, want int) {
	t.Helper()
	for _, table := range []string{"tm_ai_agent_runs", "tm_ai_agent_events", "tm_ai_agent_profiles", "tm_ai_agent_material_bindings"} {
		if f.queryInt("SELECT COUNT(*) FROM "+table) != want {
			t.Fatalf("绑定事务行数错误：%s", table)
		}
	}
	if want > 0 && f.queryInt("SELECT reserved_submissions FROM tm_ai_agent_profiles") != want {
		t.Fatal("配置名额重复预留")
	}
	f.unchanged(t)
}
func materialBindingSignal(t *testing.T, err error) {
	t.Helper()
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != 1644 {
		t.Fatalf("未由不可变/关联触发器明确拒绝：%v", err)
	}
}

func TestIntegrationAIReportAgentMaterialBindingRollback(t *testing.T) {
	f := newMaterialBindingFixture(t)
	fault := errors.New("仅测试：父事务回调失败")
	if _, fresh, err := f.reserve(f.pack, fault); !fresh || !errors.Is(err, fault) {
		t.Fatal("回调故障未传播", err)
	}
	f.counts(t, 0)
	if _, err := f.repo.MaterialBindingByRequest(f.ctx, 7, f.request.RequestID); !errors.Is(err, agents.ErrNotFound) {
		t.Fatal("回滚后仍可读取绑定", err)
	}
	if _, fresh, err := f.reserve(f.pack, nil); err != nil || !fresh {
		t.Fatal("回滚后的合法预留失败", err)
	}
	f.counts(t, 1)
}

func TestIntegrationAIReportAgentMaterialBindingConcurrent(t *testing.T) {
	f := newMaterialBindingFixture(t)
	start := make(chan struct{})
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	var fresh atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-start:
			case <-f.ctx.Done():
				errs <- f.ctx.Err()
				return
			}
			b, n, err := f.reserve(f.pack, nil)
			if n {
				fresh.Add(1)
			}
			if err == nil && (b.TaskID() != f.identity.TaskID || b.ContentDigest() != f.pack.ContentDigest()) {
				err = agents.ErrConflict
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal("同键并发预留失败", err)
		}
	}
	if fresh.Load() != 1 {
		t.Fatal("同键并发未仅预留一次")
	}
	f.counts(t, 1)
}

func TestIntegrationAIReportAgentMaterialBindingRestartAndConflict(t *testing.T) {
	f := newMaterialBindingFixture(t)
	original, _, err := f.reserve(f.pack, nil)
	if err != nil {
		t.Fatal(err)
	}
	var before []byte
	if err = f.db.QueryRowContext(f.ctx, "SELECT manifest FROM tm_ai_agent_material_bindings").Scan(&before); err != nil {
		t.Fatal(err)
	}
	f.mustExec("UPDATE addon_quant_theme SET name='仅测试：变更名称' WHERE id=7")
	changed, err := f.svc.Prepare(f.ctx, 1, "theme", 7)
	if err != nil {
		t.Fatal(err)
	}
	if changed.MaterialsDigest() != f.pack.MaterialsDigest() || changed.ContentDigest() == f.pack.ContentDigest() {
		t.Fatal("名称变化未保持声明摘要并改变完整摘要")
	}
	if _, _, err = f.reserve(changed, nil); !errors.Is(err, agents.ErrConflict) {
		t.Fatal("同声明不同内容覆盖原绑定", err)
	}
	restarted := repository.NewAIAgentRepo(materialBindingOpen(t, f.pointsTestEnv))
	restored, err := restarted.MaterialBindingByRequest(f.ctx, 7, f.request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := restored.Pack()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Record() != original.Record() || restored.UserID() != 1 || restored.TaskNo() != f.identity.TaskNo || pack.ContentDigest() != f.pack.ContentDigest() || pack.MaterialsDigest() != f.pack.MaterialsDigest() || !reflect.DeepEqual(pack.Snapshots(), f.pack.Snapshots()) || pack.Scope() != f.pack.Scope() {
		t.Fatal("使用新连接的仓储未恢复原包双摘要")
	}
	if err = f.svc.Recheck(f.ctx, pack); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("历史恢复被当作当前授权", err)
	}
	if err = os.Remove(f.approvalPath); err != nil {
		t.Fatal(err)
	}
	if _, err = restored.Pack(); err != nil {
		t.Fatal("撤回当前批准破坏历史身份读取", err)
	}
	if err = f.svc.Recheck(f.ctx, pack); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("批准撤回后历史包仍获授权", err)
	}
	var after []byte
	if err = f.db.QueryRowContext(f.ctx, "SELECT manifest FROM tm_ai_agent_material_bindings").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("冲突或重启读取替换原manifest")
	}
	if _, err = restarted.MaterialBindingByRequest(f.ctx, 8, f.request.RequestID); !errors.Is(err, agents.ErrNotFound) {
		t.Fatal("其他操作人读取绑定", err)
	}
	f.counts(t, 1)
}

func TestIntegrationAIReportAgentMaterialBindingInvalidIdentity(t *testing.T) {
	for _, mode := range []string{"错任务", "错会员", "错对象", "错请求", "错owner", "旧fence"} {
		t.Run(mode, func(t *testing.T) {
			f := newMaterialBindingFixture(t)
			switch mode {
			case "错任务":
				f.identity.TaskID++
			case "错会员":
				f.identity.UserID = 2
			case "错对象":
				f.request.SubjectID++
			case "错请求":
				f.request.RequestID = "wrong_binding_request_01"
			case "错owner":
				f.identity.Owner = "worker_material_other_01"
			case "旧fence":
				f.identity.Fence++
			}
			if _, fresh, err := f.reserve(f.pack, nil); err == nil || fresh {
				t.Fatal("错误身份或租约通过预留")
			}
			f.counts(t, 0)
		})
	}
}

func TestIntegrationAIReportAgentMaterialBindingReplayNotAuthorization(t *testing.T) {
	f := newMaterialBindingFixture(t)
	first, _, err := f.reserve(f.pack, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.profile.MaxSubmissions++
	if _, fresh, err := f.reserve(f.pack, nil); !errors.Is(err, agents.ErrConflict) || fresh {
		t.Fatal("同摘要不同限额没有冲突", err)
	}
	f.profile.MaxSubmissions--
	ledger, err := service.NewPointsService(repository.NewPointsRepoWithIsolation(f.db, sql.LevelReadCommitted))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepoWithIsolation(f.db, sql.LevelReadCommitted), ledger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tasks.Cancel(f.ctx, 1, f.identity.TaskID); err != nil {
		t.Fatal(err)
	}
	// 取消事件是本测试主动产生的基线变化，不是绑定重放的副作用。
	f.baseline = taskProbeCounts(f.pointsTestEnv)
	f.identity.Fence++
	replay, fresh, err := f.reserve(f.pack, nil)
	if err != nil || fresh || replay.Record() != first.Record() {
		t.Fatal("旧绑定重放被当成新发送权或原记录丢失", err)
	}
	f.counts(t, 1)
}

func TestIntegrationAIReportAgentMaterialBindingSQLIdentity(t *testing.T) {
	for _, mode := range []string{"错任务", "错会员", "错对象", "错请求", "超256KiB", "租约过期", "截止过期", "任务取消", "步骤已推进"} {
		t.Run(mode, func(t *testing.T) {
			var f *materialBindingFixture
			switch mode {
			case "租约过期":
				f = newMaterialBindingTimedFixture(t, 300000, time.Second)
			case "截止过期":
				f = newMaterialBindingTimedFixture(t, 2000, time.Minute)
			default:
				f = newMaterialBindingFixture(t)
			}
			materialBindingTaskBoundary(t, f, mode)
			raw, err := materialpack.MarshalManifest(f.pack)
			if err != nil {
				t.Fatal(err)
			}
			req := f.request
			taskID := f.identity.TaskID
			switch mode {
			case "错任务":
				taskID++
			case "错会员":
				req.MemberID = 2
			case "错对象":
				req.SubjectID++
			case "错请求":
				req.RequestID = "wrong_sql_request_01"
			case "超256KiB":
				raw = append(raw, bytes.Repeat([]byte(" "), 262145-len(raw))...)
			}
			// 直接SQL绕过Go身份检查，但保留合法run与初始事件，明确验证绑定触发器。
			tx, err := f.db.BeginTx(f.ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err = tx.ExecContext(f.ctx, "INSERT INTO tm_ai_agent_profiles(profile_digest,max_submissions,reserved_submissions) VALUES(?,8,1)", f.profile.Digest); err != nil {
				t.Fatal(err)
			}
			result, err := tx.ExecContext(f.ctx, "INSERT INTO tm_ai_agent_runs(request_id,actor_id,symbol,member_id,subject_type,subject_id,materials_digest,profile_digest) VALUES(?,7,'',?,?,?,?,?)", req.RequestID, req.MemberID, req.SubjectType, req.SubjectID, req.MaterialsDigest, f.profile.Digest)
			if err != nil {
				t.Fatal(err)
			}
			runID, err := result.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			record := agents.Record{ID: runID, ActorID: 7, Request: req, ProfileDigest: f.profile.Digest, State: agents.Submitting}
			audit, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(f.ctx, "INSERT INTO tm_ai_agent_events(run_id,seq,event,record_json) VALUES(?,1,'submit_reserved',?)", runID, audit); err != nil {
				t.Fatal(err)
			}
			_, err = tx.ExecContext(f.ctx, "INSERT INTO tm_ai_agent_material_bindings(task_id,user_id,run_id,materials_digest,content_digest,manifest,created_at,updated_at) VALUES(?,?,?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))", taskID, 1, runID, f.pack.MaterialsDigest(), f.pack.ContentDigest(), raw)
			if mode == "超256KiB" {
				var me *mysql.MySQLError
				if !errors.As(err, &me) || me.Number != 3819 {
					t.Fatalf("SQL未以固定256KiB约束拒绝：%v", err)
				}
			} else {
				materialBindingSignal(t, err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			f.counts(t, 0)
		})
	}
}

// 所有任务状态由原服务及真实时钟产生；不删触发器，不直接篡改任务身份/截止时间。
func materialBindingTaskBoundary(t *testing.T, f *materialBindingFixture, mode string) {
	t.Helper()
	repo := repository.NewAITaskRepoWithIsolation(f.db, sql.LevelReadCommitted)
	ledger, err := service.NewPointsService(repository.NewPointsRepoWithIsolation(f.db, sql.LevelReadCommitted))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := service.NewAITaskService(repo, ledger)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "租约过期", "截止过期":
		v, err := tasks.Inspect(f.ctx, 1, f.identity.TaskID)
		if err != nil || v.LeaseUntil == nil {
			t.Fatal("读取真实租约失败", err)
		}
		until := *v.LeaseUntil
		if mode == "截止过期" {
			until = v.Deadline
		}
		waitTaskTime(t, f.pointsTestEnv, until)
		var expired, deadlineExpired bool
		if err = f.db.QueryRowContext(f.ctx, "SELECT lease_until<=UTC_TIMESTAMP(3),deadline_at<=UTC_TIMESTAMP(3) FROM tm_ai_tasks WHERE id=?", v.ID).Scan(&expired, &deadlineExpired); err != nil {
			t.Fatal(err)
		}
		if !expired || deadlineExpired != (mode == "截止过期") {
			t.Fatal("真实数据库时钟未到预期边界")
		}
	// lease_until<=deadline_at 是原任务不变量，截止过期必然也租约过期。
	case "任务取消":
		if _, err = tasks.Cancel(f.ctx, 1, f.identity.TaskID); err != nil {
			t.Fatal(err)
		}
		if f.queryInt("SELECT cancel_req FROM tm_ai_tasks WHERE id=?", f.identity.TaskID) != 1 {
			t.Fatal("取消未落库")
		}
	case "步骤已推进":
		budgets, err := service.NewAITaskBudgetService(repo)
		if err != nil {
			t.Fatal(err)
		}
		r := task.StageRequest{UserID: 1, TaskID: f.identity.TaskID, Fence: f.identity.Fence, Owner: f.identity.Owner, OperationKey: "binding_prior_stage_01", Stage: "extract", InputDigest: strings.Repeat("d", 64), StepNo: 1}
		if _, err = budgets.Reserve(f.ctx, task.ReserveRequest{UserID: 1, TaskID: r.TaskID, Fence: r.Fence, Owner: r.Owner, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 100}}); err != nil {
			t.Fatal(err)
		}
		stages, err := service.NewAITaskStageService(repo, testStagePolicies())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = stages.Capture(f.ctx, r, testStageBody); err != nil {
			t.Fatal(err)
		}
		if _, err = stages.Complete(f.ctx, r); err != nil {
			t.Fatal(err)
		}
		if f.queryInt("SELECT current_step FROM tm_ai_tasks WHERE id=?", r.TaskID) != 1 {
			t.Fatal("合法阶段未推进")
		}
	}
	// 合法取消/阶段操作的事件和预算变化不归因于随后绑定写入。
	f.baseline = taskProbeCounts(f.pointsTestEnv)
}

func TestIntegrationAIReportAgentMaterialBindingNoHistoricalBackfill(t *testing.T) {
	f := newMaterialBindingFixture(t)
	var original agents.Record
	if err := f.repo.WithinAgentTx(f.ctx, func(tx agents.Tx) error {
		var err error
		original, _, err = tx.Reserve(f.ctx, f.profile, f.request, 7)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var before []byte
	if err := f.db.QueryRowContext(f.ctx, "SELECT record_json FROM tm_ai_agent_events WHERE run_id=? AND seq=1", original.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := f.reserve(f.pack, nil); !errors.Is(err, agents.ErrConflict) || fresh {
		t.Fatal("无历史manifest的reference被今日资料补绑", err)
	}
	got, err := f.repo.ByRequest(f.ctx, f.request.RequestID)
	if err != nil || got != original {
		t.Fatal("补绑拒绝改写原run", err)
	}
	var after []byte
	if err = f.db.QueryRowContext(f.ctx, "SELECT record_json FROM tm_ai_agent_events WHERE run_id=? AND seq=1", original.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("补绑拒绝改写原event")
	}
	for _, table := range []string{"tm_ai_agent_runs", "tm_ai_agent_events", "tm_ai_agent_profiles"} {
		if f.queryInt("SELECT COUNT(*) FROM "+table) != 1 {
			t.Fatal("补绑拒绝改变原记录数", table)
		}
	}
	if f.queryInt("SELECT reserved_submissions FROM tm_ai_agent_profiles") != 1 || f.queryInt("SELECT max_submissions FROM tm_ai_agent_profiles") != f.profile.MaxSubmissions || f.queryInt("SELECT COUNT(*) FROM tm_ai_agent_material_bindings") != 0 {
		t.Fatal("补绑拒绝污染名额或创建绑定")
	}
	if _, err = f.repo.MaterialBindingByRequest(f.ctx, 7, f.request.RequestID); !errors.Is(err, agents.ErrNotFound) {
		t.Fatal("补绑拒绝后仍可读取绑定", err)
	}
	f.unchanged(t)
}

func TestIntegrationAIReportAgentMaterialBindingSQLGuards(t *testing.T) {
	f := newMaterialBindingFixture(t)
	if _, _, err := f.reserve(f.pack, nil); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"UPDATE tm_ai_agent_material_bindings SET manifest=manifest", "UPDATE tm_ai_agent_material_bindings SET content_digest=REPEAT('c',64)", "DELETE FROM tm_ai_agent_material_bindings"} {
		_, err := f.db.ExecContext(f.ctx, q)
		materialBindingSignal(t, err)
	}
	f.counts(t, 1)
}

func TestIntegrationAIReportAgentMaterialBindingRequestGolden(t *testing.T) {
	f := newMaterialBindingFixture(t)
	var sqlID string
	err := f.db.QueryRowContext(f.ctx, `SELECT SHA2(CONVERT(CONCAT('["task_agent_request_v1",',CAST(user_id AS CHAR),',',CAST(id AS CHAR),',"',task_no,'","agent_call:1"]') USING ascii),256) FROM tm_ai_tasks WHERE id=?`, f.identity.TaskID).Scan(&sqlID)
	if err != nil {
		t.Fatal(err)
	}
	plan := agenttask.Plan{UserID: f.identity.UserID, TaskID: f.identity.TaskID, TaskNo: f.identity.TaskNo}
	if sqlID != f.identity.RequestID() || sqlID != plan.RequestID() {
		t.Fatal("Go与SQL原请求编码不同")
	}
	fixed := repository.AgentMaterialTaskIdentity{UserID: 1, TaskID: 2, TaskNo: strings.Repeat("a", 32)}
	var fixedSQL string
	if err = f.db.QueryRowContext(f.ctx, `SELECT SHA2('["task_agent_request_v1",1,2,"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","agent_call:1"]',256)`).Scan(&fixedSQL); err != nil {
		t.Fatal(err)
	}
	const golden = "7e33870211dab469061364b9ec849cd1c109dc0271395cecfcc9dc3ec734b1ff"
	if fixed.RequestID() != golden || fixedSQL != golden {
		t.Fatal("固定向量请求摘要不同")
	}
	f.counts(t, 0)
}

func TestIntegrationAIReportAgentMaterialBindingLegacyAndPermission(t *testing.T) {
	f := newMaterialBindingFixture(t)
	old := agents.Request{RequestID: "legacy_material_binding_01", Symbol: "600001"}
	var saved agents.Record
	err := f.repo.WithinAgentTx(f.ctx, func(tx agents.Tx) error {
		var err error
		saved, _, err = tx.Reserve(f.ctx, f.profile, old, 7)
		return err
	})
	if err != nil {
		t.Fatal("旧symbol预留路径失败", err)
	}
	got, err := f.repo.ByRequest(f.ctx, old.RequestID)
	if err != nil || got != saved || got.Request.IsReference() || f.queryInt("SELECT COUNT(*) FROM tm_ai_agent_material_bindings") != 0 {
		t.Fatal("旧symbol路径被强制绑定资料", err)
	}
	if _, _, err = f.reserve(f.pack, nil); err != nil {
		t.Fatal(err)
	}
	reader := agentMaterialServiceReader(t, f.pointsTestEnv)
	_, err = reader.ExecContext(f.ctx, "SELECT * FROM tm_ai_agent_material_bindings LIMIT 0")
	agentMaterialAssertPermissionDenied(t, err)
	if b, err := repository.NewAIAgentRepo(reader).MaterialBindingByRequest(f.ctx, 7, f.request.RequestID); err == nil || b.ContentDigest() != "" {
		t.Fatal("缺绑定权限仍返回原包")
	}
	f.unchanged(t)
}
