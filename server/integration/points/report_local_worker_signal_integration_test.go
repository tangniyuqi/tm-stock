//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
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
)

func TestIntegrationAIReportLocalWorkerSignal(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	binary := filepath.Join(t.TempDir(), "local-ai-worker")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Minute)
	defer stop()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/ai-worker")
	build.Dir = filepath.Join("..", "..")
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("构建实际worker失败")
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					for _, mode := range []string{"idle", "agent", "model"} {
						t.Run(mode, func(t *testing.T) { verifyLocalWorkerSignal(t, binary, level, kind, mode) })
					}
				})
			}
		})
	}
}

func verifyLocalWorkerSignal(t *testing.T, binary string, level sql.IsolationLevel, kind int, mode string) {
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
	dsn := localWorkerDSN(t, e)
	dir := t.TempDir()
	runtimeFile, executionFile := filepath.Join(dir, "runtime.json"), filepath.Join(dir, "execution.json")
	write := func(path string, v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
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
	child, sendSignal, marker := startLocalSignalWorker(t, e, binary, dsn, runtimeFile, f.path, executionFile)
	child.waitReady(t)
	var id int64
	price := 0
	idle := mode == "idle" || mode == "invalid"
	if !idle {
		created, err := f.creates.Create(e.ctx, 1, "local_signal_task_key", dto.AITaskCreateInput{AnalysisType: analysis, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "local-v1"})
		if err != nil {
			t.Fatal(err)
		}
		id, price = created.TaskID, int(created.PricePoints)
		waitCtx, stop := context.WithTimeout(e.ctx, 20*time.Second)
		defer stop()
		tick := time.NewTicker(25 * time.Millisecond)
		defer tick.Stop()
		for {
			ready := mode == "agent" && e.agentPolls.Load() > 0 && e.queryInt(`SELECT COUNT(*) FROM tm_ai_agent_runs WHERE state='running' AND remote_id IS NOT NULL`) == 1 || mode == "model" && e.server.Count() == 1 && e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) == 1
			if ready {
				break
			}
			select {
			case <-tick.C:
			case <-waitCtx.Done():
				t.Fatal("实际worker未进入信号阶段")
			case err := <-child.done:
				child.waited = true
				t.Fatal("信号前实际worker提前退出", err, child.output.String())
			}
		}
	}
	if mode == "invalid" {
		if _, err := io.WriteString(child.input, "stop\n"); err != nil {
			t.Fatal("不能验证非法控制命令")
		}
	} else if err := sendSignal(); err != nil {
		t.Fatal("不能向本轮worker发送操作系统信号")
	}
	child.wait(t, false)
	if mode == "invalid" {
		if marker == "" || child.cmd.ProcessState.ExitCode() != 1 || strings.Contains(child.output.String(), "LOCAL_SIGNAL_HELPER_SENT") || strings.Contains(child.output.String(), "LOCAL_SIGNAL_WORKER_EXIT:") || !strings.Contains(child.output.String(), "LOCAL_SIGNAL_HELPER_CLEANED") {
			t.Fatal("非法管道命令触发了信号或伪造退出", child.output.String())
		}
	} else {
		if child.cmd.ProcessState.ExitCode() != 130 || marker != "" && (!strings.Contains(child.output.String(), marker) || !strings.Contains(child.output.String(), "LOCAL_SIGNAL_HELPER_SENT")) {
			t.Fatal("实际worker没有通过系统信号上下文退出", child.output.String())
		}
		if !strings.Contains(child.output.String(), "LOCAL_REPORT_WORKER_STOP_REASON:") || !strings.Contains(child.output.String(), "阶段5") {
			t.Fatal("实际worker未走原停止路径")
		}
	}
	models, submits, audits, step := 0, 0, 0, 0
	if mode == "agent" {
		submits = 1
	}
	if mode == "model" {
		models, submits, audits, step = 1, 1, 2, 1
	}
	if e.server.Count() != models || int(e.submits.Load()) != submits || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != audits || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='release'`) != 1 || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != price || e.queryInt(`SELECT avail FROM tm_points_accounts WHERE user_id=1`) != 60-price {
		t.Fatal("系统信号被当成任务取消、释放或成功，或产生额外调用")
	}
	if !idle {
		v, err := e.tasks.Inspect(e.ctx, 1, id)
		if err != nil || v.Status != task.Running || v.CancelRequested || v.CurrentStep != step || v.LeaseUntil == nil {
			t.Fatal("系统信号篡改原任务状态", err)
		}
	}
	if mode == "agent" && e.queryInt(`SELECT COUNT(*) FROM tm_ai_agent_runs WHERE state='running' AND remote_id IS NOT NULL`) != 1 {
		t.Fatal("本地退出伪造了远端已停止")
	}
	if strings.Contains(child.output.String()+child.stderr.String(), cc.Bearer) || strings.Contains(child.output.String()+child.stderr.String(), dsn) {
		t.Fatal("信号路径输出泄漏凭据")
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
}
