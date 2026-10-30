//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func runLocalProcessH5(t *testing.T, depth, origin string, scenario *localProcessCase) {
	t.Helper()
	api, worker, login := buildLocalReportBinaries(t)
	kind, workflow := 1, "local_running"
	if depth == "deep" {
		kind, workflow = 2, "local_deep_running"
	}
	live := scenario != nil
	duration, lifetime := 5*time.Minute, 10*time.Minute
	if live {
		duration, lifetime = 15*time.Minute, 18*time.Minute
		workflow = "local"
		if kind == 2 {
			workflow = "local_deep"
		}
	}
	f := newLocalPublicationFixtureWithDuration(t, sql.LevelReadCommitted, kind, workflow, duration)
	e := f.env
	ctx, cancel := context.WithTimeout(context.Background(), lifetime)
	defer cancel()
	e.pointsTestEnv.ctx = ctx
	localAPISchema(t, e)
	// 搜索仍走正式目录仓储；补全本轮合成题材表的只读字段，不返回测试拼装的搜索成功。
	e.mustExec(`ALTER TABLE addon_quant_theme ADD code varchar(32) NULL, ADD level tinyint NOT NULL DEFAULT 1, ADD parent_id bigint NOT NULL DEFAULT 0, ADD description text, ADD sort int NOT NULL DEFAULT 0, ADD updated_at datetime(3) NULL, ADD created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)`)
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	signing := hex.EncodeToString(random[:])
	bridge := startIsolatedMemberLogin(t, ctx, e.database, signing, login)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.runtime.Listen = listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	runtimeFile, executionFile := filepath.Join(dir, "runtime.json"), filepath.Join(dir, "execution.json")
	write := func(path string, v any) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(runtimeFile, f.runtime)
	cc, mc := e.agentConfig, workflowModelConfig(e.server.URL())
	execution := config.AILocalExecution{Version: 1, InstanceID: f.runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: time.Now().UnixMilli() - 1000, ExpiresAtMS: f.file.ExpiresAtMS, Model: config.AILocalExecutionModel{Name: mc.Name, BaseURL: mc.BaseURL, Model: mc.Model, TimeoutMS: mc.TimeoutMS}, Agent: config.AILocalExecutionAgent{BaseURL: cc.BaseURL, ApprovalRef: cc.ApprovalRef, LicenseRef: cc.LicenseRef, QuickModel: cc.QuickModel, DeepModel: cc.DeepModel, TimeoutMS: int(cc.Timeout / time.Millisecond), MaxSubmissions: cc.MaxSubmissions}, LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 300000, AgentPollMS: 50}
	if live {
		scenario.configure(t, f, &execution)
		f.file.Plans = []config.AILocalPlan{f.spec}
		write(f.path, f.file)
	}
	write(executionFile, execution)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	base := "http://" + f.runtime.Listen
	startLocalAPI(t, e, api, localAPIDSN(t, e), signing, runtimeFile, f.path, client, base, kind == 2)
	child := startLocalWorkerChild(t, e, worker, localWorkerDSN(t, e), runtimeFile, f.path, executionFile, e.modelSession)
	child.waitReady(t)
	g := newLocalAPIWorkerH5Gateway(t, origin, base, bridge, func() bool {
		if e.agentPolls.Load() == 0 {
			return false
		}
		e.agentComplete.Store(true)
		return true
	}, live)
	t.Log("正式API与独立worker的实际H5入口（仅测试）：", g.server.URL+"/__test__/")
	func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-g.finish:
				return
			case err := <-child.done:
				child.waited = true
				// 仅在已等待实际退出后读缓冲，临时桥凭据不得进入诊断。
				diagnostic := child.output.String() + child.stderr.String()
				for _, secret := range []string{e.modelSession, e.agentConfig.Bearer} {
					if secret != "" {
						diagnostic = strings.ReplaceAll(diagnostic, secret, "[隐藏临时凭据]")
					}
				}
				t.Fatal("实际worker在浏览器验收完成前退出", err, diagnostic)
			case <-ctx.Done():
				t.Fatal("未在本轮浏览器期限内完成验收")
			case <-tick.C:
				if !live {
					continue
				}
				var id int64
				err := e.db.QueryRowContext(ctx, `SELECT task_id FROM tm_ai_task_admissions`).Scan(&id)
				if errors.Is(err, sql.ErrNoRows) {
					continue // 页面尚未创建任务，不预造业务记录。
				}
				if err != nil {
					t.Fatal("真实浏览器任务监测读取失败")
				}
				v, err := e.tasks.Inspect(ctx, 1, id)
				if err != nil {
					t.Fatal("真实浏览器任务状态读取失败")
				}
				if v.Status >= 4 {
					t.Fatalf("真实浏览器任务未交付：status=%d step=%d code=%s frozen=%d reports=%d settle=%d", v.Status, v.CurrentStep, v.ErrorCode, e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`), e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`))
				}
			}
		}
	}()
	// 页面交付与重读计数来自实际API响应头；业务一致性仍直接核对同一个原账本。
	var taskID int64
	if err := e.db.QueryRowContext(ctx, `SELECT task_id FROM tm_ai_task_admissions`).Scan(&taskID); err != nil {
		t.Fatal("没有原持久准入任务")
	}
	v, err := e.tasks.Inspect(ctx, 1, taskID)
	if err != nil || v.Status != 3 || v.CurrentStep != 5 {
		t.Fatal("页面交付未对应原终检成功任务")
	}
	wantAgent, wantModel := int32(1), 3
	if live {
		wantAgent, wantModel = 0, 0
	}
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=1 AND task_id>0`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=2`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 6 || e.queryInt(`SELECT avail FROM tm_points_accounts WHERE user_id=1`) != 60-int(f.spec.PricePoints) || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts WHERE user_id=2`) != 0 || e.submits.Load() != wantAgent || len(e.server.Requests()) != wantModel {
		t.Fatal("实际进程H5报告、积分或调用次数不符")
	}
	if _, err := io.WriteString(child.input, "stop\n"); err != nil {
		t.Fatal(err)
	}
	child.wait(t, true)
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
	bridge.finish(t, ctx)
	if live {
		t.Log("TM_STOCK_LIVE_PROCESS_H5_VERIFIED")
	} else {
		t.Log("实际H5原密码→正式API→独立队列worker→原终检报告/一次结算通过；回环Agent和模型，非真实模型证据")
	}
}
