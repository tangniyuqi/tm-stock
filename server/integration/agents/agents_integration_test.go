//go:build integration

package agents_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func agentDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("须在可丢弃 MySQL 实例设置 TM_TEST_DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("连接隔离测试库失败")
	}
	db.SetMaxOpenConns(12)
	drop := func() {
		for _, q := range []string{"DROP TRIGGER IF EXISTS tm_ai_agent_test_fail", "DROP TABLE IF EXISTS tm_ai_agent_events", "DROP TABLE IF EXISTS tm_ai_agent_runs", "DROP TABLE IF EXISTS tm_ai_agent_profiles"} {
			if _, err := db.Exec(q); err != nil {
				t.Error(err)
			}
		}
	}
	drop()
	t.Cleanup(func() { drop(); _ = db.Close() })
	ddl, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261015_create_tm_ai_agent_runs.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(string(ddl)); err != nil {
			t.Fatal("迁移执行失败", err)
		}
	}
	return db
}

type executor struct {
	calls       atomic.Int32
	cancelCalls atomic.Int32
	onSubmit    func()
	state       atomic.Value
	fail        bool
}

func (e *executor) Submit(_ context.Context, s string) (agents.Acceptance, error) {
	n := e.calls.Add(1)
	if e.onSubmit != nil {
		e.onSubmit()
	}
	if e.fail {
		return agents.Acceptance{}, context.DeadlineExceeded
	}
	return agents.Acceptance{RemoteID: s + "-" + string(rune('a'+n))}, nil
}
func (e *executor) Status(_ context.Context, id string) (agents.Observation, error) {
	v := e.state.Load()
	state := agents.Completed
	if v != nil {
		state = v.(string)
	}
	return agents.Observation{RemoteID: id, State: state}, nil
}
func (e *executor) Result(_ context.Context, id string) (agents.Result, error) {
	return agents.Result{RemoteID: id, Payload: []byte(`{"analysis_id":"` + id + `","reports":{"news":"模拟上游、未核验的内部正文"}}`)}, nil
}
func (e *executor) Cancel(context.Context, string) error { e.cancelCalls.Add(1); return nil }

func agentSvc(t *testing.T, db *sql.DB, e *executor, p agents.Profile) *service.AIAgentService {
	t.Helper()
	s, err := service.NewAIAgentService(repository.NewAIAgentRepo(db), e, p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func profile(cap int) agents.Profile {
	return agents.Profile{Digest: strings.Repeat("a", 64), MaxSubmissions: cap}
}

func TestIntegrationAgentLifecycleAndProtections(t *testing.T) {
	db := agentDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := repository.NewAIAgentRepo(db)
	if err := repo.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tm_ai_agent_runs`).Scan(&count); err != nil || count != 0 {
		t.Fatal("自检留下数据", err, count)
	}
	e := &executor{}
	s := agentSvc(t, db, e, profile(2))
	req := agents.Request{RequestID: "integration_agent_01", Symbol: "600001"}
	r, err := s.Submit(ctx, 7, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agentSvc(t, db, e, profile(2)).Submit(ctx, 7, req); err != nil || e.calls.Load() != 1 {
		t.Fatal("真实持久记录未幂等", err)
	}
	if _, err := s.Submit(ctx, 8, req); !errors.Is(err, agents.ErrConflict) {
		t.Fatal("身份错配放行", err)
	}
	if _, err := s.Cancel(ctx, 7, req.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cancel(ctx, 7, req.RequestID); err != nil || e.cancelCalls.Load() != 1 {
		t.Fatal("取消重复调用", err)
	}
	r, err = s.Status(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Completed {
		t.Fatal(r, err)
	}
	r, err = s.Result(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Quarantined {
		t.Fatal(r, err)
	}
	var raw []byte
	var seq, events int
	if err := db.QueryRow(`SELECT result_payload,event_seq FROM tm_ai_agent_runs WHERE id=?`, r.ID).Scan(&raw, &seq); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tm_ai_agent_events WHERE run_id=?`, r.ID).Scan(&events); err != nil || seq != events || seq != 6 {
		t.Fatal("事件不完整", seq, events, err)
	}
	if (agents.Result{RemoteID: r.RemoteID, Payload: raw}).Digest() != r.ResultDigest {
		t.Fatal("隔离正文摘要不符")
	}
	public, _ := json.Marshal(r)
	if strings.Contains(string(public), "内部正文") {
		t.Fatal("状态输出泄漏正文")
	}
	for _, q := range []string{`UPDATE tm_ai_agent_events SET record_json='{}'`, `DELETE FROM tm_ai_agent_events`, `DELETE FROM tm_ai_agent_runs`, `UPDATE tm_ai_agent_runs SET state='accepted'`} {
		if _, err := db.ExecContext(ctx, q); err == nil {
			t.Fatal("数据库未拒绝非法改写", q)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE tm_ai_agent_runs SET result_payload=? WHERE id=?`, []byte(`{"analysis_id":"wrong","reports":{"news":"篡改"}}`), r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByRequest(ctx, req.RequestID); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("正文篡改未检出", err)
	}
}

func TestIntegrationAgentConcurrentCancelAndAuditIntegrity(t *testing.T) {
	db := agentDB(t)
	e := &executor{}
	s := agentSvc(t, db, e, profile(2))
	ctx := context.Background()
	req := agents.Request{RequestID: "integration_cancel_race", Symbol: "600001"}
	r, err := s.Submit(ctx, 7, req)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Cancel(ctx, 7, req.RequestID)
			if err != nil && !errors.Is(err, agents.ErrUncertain) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if e.cancelCalls.Load() != 1 {
		t.Fatal("真库取消重复调用", e.cancelCalls.Load())
	}
	if _, err := db.Exec(`DROP TRIGGER tm_ai_agent_events_no_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE tm_ai_agent_events SET record_json='{}' WHERE run_id=? AND seq=(SELECT event_seq FROM tm_ai_agent_runs WHERE id=?)`, r.ID, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Show(ctx, 7, req.RequestID); !errors.Is(err, agents.ErrInvalid) {
		t.Fatal("事件与当前记录不符未拒绝", err)
	}
}

func TestIntegrationAgentCLI(t *testing.T) {
	db := agentDB(t)
	e := &executor{}
	s := agentSvc(t, db, e, profile(2))
	req := agents.Request{RequestID: "integration_cli_01", Symbol: "600001"}
	r, err := s.Submit(context.Background(), 7, req)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ai-agent.exe")
	coverDir := t.TempDir()
	build := exec.Command("go", "build", "-cover", "-coverpkg=github.com/tangniyuqi/tm-stock/server/cmd/ai-agent", "-o", path, "./cmd/ai-agent")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("构建内部命令失败：%s %v", out, err)
	}
	run := func(actor, op, input string) ([]byte, error) {
		cmd := exec.Command(path, op)
		cmd.Stdin = strings.NewReader(input)
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "TM_DB_DSN=") && !strings.HasPrefix(v, "TM_AI_AGENTS_ACTOR_ID=") && !strings.HasPrefix(v, "TM_AI_AGENTS_ENABLED=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "TM_DB_DSN="+os.Getenv("TM_TEST_DSN"), "TM_AI_AGENTS_ACTOR_ID="+actor, "TM_AI_AGENTS_ENABLED=false", "GOCOVERDIR="+coverDir)
		return cmd.CombinedOutput()
	}
	out, err := run("7", "show", `{"request_id":"integration_cli_01"}`)
	if err != nil {
		t.Fatal("命令查询失败", err, string(out))
	}
	var got struct {
		Record agents.Record `json:"record"`
		Usage  bool          `json:"billable_usage_known"`
		Report bool          `json:"public_report_available"`
	}
	if json.Unmarshal(out, &got) != nil || got.Record != r || got.Usage || got.Report {
		t.Fatal("命令输出错误", string(out))
	}
	if out, err := run("8", "show", `{"request_id":"integration_cli_01"}`); err == nil || strings.Contains(string(out), req.Symbol) {
		t.Fatal("命令身份隔离失败")
	}
	if _, err := run("7", "submit", `{"request_id":"integration_cli_02","symbol":"600001"}`); err == nil {
		t.Fatal("命令开关默认未关闭")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tm_ai_agent_runs`).Scan(&n); err != nil || n != 1 || e.calls.Load() != 1 {
		t.Fatal("关闭开关仍建任务或外联", err, n)
	}
	var submissions, cancellations atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/analysis/single":
			submissions.Add(1)
			io.WriteString(w, `{"success":true,"data":{"task_id":"cli-local-remote","status":"pending"}}`)
		case "/api/analysis/tasks/cli-local-remote/status":
			io.WriteString(w, `{"success":true,"data":{"task_id":"cli-local-remote","status":"completed"}}`)
		case "/api/analysis/tasks/cli-local-remote/cancel":
			cancellations.Add(1)
			io.WriteString(w, `{"success":true}`)
		case "/api/analysis/tasks/cli-local-remote/result":
			io.WriteString(w, `{"success":true,"data":{"analysis_id":"cli-local-remote","reports":{"news":"模拟上游隔离正文"}}}`)
		default:
			t.Error("意外远端路径", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	local := func(op, input string) agents.Record {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, op)
		cmd.Stdin = strings.NewReader(input)
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "TM_AI_AGENTS_") && !strings.HasPrefix(v, "TM_DB_DSN=") && !strings.HasPrefix(v, "GOCOVERDIR=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "TM_DB_DSN="+os.Getenv("TM_TEST_DSN"), "TM_AI_AGENTS_ACTOR_ID=9", "TM_AI_AGENTS_ENABLED=true", "TM_AI_AGENTS_LOCAL_EVALUATION=true", "TM_AI_AGENTS_URL="+upstream.URL, "TM_AI_AGENTS_APPROVED_HOST=127.0.0.1", "TM_AI_AGENTS_TOKEN="+strings.Repeat("x", 40), "TM_AI_AGENTS_APPROVAL_REF=test-only:approval", "TM_AI_AGENTS_LICENSE_REF=test-only:evaluation", "TM_AI_AGENTS_QUICK_MODEL=test-quick", "TM_AI_AGENTS_DEEP_MODEL=test-deep", "TM_AI_AGENTS_TIMEOUT_MS=1000", "TM_AI_AGENTS_MAX_SUBMISSIONS=2", "GOCOVERDIR="+coverDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal("本地模拟 API 命令失败", op, err, string(out))
		}
		var got struct {
			Record agents.Record `json:"record"`
		}
		if json.Unmarshal(out, &got) != nil || strings.Contains(string(out), "隔离正文") {
			t.Fatal("命令泄漏正文或输出错误", string(out))
		}
		return got.Record
	}
	input := `{"request_id":"integration_cli_local","symbol":"600002"}`
	lookup := `{"request_id":"integration_cli_local"}`
	if local("submit", input).State != agents.Accepted || local("submit", input).State != agents.Accepted {
		t.Fatal("实际命令提交/幂等错误")
	}
	if local("cancel", lookup).State != agents.Accepted || local("cancel", lookup).State != agents.Accepted {
		t.Fatal("取消受理被当终态")
	}
	if local("status", lookup).State != agents.Completed || local("result", lookup).State != agents.Quarantined {
		t.Fatal("实际命令生命周期错误")
	}
	if submissions.Load() != 1 || cancellations.Load() != 1 {
		t.Fatal("实际命令重复提交或取消")
	}
	cover := exec.Command("go", "tool", "covdata", "percent", "-i", coverDir)
	if out, err := cover.CombinedOutput(); err != nil {
		t.Fatal("读取命令覆盖率失败", err)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}

func TestIntegrationAgentConcurrentReserveAndDurableLimit(t *testing.T) {
	db := agentDB(t)
	e := &executor{}
	s := agentSvc(t, db, e, profile(2))
	ctx := context.Background()
	req := agents.Request{RequestID: "integration_concurrent_01", Symbol: "600001"}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Submit(ctx, 7, req)
			if err != nil && !errors.Is(err, agents.ErrUncertain) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if e.calls.Load() != 1 {
		t.Fatal("并发重复提交", e.calls.Load())
	}
	// 新连接池/新 service 共享同一数据库限额，不以进程内计数冒充持久限额。
	other, err := sql.Open("mysql", os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("打开第二连接池失败")
	}
	defer func() { _ = other.Close() }()
	s2 := agentSvc(t, other, e, profile(2))
	req.RequestID = "integration_concurrent_02"
	e.fail = true
	if _, err := s2.Submit(ctx, 7, req); !errors.Is(err, agents.ErrUncertain) {
		t.Fatal(err)
	}
	if _, err := s.Submit(ctx, 7, req); !errors.Is(err, agents.ErrUncertain) {
		t.Fatal("未知提交被重放", err)
	}
	req.RequestID = "integration_concurrent_03"
	if _, err := s.Submit(ctx, 7, req); !errors.Is(err, agents.ErrLimit) {
		t.Fatal("未知提交返还限额", err)
	}
	var used int
	if err := db.QueryRow(`SELECT reserved_submissions FROM tm_ai_agent_profiles`).Scan(&used); err != nil || used != 2 || e.calls.Load() != 2 {
		t.Fatal("持久限额失配", err, used, e.calls.Load())
	}
}

func TestIntegrationAgentAuditFailureRollsBackAndNeverReplays(t *testing.T) {
	db := agentDB(t)
	e := &executor{}
	e.onSubmit = func() {
		if _, err := db.Exec(`CREATE TRIGGER tm_ai_agent_test_fail BEFORE INSERT ON tm_ai_agent_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only audit failure'`); err != nil {
			t.Error(err)
		}
	}
	s := agentSvc(t, db, e, profile(2))
	ctx := context.Background()
	req := agents.Request{RequestID: "integration_audit_fail_01", Symbol: "600001"}
	r, err := s.Submit(ctx, 7, req)
	if !errors.Is(err, agents.ErrUncertain) {
		t.Fatal("审计失败仍成功", r, err)
	}
	if _, err := db.Exec(`DROP TRIGGER tm_ai_agent_test_fail`); err != nil {
		t.Fatal(err)
	}
	r, err = s.Show(ctx, 7, req.RequestID)
	if err != nil || r.State != agents.Submitting || r.RemoteID != "" {
		t.Fatal("终态事务未回滚", r, err)
	}
	if _, err := s.Submit(ctx, 7, req); !errors.Is(err, agents.ErrUncertain) || e.calls.Load() != 1 {
		t.Fatal("审计故障后重复调用", err)
	}
	var seq, n int
	if err := db.QueryRow(`SELECT event_seq FROM tm_ai_agent_runs WHERE id=?`, r.ID).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tm_ai_agent_events WHERE run_id=?`, r.ID).Scan(&n); err != nil || n != seq || n != 1 {
		t.Fatal("预登记事件未保留", err, seq, n)
	}
}

func TestIntegrationAgentProbeDetectsMissingProtection(t *testing.T) {
	db := agentDB(t)
	repo := repository.NewAIAgentRepo(db)
	ctx := context.Background()
	for _, q := range []string{`ALTER TABLE tm_ai_agent_runs DROP INDEX uk_tm_ai_agent_remote`, `DROP TRIGGER tm_ai_agent_events_no_update`, `ALTER TABLE tm_ai_agent_profiles DROP CHECK chk_tm_ai_agent_profile`} {
		if strings.Contains(q, "DROP INDEX") {
			if _, err := db.Exec(`CREATE INDEX tm_ai_agent_test_profile ON tm_ai_agent_runs(profile_digest)`); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
		if err := repo.VerifySchema(ctx); err == nil {
			t.Fatal("自检没有检出保护被移除", q)
		}
		// 每个反例重新执行开发迁移必须在空测试表上；不把 IF NOT EXISTS 当升级器。
		for _, q := range []string{"DROP TABLE tm_ai_agent_events", "DROP TABLE tm_ai_agent_runs", "DROP TABLE tm_ai_agent_profiles"} {
			if _, err := db.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		ddl, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261015_create_tm_ai_agent_runs.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(ddl)); err != nil {
			t.Fatal(err)
		}
	}
}
