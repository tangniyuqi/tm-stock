//go:build integration

package points_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 实际命令、真实进程终止与自然租约；不改变旧围栏、时间或回执以制造恢复成功。
func TestIntegrationAIReportLocalWorkerRecovery(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	binary := filepath.Join(t.TempDir(), "local-ai-worker")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	buildCtx, stop := context.WithTimeout(context.Background(), 5*time.Minute)
	defer stop()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/ai-worker")
	build.Dir = filepath.Join("..", "..")
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("构建实际worker失败")
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					for _, mode := range []string{"final", "agent", "model", "queued"} {
						t.Run(mode, func(t *testing.T) { verifyLocalWorkerRecovery(t, binary, level, kind, mode) })
					}
				})
			}
		})
	}
}

func verifyLocalWorkerRecovery(t *testing.T, binary string, level sql.IsolationLevel, kind int, mode string) {
	t.Helper()
	workflow, analysis := "local", "light"
	if kind == 2 {
		workflow, analysis = "local_deep", "deep"
	}
	if mode == "agent" {
		workflow += "_running"
	}
	if mode == "model" {
		workflow += "_model_wait"
	}
	f := newLocalPublicationFixture(t, level, kind, workflow)
	e := f.env
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	e.pointsTestEnv.ctx = ctx
	dsn := localWorkerDSN(t, e)
	dir := t.TempDir()
	runtimeFile, executionFile := filepath.Join(dir, "runtime.json"), filepath.Join(dir, "execution.json")
	write := func(path string, v any) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal("写隔离配置失败")
		}
	}
	write(runtimeFile, f.runtime)
	cc, mc := e.agentConfig, workflowModelConfig(e.server.URL())
	at := time.Now().UnixMilli()
	write(executionFile, config.AILocalExecution{Version: 1, InstanceID: f.runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: at - 1000, ExpiresAtMS: f.file.ExpiresAtMS,
		Model:   config.AILocalExecutionModel{Name: mc.Name, BaseURL: mc.BaseURL, Model: mc.Model, TimeoutMS: mc.TimeoutMS},
		Agent:   config.AILocalExecutionAgent{BaseURL: cc.BaseURL, ApprovalRef: cc.ApprovalRef, LicenseRef: cc.LicenseRef, QuickModel: cc.QuickModel, DeepModel: cc.DeepModel, TimeoutMS: int(cc.Timeout / time.Millisecond), MaxSubmissions: cc.MaxSubmissions},
		LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 30000, AgentPollMS: 50})
	resume := func(uid, id int64, want int, marker string) {
		t.Helper()
		callCtx, stop := context.WithTimeout(e.ctx, 40*time.Second)
		defer stop()
		cmd := exec.CommandContext(callCtx, binary, "resume-local", "-config", runtimeFile, "-plans", f.path, "-execution", executionFile, "-user", strconv.FormatInt(uid, 10), "-task", strconv.FormatInt(id, 10))
		cmd.Env = localWorkerEnvironment(dsn, cc.Bearer)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			var exited *exec.ExitError
			if !errors.As(err, &exited) {
				t.Fatal("恢复进程无法运行")
			}
			code = exited.ExitCode()
		}
		if callCtx.Err() != nil || code != want || !strings.Contains(stdout.String(), marker) {
			t.Fatal("实际恢复回执不符", code, stdout.String())
		}
		if strings.Contains(stdout.String()+stderr.String(), cc.Bearer) || strings.Contains(stdout.String()+stderr.String(), dsn) {
			t.Fatal("恢复输出泄漏凭据")
		}
	}
	if mode == "queued" {
		created, err := f.creates.Create(e.ctx, 1, "local_recovery_queued_key", dto.AITaskCreateInput{AnalysisType: analysis, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "local-v1"})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			resume(1, created.TaskID, 1, "阶段4")
		}
		// 既有取消终态可以核对；退出0不能被解释成成功报告。
		resume(1, e.v.ID, 0, "LOCAL_REPORT_RESUME_TERMINAL")
		v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
		if err != nil || v.Status != task.Queued || v.Fence != 0 || v.Attempt != 0 || e.submits.Load() != 0 || e.server.Count() != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_reservations`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != int(created.PricePoints) {
			t.Fatal("恢复命令领取了排队任务或改写终态", err)
		}
		requirePointAudit(t, e.pointsTestEnv, e.ledger)
		return
	}
	// 先就绪再冻结最终报告的空索引区间，启动自检不受测试锁干扰。
	child := startLocalWorkerChild(t, e, binary, dsn, runtimeFile, f.path, executionFile)
	child.waitReady(t)
	var finalLock *sql.Tx
	if mode == "final" {
		var err error
		finalLock, err = e.db.BeginTx(e.ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = finalLock.Rollback() })
		rows, err := finalLock.QueryContext(e.ctx, `SELECT task_id FROM tm_ai_report_commits WHERE task_id>0 FOR UPDATE`)
		if err != nil {
			t.Fatal(err)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
	created, err := f.creates.Create(e.ctx, 1, "local_recovery_process_key", dto.AITaskCreateInput{AnalysisType: analysis, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "local-v1"})
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, stopWait := context.WithTimeout(e.ctx, 20*time.Second)
	defer stopWait()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		ready := false
		switch mode {
		case "final":
			// 当前读不抢占正在终检的任务锁；真实等待关系证明结算事务尚未提交。
			var n int
			err := e.db.QueryRowContext(waitCtx, `SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID AND l.ENGINE=w.ENGINE WHERE l.OBJECT_SCHEMA=? AND l.OBJECT_NAME='tm_ai_report_commits'`, e.database).Scan(&n)
			if err != nil {
				t.Fatal("不能核验本轮最终提交阻塞")
			}
			ready = n > 0 && e.queryInt(`SELECT current_step FROM tm_ai_tasks WHERE id=?`, created.TaskID) == 5
		case "agent":
			ready = e.agentPolls.Load() > 0 && e.queryInt(`SELECT COUNT(*) FROM tm_ai_agent_runs WHERE state='running' AND remote_id IS NOT NULL`) == 1
		case "model":
			ready = e.server.Count() == 1 && e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) == 1
		}
		if ready {
			break
		}
		select {
		case <-tick.C:
		case <-waitCtx.Done():
			t.Fatal("实际worker未进入强杀阶段")
		case err := <-child.done:
			child.waited = true
			t.Fatal("强杀前worker提前退出", err, child.output.String())
		}
	}
	if err := child.cmd.Process.Kill(); err != nil {
		t.Fatal("强杀实际worker失败")
	}
	child.wait(t, false)
	t.Log("实际worker已强杀并回收，未把退出当作任务终态")
	if finalLock != nil {
		if err := finalLock.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
	before, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
	if err != nil || before.Status != task.Running || before.LeaseUntil == nil || e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != int(created.PricePoints) {
		t.Fatal("强杀制造终态或释放", err)
	}
	// 未过原租约必须拒绝，跨用户和缺失目标不能外呼或改变原记录。
	resume(1, created.TaskID, 1, "LOCAL_REPORT_RESUME_REFUSED")
	resume(2, created.TaskID, 1, "阶段4")
	resume(3, created.TaskID, 1, "阶段4")
	if e.server.Count() != map[string]int{"final": 3, "agent": 0, "model": 1}[mode] || e.submits.Load() != 1 {
		t.Fatal("到期前/越权恢复产生调用")
	}
	t.Log("原租约未到期及跨用户恢复已拒绝；等待自然到期")
	waitTaskTime(t, e.pointsTestEnv, *before.LeaseUntil)
	if mode == "agent" {
		e.agentComplete.Store(true)
	}
	want, marker := 2, "LOCAL_REPORT_RESUME_UNKNOWN"
	if mode == "final" {
		want, marker = 0, "LOCAL_REPORT_RESUME_TERMINAL"
	}
	resume(1, created.TaskID, want, marker)
	after, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Fence != before.Fence+1 || after.Attempt != before.Attempt+1 {
		t.Fatal("未按自然租约更新围栏/尝试")
	}
	if _, err := e.tasks.Heartbeat(e.ctx, 1, created.TaskID, before.LeaseOwner, before.Fence, time.Minute); !errors.Is(err, task.ErrLease) {
		t.Fatal("原进程旧围栏仍可写", err)
	}
	if mode == "final" {
		resume(1, created.TaskID, 0, marker)
		if after.Status != task.Succeeded || after.CurrentStep != 5 || e.server.Count() != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT avail FROM tm_points_accounts WHERE user_id=1`) != 60-int(created.PricePoints) || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
			t.Fatal("完整回执恢复未恰好一次交付/结算")
		}
		base, err := service.NewAIReportService(e.repo, e.ledger, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		control, err := service.NewAILocalReportReadControl(f.admission)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := service.NewAIReportReadServiceWithPolicySource(base, control, control, true)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			report, err := reader.Read(e.ctx, 1, created.TaskID)
			if err != nil || len(report.Sections) != 4 {
				t.Fatal("恢复报告不可读", err)
			}
			n := 0
			for _, s := range report.Sections {
				n += len(s.Quotes)
			}
			if kind == 2 && n < 6 {
				t.Fatal("恢复深度报告缺少六引文")
			}
		}
	} else {
		// 这不是自动恢复成功：旧围栏缺失回执必须保留原键，不补写或二次付费。
		resume(1, created.TaskID, 1, "LOCAL_REPORT_RESUME_REFUSED")
		if after.Status != task.Running || after.CurrentStep != before.CurrentStep || e.server.Count() != map[string]int{"agent": 0, "model": 1}[mode] || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != int(created.PricePoints) {
			t.Fatal("缺回执被当作成功/重发/释放")
		}
	}
	if e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'`) != 1 {
		t.Fatal("恢复重复准入/提交/释放")
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
}
