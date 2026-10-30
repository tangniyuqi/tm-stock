//go:build integration

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
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 实际二进制之间的HTTP证据；模型与Agent为回环协议替身，不计作浏览器或真实模型验收。
func TestIntegrationAIReportLocalAPIWorker(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) { verifyLocalAPIWorker(t, level, kind, api, worker, login) })
			}
		})
	}
}

func buildLocalReportBinaries(t *testing.T) (api, worker, login string) {
	t.Helper()
	dir := t.TempDir()
	build := func(name, root, target string, args ...string) string {
		t.Helper()
		binary := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		// CI冷缓存需编译原GVA依赖；此期限不改变任何任务或启动自检时限。
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", append(args, "-o", binary, target)...)
		// 输出可能包含环境相关信息，失败时不转发。
		cmd.Dir = root
		if _, err := cmd.CombinedOutput(); err != nil {
			t.Fatal("构建实际进程失败：", name)
		}
		return binary
	}
	// 先编译，再开始每个隔离库的有效期。
	api = build("api", filepath.Join("..", ".."), "./cmd/api", "build")
	worker = build("worker", filepath.Join("..", ".."), "./cmd/ai-worker", "build")
	login = build("member-login", filepath.Join("..", "..", "..", "backend", "server"), "./router/client", "test", "-c", "-tags=integration")
	return
}

type localAPIData struct {
	ID              int64                 `json:"id"`
	TaskID          int64                 `json:"taskId"`
	MessageSeq      int64                 `json:"messageSeq"`
	State           string                `json:"state"`
	Status          int                   `json:"status"`
	EventSeq        int64                 `json:"eventSeq"`
	ReportAvailable bool                  `json:"reportAvailable"`
	PricePoints     int64                 `json:"pricePoints"`
	PriceVersion    string                `json:"priceVersion"`
	AvailablePoints int64                 `json:"availablePoints"`
	FrozenPoints    int64                 `json:"frozenPoints"`
	Sections        []dto.AIReportSection `json:"sections"`
	Items           []struct {
		Seq    int64  `json:"seq"`
		TaskID int64  `json:"taskId"`
		Kind   string `json:"kind"`
		Skill  string `json:"skill"`
		Type   string `json:"type"`
		Status int    `json:"status"`
	} `json:"items"`
}

func localAPISchema(t *testing.T, e *workflowEnv) {
	t.Helper()
	reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_conversations.sql")
	reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_conversations_tasks.sql")
	for _, p := range []struct{ name, condition string }{
		{"tm_ai_local_conversation_trigger_metadata", `event_object_table IN ('tm_ai_conversations','tm_ai_conversation_messages')`},
		{"tm_ai_local_intent_trigger_metadata", `(event_object_table='tm_ai_conversation_tasks' OR event_object_table='tm_ai_conversations' AND trigger_name='tm_ai_conversation_intent_archive_guard')`},
	} {
		e.mustExec(`CREATE PROCEDURE ` + p.name + `() SQL SECURITY DEFINER READS SQL DATA SELECT trigger_schema,event_object_table,trigger_name,event_manipulation,action_timing FROM information_schema.triggers WHERE trigger_schema='` + e.database + `' AND ` + p.condition)
	}
}

func localAPIDSN(t *testing.T, e *workflowEnv, feedback ...bool) string {
	t.Helper()
	if len(feedback) > 1 {
		t.Fatal("每轮只能声明一个反馈范围")
	}
	tables := []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_admissions", "tm_ai_report_commits", "tm_ai_conversations", "tm_ai_conversation_messages", "tm_ai_conversation_tasks"}
	if len(feedback) == 1 && feedback[0] {
		tables = append(tables, "tm_ai_report_feedbacks", "tm_ai_report_feedback_events")
	}
	return localProcessDSN(t, e, tables, true)
}

func localPasswordTokens(t *testing.T, ctx context.Context, b *h5MemberLoginBridge, client *http.Client) (tokens []string, password string) {
	t.Helper()
	request := func(method, path, token, body string) []byte {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, b.url+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Test-Control", b.control)
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("原登录HTTP请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(raw) > 65536 || res.StatusCode != 200 {
			t.Fatal("原登录HTTP响应失败")
		}
		return raw
	}
	html := request("GET", "/__test__/login", "", "")
	m := regexp.MustCompile(`仅本次测试口令：([^<]+)</p>`).FindSubmatch(html)
	if len(m) != 2 {
		t.Fatal("未获取本轮随机测试口令；正文隐藏")
	}
	password = string(m[1])
	var out struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(request("POST", "/client/auth/login", "", `{"mobile":"ai_member_one","password":"wrong-local-only"}`), &out) != nil || out.Code == 0 {
		t.Fatal("错误密码未拒绝")
	}
	for _, name := range []string{"ai_member_one", "ai_member_two"} {
		body, err := json.Marshal(map[string]string{"mobile": name, "password": password})
		if err != nil {
			t.Fatal("编码登录参数失败")
		}
		out.Data.Token = ""
		if json.Unmarshal(request("POST", "/client/auth/login", "", string(body)), &out) != nil || out.Code != 0 || out.Data.Token == "" {
			t.Fatal("原登录未签发JWT")
		}
		tokens = append(tokens, out.Data.Token)
		var profile struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(request("GET", "/client/member/profile", out.Data.Token, ""), &profile) != nil || profile.Code != 0 {
			t.Fatal("原会员资料核验失败")
		}
	}
	return
}

func startLocalAPI(t *testing.T, e *workflowEnv, binary, dsn, signing, runtimeFile, plansFile string, client *http.Client, base string, deep bool, feedback ...bool) {
	startLocalAPIWithQA(t, e, binary, dsn, signing, runtimeFile, plansFile, client, base, deep, "", feedback...)
}
func startLocalAPIWithQA(t *testing.T, e *workflowEnv, binary, dsn, signing, runtimeFile, plansFile string, client *http.Client, base string, deep bool, qaFile string, feedback ...bool) {
	t.Helper()
	if len(feedback) > 1 {
		t.Fatal("每轮只能声明一个反馈范围")
	}
	enabled := len(feedback) == 1 && feedback[0]
	startLocalAPIConfigured(t, e, binary, dsn, signing, runtimeFile, plansFile, client, base, deep, qaFile, enabled, "")
}

func startLocalAPIConfigured(t *testing.T, e *workflowEnv, binary, dsn, signing, runtimeFile, plansFile string, client *http.Client, base string, deep bool, qaFile string, feedback bool, dossierPrepareDSN string, references ...bool) {
	t.Helper()
	if len(references) > 1 || len(references) == 1 && references[0] && dossierPrepareDSN == "" {
		t.Fatal("引用进程配置不完整")
	}
	ctx, cancel := context.WithCancel(e.ctx)
	cmd := exec.CommandContext(ctx, binary)
	// 与worker分别注入账号；不继承任意生产开关或供应商密钥。
	cmd.Env = localWorkerEnvironment(dsn, "")
	cmd.Env = append(cmd.Env, "TM_AI_LOCAL_CONFIG="+runtimeFile, "TM_AI_LOCAL_PLANS="+plansFile, "TM_JWT_SIGNING_KEY="+signing, "TM_JWT_ISSUER=GVA", "TM_JWT_AUDIENCE=GVA")
	for _, flag := range []string{"TM_AI_TASK_READ_ENABLED", "TM_AI_TASK_CREATE_ENABLED", "TM_AI_CONVERSATIONS_ENABLED", "TM_AI_CONVERSATION_TASKS_ENABLED", "TM_AI_TASK_CANCEL_ENABLED", "TM_AI_POINTS_BALANCE_ENABLED", "TM_AI_REPORT_READ_ENABLED", "TM_AI_TASK_STREAM_ENABLED"} {
		cmd.Env = append(cmd.Env, flag+"=true")
	}
	cmd.Env = append(cmd.Env, "TM_AI_DEEP_SUBMIT_ENABLED="+strconv.FormatBool(deep))
	if qaFile != "" {
		cmd.Env = append(cmd.Env, "TM_AI_LOCAL_QA="+qaFile, "TM_AI_QA_READ_ENABLED=true", "TM_AI_QA_SUBMIT_ENABLED=true", "TM_AI_REPORT_QA_SUBMIT_ENABLED=true")
	}
	if feedback {
		cmd.Env = append(cmd.Env, "TM_AI_FEEDBACK_ENABLED=true")
	}
	if dossierPrepareDSN != "" {
		cmd.Env = append(cmd.Env, "TM_AI_LOCAL_DOSSIER_READ_ENABLED=true", "TM_AI_LOCAL_DOSSIER_PREPARE_DSN="+dossierPrepareDSN)
	}
	if len(references) == 1 && references[0] {
		cmd.Env = append(cmd.Env, "TM_AI_LOCAL_CONVERSATION_DOSSIER_ENABLED=true")
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal("启动实际API失败")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ended := false
	t.Cleanup(func() {
		cancel()
		if !ended {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("实际API未退出")
			}
		}
	})
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		r, err := http.NewRequestWithContext(ctx, "GET", base+"/healthz", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Do(r)
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				return
			}
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("实际API就绪超时")
		case <-done:
			ended = true
			t.Fatal("实际API提前退出；输出隐藏")
		}
	}
}

type localProcessCase struct {
	configure             func(*testing.T, *localPublicationFixture, *config.AILocalExecution)
	external              bool
	feedbackAdmin         string
	feedbackBrowserRunner string
	qa                    bool
}

func verifyLocalAPIWorker(t *testing.T, level sql.IsolationLevel, kind int, api, worker, login string, cases ...localProcessCase) {
	t.Helper()
	mode, analysis := "local", "light"
	if kind == 2 {
		mode, analysis = "local_deep", "deep"
	}
	f := newLocalPublicationFixture(t, level, kind, mode)
	e := f.env
	if len(cases) > 1 {
		t.Fatal("每轮只允许一个进程配置")
	}
	external := len(cases) == 1 && cases[0].external
	feedbackEnabled := len(cases) == 1 && cases[0].feedbackAdmin != ""
	if external || len(cases) == 1 && cases[0].feedbackBrowserRunner != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		t.Cleanup(cancel)
		e.pointsTestEnv.ctx = ctx
	}
	localAPISchema(t, e)
	if feedbackEnabled {
		reportDDL(t, e.pointsTestEnv, "20261020_create_tm_ai_report_feedbacks.sql")
	}
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	signing := hex.EncodeToString(seed[:])
	bridge := startIsolatedMemberLogin(t, e.ctx, e.database, signing, login)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(client.CloseIdleConnections)
	tokens, password := localPasswordTokens(t, e.ctx, bridge, client)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.runtime.Listen = listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	base := "http://" + f.runtime.Listen
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
	execution := config.AILocalExecution{Version: 1, InstanceID: f.runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: time.Now().UnixMilli() - 1000, ExpiresAtMS: f.file.ExpiresAtMS, Model: config.AILocalExecutionModel{Name: mc.Name, BaseURL: mc.BaseURL, Model: mc.Model, TimeoutMS: mc.TimeoutMS}, Agent: config.AILocalExecutionAgent{BaseURL: cc.BaseURL, ApprovalRef: cc.ApprovalRef, LicenseRef: cc.LicenseRef, QuickModel: cc.QuickModel, DeepModel: cc.DeepModel, TimeoutMS: int(cc.Timeout / time.Millisecond), MaxSubmissions: cc.MaxSubmissions}, LeaseMS: 60000, HeartbeatMS: 1000, WorkerTimeoutMS: 30000, AgentPollMS: 50}
	if len(cases) == 1 && cases[0].configure != nil {
		cases[0].configure(t, f, &execution)
		f.file.Plans = []config.AILocalPlan{f.spec}
		write(f.path, f.file)
	}
	write(executionFile, execution)
	var feedbackPeer *localFeedbackPeer
	if feedbackEnabled {
		feedbackPeer = startLocalFeedbackPeer(t, f, cases[0].feedbackAdmin, client)
	}
	dsn := localAPIDSN(t, e, feedbackEnabled)
	var qaFixture *localProcessQA
	if len(cases) == 1 && cases[0].qa {
		qaFixture = prepareLocalProcessQA(t, f, dsn)
	}
	qaPath := ""
	if qaFixture != nil {
		qaPath = qaFixture.path
	}
	startLocalAPIWithQA(t, e, api, dsn, signing, runtimeFile, f.path, client, base, kind == 2, qaPath, feedbackEnabled)
	hidden := func(raw []byte) {
		t.Helper()
		for _, s := range []string{password, tokens[0], tokens[1], "req_hash", "idem_key", "task_key", "lease_owner", "budget_json", "profile_digest", "local-eval:", "l1_test_remote"} {
			if strings.Contains(string(raw), s) {
				t.Fatal("公开响应泄漏内部信息；正文隐藏")
			}
		}
	}
	call := func(method, path, token, key, body string, want int) localAPIData {
		t.Helper()
		r, err := http.NewRequestWithContext(e.ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("实际API请求失败", method, path)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 131073))
		if err != nil || len(raw) > 131072 || res.StatusCode != want {
			t.Fatalf("实际API响应不符：%s %s 状态%d，预期%d；正文隐藏", method, path, res.StatusCode, want)
		}
		hidden(raw)
		var out struct {
			Code int          `json:"code"`
			Data localAPIData `json:"data"`
		}
		if json.Unmarshal(raw, &out) != nil || want == 200 && (out.Code != 0 || res.Header.Get("Cache-Control") != "no-store") || want != 200 && out.Code == 0 {
			t.Fatal("实际API响应契约不符")
		}
		return out.Data
	}
	balance := func(available, frozen int64) {
		t.Helper()
		v := call("GET", "/api/v1/ai/points/balance", tokens[0], "", "", 200)
		if v.AvailablePoints != available || v.FrozenPoints != frozen {
			t.Fatal("测试积分余额不符")
		}
	}
	balance(60, 0)
	conv := call("POST", "/api/v1/ai/conversations", tokens[0], "local_api_worker_conv", `{"title":"仅测试：独立API和worker"}`, 200)
	if conv.ID <= 0 {
		t.Fatal("会话未建立")
	}
	root := "/api/v1/ai/conversations/" + strconv.FormatInt(conv.ID, 10)
	quoteBody := `{"analysis_type":"` + analysis + `","subject_type":"theme","subject_id":1}`
	call("POST", "/api/v1/ai/tasks/quote", "", "", quoteBody, 401)
	q := call("POST", "/api/v1/ai/tasks/quote", tokens[0], "", quoteBody, 200)
	price := f.spec.PricePoints
	if q.PricePoints != price || q.PriceVersion != "local-v1" || q.AvailablePoints != 60 {
		t.Fatal("报价与本轮计划不符")
	}
	intents := root + "/task-intents"
	key := "local_api_worker_intent"
	body := strings.TrimSuffix(quoteBody, "}") + `,"price_version":"local-v1"}`
	prepared := call("POST", intents, tokens[0], key, body, 200)
	if prepared.ID <= 0 || prepared.TaskID != 0 || prepared.State != "prepared" {
		t.Fatal("意图准备契约不符")
	}
	if again := call("POST", intents, tokens[0], key, body, 200); again.ID != prepared.ID {
		t.Fatal("准备重放重复意图")
	}
	balance(60, 0)
	intent := intents + "/" + strconv.FormatInt(prepared.ID, 10)
	call("POST", intent+"/confirm", tokens[1], "", `{"confirmed":true}`, 404)
	call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":false}`, 400)
	linked := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200)
	if linked.State != "linked" || linked.TaskID <= 0 || linked.MessageSeq != 1 {
		t.Fatal("意图未关联唯一任务消息")
	}
	for i := 0; i < 2; i++ {
		if v := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200); v.TaskID != linked.TaskID || v.MessageSeq != 1 {
			t.Fatal("确认重放重复创建")
		}
	}
	balance(60-price, price)
	taskPath := "/api/v1/ai/tasks/" + strconv.FormatInt(linked.TaskID, 10)
	if v := call("GET", taskPath, tokens[0], "", "", 200); v.Status != 1 || v.ReportAvailable {
		t.Fatal("worker启动前任务不是排队状态")
	}
	call("GET", taskPath+"/report", tokens[0], "", "", 422)
	child := startLocalWorkerChild(t, e, worker, localWorkerDSN(t, e), runtimeFile, f.path, executionFile, e.modelSession)
	child.waitReady(t)
	wait := 30 * time.Second
	if external {
		wait = 14 * time.Minute
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		v := call("GET", taskPath, tokens[0], "", "", 200)
		if v.Status == 3 && v.ReportAvailable {
			break
		}
		if v.Status >= 4 {
			t.Fatalf("实际任务未交付：status=%d；不重提、不将失败记为成功", v.Status)
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("API未观察到实际worker交付")
		case err := <-child.done:
			child.waited = true
			t.Fatal("实际worker提前结束", err)
		}
	}
	for i := 0; i < 2; i++ {
		v := call("GET", taskPath+"/report", tokens[0], "", "", 200)
		if v.TaskID != linked.TaskID || len(v.Sections) != 4 {
			t.Fatal("报告任务归属或章节不符")
		}
		quotes := 0
		for _, s := range v.Sections {
			for _, q := range s.Quotes {
				if q.Text == "" || q.SourceURL == "" || q.CollectedAt <= 0 {
					t.Fatal("报告缺少溯源")
				}
				quotes++
			}
		}
		if kind == 2 && quotes < 6 {
			t.Fatal("深度报告少于六引文")
		}
	}
	if v := call("POST", intent+"/confirm", tokens[0], "", `{"confirmed":true}`, 200); v.TaskID != linked.TaskID || v.MessageSeq != 1 {
		t.Fatal("成功后确认重放另建任务")
	}
	if v := call("POST", intent+"/reconcile", tokens[0], "", `{}`, 200); v.TaskID != linked.TaskID {
		t.Fatal("成功后核查未恢复原任务")
	}
	if v := call("GET", root+"/task-intent-receipt", tokens[0], key, "", 200); v.TaskID != linked.TaskID {
		t.Fatal("成功后原键回执未恢复原任务")
	}
	for _, suffix := range []string{"", "/events", "/stream", "/report"} {
		call("GET", taskPath+suffix, tokens[1], "", "", 404)
	}
	call("GET", taskPath+"/report", "", "", "", 401)
	message := call("GET", root+"/messages", tokens[0], "", "", 200)
	if len(message.Items) != 1 || message.Items[0].TaskID != linked.TaskID || message.Items[0].Skill != analysis || message.Items[0].Kind != "task" {
		t.Fatal("会话没有唯一关联消息")
	}
	events := call("GET", taskPath+"/events", tokens[0], "", "", 200)
	if len(events.Items) == 0 {
		t.Fatal("缺少公开事件")
	}
	last := events.Items[len(events.Items)-1].Seq
	if events.Items[len(events.Items)-1].Status != 3 {
		t.Fatal("事件未记录成功终态")
	}
	for _, after := range []int64{0, last - 1, last} {
		r, err := http.NewRequestWithContext(e.ctx, "GET", base+taskPath+"/stream?after_seq="+strconv.FormatInt(after, 10), nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+tokens[0])
		r.Header.Set("Last-Event-ID", strconv.FormatInt(after, 10))
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("终态SSE请求失败")
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 131073))
		res.Body.Close()
		if err != nil || len(raw) > 131072 || res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("终态SSE未正常结束")
		}
		hidden(raw)
		var got []int64
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "id: ") {
				id, err := strconv.ParseInt(strings.TrimPrefix(line, "id: "), 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, id)
			}
		}
		var want []int64
		for _, v := range events.Items {
			if v.Seq > after {
				want = append(want, v.Seq)
			}
		}
		if len(got) != len(want) {
			t.Fatal("SSE续读数量不符")
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatal("SSE续读重复或乱序")
			}
		}
	}
	if feedbackPeer != nil {
		verifyLocalFeedbackProcess(t, f, feedbackPeer, client, base, taskPath, tokens)
	}
	// 重放与重读完成后，让两个队列再扫描两轮，检查原进程不会再提交或扣分。
	timer := time.NewTimer(2200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case err := <-child.done:
		child.waited = true
		t.Fatal("成功后worker提前退出", err)
	}
	if _, err := io.WriteString(child.input, "stop\n"); err != nil {
		t.Fatal(err)
	}
	child.wait(t, true)
	balance(60-price, 0)
	wantAgent, wantModel := int32(1), 3
	if external {
		wantAgent, wantModel = 0, 0
	}
	if e.submits.Load() != wantAgent || len(e.server.Requests()) != wantModel || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts WHERE user_id=2`) != 0 {
		t.Fatal("实际API和worker闭环重复调用、结算或污染其它会员")
	}
	requirePointAudit(t, e.pointsTestEnv, e.ledger)
	if qaFixture != nil {
		verifyLocalProcessQA(t, f, qaFixture, client, base, root, linked.TaskID, tokens)
	}
	if len(cases) == 1 && cases[0].feedbackBrowserRunner != "" {
		if feedbackPeer == nil {
			t.Fatal("缺少本轮实际管理进程")
		}
		s, err := service.NewAIFeedbackService(repository.NewAIFeedbackRepo(e.db))
		if err != nil {
			t.Fatal(err)
		}
		feedbackConsoleBrowser(t, e, s, feedbackPeer.base, feedbackPeer.credential, cases[0].feedbackBrowserRunner, linked.TaskID)
	}
	bridge.finishAs(t, e.ctx, false)
	if external {
		t.Log("TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED")
	} else {
		t.Log("实际API→会话意图→队列worker→报告终检→一次结算及SSE恢复通过；回环替身，非浏览器/真实模型")
	}
}
