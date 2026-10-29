//go:build integration && browser

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestIntegrationAIReportLocalAPIWorkerH5(t *testing.T) {
	mode, depth, origin := os.Getenv("TM_TEST_AI_H5_LOCAL_API_WORKER"), os.Getenv("TM_TEST_AI_H5_LOCAL_DEPTH"), os.Getenv("TM_TEST_AI_H5_ORIGIN")
	if !localAPIWorkerH5Configuration(mode, depth, origin) {
		t.Fatal("只允许显式本地原登录、轻深选择及完整H5回环源")
	}
	for _, key := range []string{"TM_TEST_AI_LIVE", "TM_TEST_AI_H5_DEEP", "TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_TIMEOUT_RECOVERY", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_REPORT_QA", "TM_TEST_AI_H5_ASSISTANT"} {
		if os.Getenv(key) != "" {
			t.Fatal("正式进程H5不能混用真实调用或其它浏览器专项")
		}
	}
	api, worker, login := buildLocalReportBinaries(t)
	kind, workflow := 1, "local_running"
	if depth == "deep" {
		kind, workflow = 2, "local_deep_running"
	}
	f := newLocalPublicationFixtureWithDuration(t, sql.LevelReadCommitted, kind, workflow, 5*time.Minute)
	e := f.env
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
	write(executionFile, config.AILocalExecution{Version: 1, InstanceID: f.runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: time.Now().UnixMilli() - 1000, ExpiresAtMS: f.file.ExpiresAtMS, Model: config.AILocalExecutionModel{Name: mc.Name, BaseURL: mc.BaseURL, Model: mc.Model, TimeoutMS: mc.TimeoutMS}, Agent: config.AILocalExecutionAgent{BaseURL: cc.BaseURL, ApprovalRef: cc.ApprovalRef, LicenseRef: cc.LicenseRef, QuickModel: cc.QuickModel, DeepModel: cc.DeepModel, TimeoutMS: int(cc.Timeout / time.Millisecond), MaxSubmissions: cc.MaxSubmissions}, LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 300000, AgentPollMS: 50})
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	base := "http://" + f.runtime.Listen
	startLocalAPI(t, e, api, localAPIDSN(t, e), signing, runtimeFile, f.path, client, base, kind == 2)
	child := startLocalWorkerChild(t, e, worker, localWorkerDSN(t, e), runtimeFile, f.path, executionFile)
	child.waitReady(t)
	g := newLocalAPIWorkerH5Gateway(t, origin, base, bridge, func() bool {
		if e.agentPolls.Load() == 0 {
			return false
		}
		e.agentComplete.Store(true)
		return true
	})
	t.Log("正式API与独立worker的实际H5入口（仅测试）：", g.server.URL+"/__test__/")
	select {
	case <-g.finish:
	case <-ctx.Done():
		t.Fatal("未在本轮浏览器期限内完成验收")
	}
	// 页面交付与重读计数来自实际API响应头；业务一致性仍直接核对同一个原账本。
	var taskID int64
	if err := e.db.QueryRowContext(ctx, `SELECT task_id FROM tm_ai_task_admissions`).Scan(&taskID); err != nil {
		t.Fatal("没有原持久准入任务")
	}
	v, err := e.tasks.Inspect(ctx, 1, taskID)
	if err != nil || v.Status != 3 || v.CurrentStep != 5 {
		t.Fatal("页面交付未对应原终检成功任务")
	}
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_admissions`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=1 AND task_id>0`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=2`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 6 || e.queryInt(`SELECT avail FROM tm_points_accounts WHERE user_id=1`) != 60-int(f.spec.PricePoints) || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts WHERE user_id=2`) != 0 || e.submits.Load() != 1 || len(e.server.Requests()) != 3 {
		t.Fatal("实际进程H5报告、积分或调用次数不符")
	}
	if _, err := io.WriteString(child.input, "stop\n"); err != nil {
		t.Fatal(err)
	}
	child.wait(t, true)
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
	bridge.finish(t, ctx)
	t.Log("实际H5原密码→正式API→独立队列worker→原终检报告/一次结算通过；回环Agent和模型，非真实模型证据")
}
