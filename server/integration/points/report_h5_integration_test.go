//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅集成测试：把真实本地H5与原真库/示例CN/模型夹具放在同一回环源。
// 主程序不导入本文件，不修改公开执行开关，也不提供生产登录或默认通过控制器。
type h5ReportAction struct {
	kind string
	id   int64
	done chan struct{}
}

type h5ReportQAPolicies struct{}

func (h5ReportQAPolicies) ForPolicy(context.Context, string) (qa.Config, error) {
	return qa.Config{}, qa.ErrUnavailable
}

type h5ReportQAModels struct{ model *provider.OpenAICompat }

func (m h5ReportQAModels) ForPlan(context.Context, qa.Proof) ([3]*provider.OpenAICompat, error) {
	return [3]*provider.OpenAICompat{m.model, m.model, m.model}, nil
}

type h5ReportQAConfig struct{ cfg qa.Config }

func (c h5ReportQAConfig) ForReport(context.Context, qa.ReportEvidence) (qa.Config, error) {
	return c.cfg, nil
}

type h5ReportGateway struct {
	createHook           func(*testing.T, int64)
	e                    *submissionHTTPEnv
	server               *httptest.Server
	actions              chan h5ReportAction
	serial               sync.Mutex
	seen                 map[int64]bool
	creationPosts        int
	explicitReplay       bool
	workerDone           chan struct{}
	workerErr            error // 仅在workerDone关闭之后读取。
	cancelAck            bool
	feedbackAdmin        http.Handler
	feedbackCredential   string
	feedbackPosts        int
	feedbackReads        int
	streamReads          int
	conversationPosts    int
	conversationPatches  int
	conversationArchives int
	intentPrepares       int
	intentConfirms       int
	intentReconciles     int
	intentCreator        *h5IntentCreator
	intentStore          *intentSQLStore
	intentMode           bool
}

// 仅测试：计数原创建核，并让原提交未知夹具绑定实际服务端任务键。
type h5IntentCreator struct {
	*countedIntentCreator
	store *submissionSQLStore
}

func (c *h5IntentCreator) CreateQuoted(ctx context.Context, uid int64, key string, in dto.AITaskCreateInput, expected int64) (dto.AITaskCreation, error) {
	c.store.key = key
	return c.countedIntentCreator.CreateQuoted(ctx, uid, key, in, expected)
}

func h5TestOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errCheckTest
	}
	ip := net.ParseIP(u.Hostname())
	port, err := strconv.Atoi(u.Port())
	if ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 {
		return nil, errCheckTest
	}
	return u, nil
}

func newH5ReportGateway(t *testing.T, e *submissionHTTPEnv, origin string, live ...bool) *h5ReportGateway {
	t.Helper()
	target, err := h5TestOrigin(origin)
	if err != nil {
		t.Fatal("H5测试只能代理显式回环HTTP源")
	}
	// 只补齐本用例的目录表字段，搜索仍执行原handler/service/repository。
	e.mustExec(`ALTER TABLE addon_quant_theme ADD code varchar(20) NOT NULL DEFAULT '', ADD level int NOT NULL DEFAULT 1, ADD description varchar(100) NOT NULL DEFAULT '自动化测试示例，非实际资料', ADD sort int NOT NULL DEFAULT 0, ADD created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3), ADD updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)`)
	if e.ctl.p.CompanyDigest == "" {
		e.mustExec(`ALTER TABLE addon_quant_theme ADD parent_id bigint NOT NULL DEFAULT 0`)
	}
	mux := e.web.server.Config.Handler.(*http.ServeMux)
	handler.NewThemeHandler(service.NewThemeService(repository.NewThemeRepo(e.db), nil, service.QuoteConfig{}), handler.DenyAllAccess{}).Register(mux)
	verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: e.web.key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(e.db))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		t.Fatal(err)
	}
	// 仅此隔离网关装配会话与公开SSE；不改变生产开关，也不自动生成会话任务。
	for _, load := range []func() (string, error){conversationMigration, intentMigration} {
		ddl, err := load()
		if err != nil {
			t.Fatal(err)
		}
		e.mustExec(ddl)
	}
	if err := repository.NewAIConversationTaskSchemaVerifier(e.db).Verify(e.ctx); err != nil {
		t.Fatal(err)
	}
	conversations, err := service.NewAIConversationService(repository.NewAIConversationRepo(e.db))
	if err != nil {
		t.Fatal(err)
	}
	handler.NewAIConversationHandler(conversations).Register(mux, auth)
	streamReads, err := service.NewAITaskReadServiceWithReports(e.repo, e.web.reader)
	if err != nil {
		t.Fatal(err)
	}
	streams, err := service.NewAITaskStreamService(streamReads, sessions)
	if err != nil {
		t.Fatal(err)
	}
	handler.NewAITaskStreamHandler(streams).Register(mux, auth)
	cancel, err := service.NewAITaskCancelService(e.tasks)
	if err != nil {
		t.Fatal(err)
	}
	handler.NewAITaskCancelHandler(cancel).Register(mux, auth)
	g := &h5ReportGateway{e: e, actions: make(chan h5ReportAction, 8), seen: make(map[int64]bool)}
	core, err := service.NewAITaskService(e.store, e.ledger)
	if err != nil {
		t.Fatal(err)
	}
	original, err := service.NewAITaskCreateService(core, e.ctl, true)
	if err != nil {
		t.Fatal(err)
	}
	g.intentCreator = &h5IntentCreator{countedIntentCreator: &countedIntentCreator{AITaskCreateService: original}, store: e.store}
	g.intentStore = &intentSQLStore{AIConversationRepo: repository.NewAIConversationRepo(e.db)}
	g.intentMode = os.Getenv("TM_TEST_AI_H5_INTENTS") == "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_INTENTS") == "local-bind-unknown"
	if os.Getenv("TM_TEST_AI_H5_INTENTS") == "local-bind-unknown" {
		g.intentStore.failState = conversation.Linked
	}
	intentService, err := service.NewAIConversationTaskService(g.intentStore, g.intentCreator, true)
	if err != nil {
		t.Fatal(err)
	}
	handler.NewAIConversationTaskHandler(intentService).Register(mux, auth, true)
	reportDDL(t, e.pointsTestEnv, "20261020_create_tm_ai_report_feedbacks.sql")
	feedbackService, err := service.NewAIFeedbackService(repository.NewAIFeedbackRepo(e.db))
	if err != nil {
		t.Fatal(err)
	}
	feedbackHandler := handler.NewAIFeedbackHandler(feedbackService)
	feedbackHandler.RegisterMember(mux, auth)
	internalMux := http.NewServeMux()
	feedbackHandler.RegisterInternal(internalMux)
	g.feedbackCredential = strings.Repeat("h", 40)
	internalAuth, err := middleware.NewAIAdminAuth([]config.AIAdminReviewer{{ActorID: 7, Token: g.feedbackCredential}})
	if err != nil {
		t.Fatal(err)
	}
	g.feedbackAdmin = internalAuth(internalMux)
	if os.Getenv("TM_TEST_AI_H5_REPORT_QA") == "1" {
		reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_qa.sql")
		e.mustExec("INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,'test-only-budget',0,10,20,1000000,2000000,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))")
		e.mustExec("UPDATE tm_ai_qa_limits SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)")
		usage := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
		qaUpstream := providertest.NewServer(t,
			providertest.Success(`{"action":"answer","reason":"none","citations":[{"snapshot_id":1,"sentence_ids":[1]}]}`, usage),
			providertest.Success(`{"verdict":"pass","reason":"verified"}`, usage),
			providertest.Success(`{"verdict":"pass","reason":"verified"}`, usage),
		)
		qaDirect, err := provider.NewDirect(provider.Config{Enabled: true, Name: "报告追问测试", BaseURL: qaUpstream.URL(), Model: "qa-report-test", APIKey: "test-only", TimeoutMS: 20000}, true)
		if err != nil {
			t.Fatal(err)
		}
		qaModel := qa.Model{Name: "qa-report-test", FilingNo: "test-only-filing", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", BudgetRef: "test-only-budget", ProfileDigest: qaDirect.DirectProfileDigest()}
		qaCfg := qa.Config{Selector: qaModel, Entailment: qaModel, Semantic: qaModel, Limits: qa.Limits{MaxSources: 16, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second}}
		evidence, err := service.NewAIReportQAEvidence(e.web.reader)
		if err != nil {
			t.Fatal(err)
		}
		qaSvc, err := service.NewAIQAServiceWithReports(repository.NewAIQARepo(e.db), h5ReportQAPolicies{}, h5ReportQAModels{qaDirect}, sessions, true, evidence, h5ReportQAConfig{cfg: qaCfg})
		if err != nil {
			t.Fatal(err)
		}
		handler.NewAIQAHandler(qaSvc).Register(mux, auth, true)
	}
	notice := "示例会员、来源、CN和模型；无真实模型调用、真实费用或生产数据。不使用生产账号。"
	if len(live) == 1 && live[0] {
		notice = "示例会员、资料和账本；本地异步桥调用真实社区版V2引擎及付费DeepSeek模型。仅测试积分，不连接生产数据。"
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, ResponseHeaderTimeout: 5 * time.Second}
	t.Cleanup(proxy.Transport.(*http.Transport).CloseIdleConnections)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "本地H5测试源不可用", http.StatusBadGateway)
	}
	tokens, err := json.Marshal(map[string]string{"one": e.web.token(t, 1, "client"), "two": e.web.token(t, 2, "client")})
	if err != nil {
		t.Fatal(err)
	}
	g.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		remote, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !net.ParseIP(remote).IsLoopback() || r.Host != strings.TrimPrefix(g.server.URL, "http://") {
			http.Error(w, "仅限本地测试", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/__test__/" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-test-only-h5'; connect-src 'self'")
			if _, err := fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>本地H5测试会话</title><h1>仅测试：本地隔离会话</h1><p>%s</p><button id="one">进入测试会员一</button><button id="two">切换测试会员二</button><button id="guest">退出测试会话</button><button id="withdraw">撤回测试来源</button><button id="resolve-feedback">处理测试反馈</button><button id="finish">结束并核对测试</button><p id="result"></p><script nonce="test-only-h5">const tokens=%s;for(const id of ['one','two'])document.getElementById(id).onclick=()=>{localStorage.setItem('token',tokens[id]);location.href='/#/pages/ai/index'};document.getElementById('guest').onclick=()=>{localStorage.removeItem('token');location.href='/#/pages/ai/index'};for(const id of ['withdraw','resolve-feedback','finish'])document.getElementById(id).onclick=async()=>{const r=await fetch('/__test__/'+id,{method:'POST'});document.getElementById('result').textContent=r.ok?'测试操作已确认':'测试操作未确认'};</script>`, notice, tokens); err != nil {
				return
			}
			return
		}
		if r.URL.Path == "/__test__/mobile" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-src 'self'; style-src 'nonce-test-only-h5'")
			// 独立iframe给真实H5提供375px布局视口，不伪造产品CSS或冒充实际移动设备。
			if _, err := fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>仅测试：375px H5布局视口</title><style nonce="test-only-h5">body{margin:0;background:#ddd;font:14px sans-serif}p{margin:0;height:40px;width:375px}iframe{display:block;width:375px;height:640px;border:0}</style><p>仅测试：375×640 H5布局视口，非移动设备验收</p><iframe title="375px H5测试" width="375" height="640" src="/#/pages/ai/index"></iframe>`); err != nil {
				return
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/__test__/") {
			kind := strings.TrimPrefix(r.URL.Path, "/__test__/")
			if r.Method != "POST" || (kind != "withdraw" && kind != "finish" && kind != "resolve-feedback") || r.Header.Get("Origin") != g.server.URL || r.ContentLength != 0 || r.URL.RawQuery != "" {
				http.Error(w, "测试控制请求无效", http.StatusForbidden)
				return
			}
			job := h5ReportAction{kind: kind, done: make(chan struct{})}
			select {
			case g.actions <- job:
			case <-r.Context().Done():
				return
			}
			select {
			case <-job.done:
				w.WriteHeader(http.StatusNoContent)
			case <-r.Context().Done():
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// SSE必须直通真实ResponseWriter，不能经Recorder缓存或占住API互斥锁。
			if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/stream") {
				g.serial.Lock()
				g.streamReads++
				g.serial.Unlock()
				mux.ServeHTTP(w, r)
				return
			}
			g.serial.Lock()
			if strings.HasSuffix(r.URL.Path, "/feedback") {
				if r.Method == "POST" {
					g.feedbackPosts++
				}
				if r.Method == "GET" {
					g.feedbackReads++
				}
			}
			if r.Method == "POST" && r.URL.Path == "/api/v1/ai/tasks" {
				g.creationPosts++
			}
			intentPath := strings.HasPrefix(r.URL.Path, "/api/v1/ai/conversations/") && strings.Contains(r.URL.Path, "/task-intent")
			if intentPath && r.Method == "POST" {
				switch {
				case strings.HasSuffix(r.URL.Path, "/task-intents"):
					g.intentPrepares++
				case strings.HasSuffix(r.URL.Path, "/confirm"):
					g.intentConfirms++
				case strings.HasSuffix(r.URL.Path, "/reconcile"):
					g.intentReconciles++
				}
			}
			if e.store.phase.Load() != 0 && !e.store.once.Load() && r.Method == "POST" && r.URL.Path == "/api/v1/ai/tasks" {
				// 故障注入绑定浏览器实际原键；不改请求头、不制造或替换业务键。
				e.store.key = r.Header.Get("Idempotency-Key")
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, r)
			if rec.Code == 200 && r.Method == "POST" && r.URL.Path == "/api/v1/ai/conversations" {
				g.conversationPosts++
			}
			if rec.Code == 200 && r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/api/v1/ai/conversations/") {
				g.conversationPatches++
			}
			if rec.Code == 200 && r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v1/ai/conversations/") && strings.HasSuffix(r.URL.Path, "/archive") {
				g.conversationArchives++
			}
			if g.workerDone != nil && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/cancel") && rec.Code == 200 {
				var out struct {
					Data dto.AITaskCancellation `json:"data"`
				}
				if json.Unmarshal(rec.Body.Bytes(), &out) == nil && out.Data.ID == e.v.ID && out.Data.Status == task.Running && out.Data.CancelRequested && !out.Data.ReportAvailable {
					g.cancelAck = true
				}
			}
			var created int64
			if rec.Code == 200 && intentPath {
				var result struct {
					Data dto.AIConversationTask `json:"data"`
				}
				if json.Unmarshal(rec.Body.Bytes(), &result) == nil && result.Data.State == conversation.Linked && result.Data.TaskID > 0 && !g.seen[result.Data.TaskID] {
					created = result.Data.TaskID
					g.seen[created] = true
				}
			}
			if rec.Code == 200 && ((r.Method == "POST" && r.URL.Path == "/api/v1/ai/tasks") || (r.Method == "GET" && r.URL.Path == "/api/v1/ai/task-receipt")) {
				var result struct {
					Data dto.AITaskCreation `json:"data"`
				}
				if json.Unmarshal(rec.Body.Bytes(), &result) == nil && result.Data.TaskID > 0 && !g.seen[result.Data.TaskID] {
					created = result.Data.TaskID
					g.seen[created] = true
				}
			}
			g.serial.Unlock()
			for key, values := range rec.Header() {
				w.Header()[key] = values
			}
			w.WriteHeader(rec.Code)
			if _, err := w.Write(rec.Body.Bytes()); err != nil {
				return
			}
			if created > 0 {
				select {
				case g.actions <- h5ReportAction{kind: "create", id: created}:
				case <-e.ctx.Done():
				}
			}
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	g.server.Config.ReadHeaderTimeout = time.Second
	g.server.Config.WriteTimeout = 10 * time.Second
	g.server.Start()
	t.Cleanup(g.server.Close)
	return g
}

// 只让测试CN停在运行状态；执行仍由原L1 worker驱动，API互斥锁不得包住整个执行。
func (g *h5ReportGateway) startRunning(t *testing.T) {
	t.Helper()
	w := g.e.newWorker(t)
	ctx, stop := context.WithCancel(g.e.ctx)
	g.workerDone = make(chan struct{})
	go func() {
		defer close(g.workerDone)
		_, g.workerErr = w.Run(ctx, 1, g.e.v.ID)
	}()
	t.Cleanup(func() {
		stop()
		select {
		case <-g.workerDone:
		case <-time.After(10 * time.Second):
			t.Error("H5测试worker未在取消上下文后退出")
		}
	})
	wait, cancel := context.WithTimeout(g.e.ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if g.e.agentPolls.Load() > 0 {
			r, err := g.e.agent.Show(wait, g.e.planner.p.Agent.ActorID, g.e.planner.p.Agent.RequestID())
			if err == nil && r.State == agents.Running {
				return
			}
		}
		select {
		case <-g.workerDone:
			t.Fatal("H5测试worker在Agent运行前已退出", g.workerErr)
		case <-wait.Done():
			t.Fatal("H5测试未观察到实际Agent运行")
		case <-ticker.C:
		}
	}
}

func (g *h5ReportGateway) process(t *testing.T, job h5ReportAction, mode string) bool {
	t.Helper()
	g.serial.Lock()
	defer g.serial.Unlock()
	if job.done != nil {
		defer close(job.done)
	}
	switch job.kind {
	case "create":
		g.e.bindNewTask(t, job.id)
		if g.createHook != nil {
			g.createHook(t, job.id)
			return true
		}
		if g.e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
			t.Fatal("H5创建未冻结原锁价")
		}
		if mode == "success" || mode == "library" || mode == "feedback" {
			g.e.run(t)
		}
		if mode == "running_cancel" {
			g.startRunning(t)
		}
	case "resolve-feedback":
		id := g.e.queryInt("SELECT id FROM tm_ai_report_feedbacks")
		for _, body := range []string{`{"expectedVersion":1,"status":"processing","resolution":""}`, `{"expectedVersion":2,"status":"resolved","resolution":"reviewed"}`} {
			r := httptest.NewRequest("POST", "/internal/v1/ai-feedback/"+strconv.Itoa(id)+"/decisions", strings.NewReader(body))
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set("Authorization", "Bearer "+g.feedbackCredential)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			g.feedbackAdmin.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatal("测试内部处理未成功", w.Code)
			}
		}
	case "withdraw":
		if err := repository.NewAISourceRepo(g.e.db).SetSourceEnabled(g.e.ctx, g.e.source.ID, false); err != nil {
			t.Fatal(err)
		}
	case "finish":
		if mode == "running_cancel" {
			if g.workerDone == nil || !g.cancelAck {
				t.Fatal("H5缺少真实运行和已受理但非终态的取消回执")
			}
			select {
			case <-g.workerDone:
				if g.workerErr != nil {
					t.Fatal("H5取消worker失败", g.workerErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("H5取消后worker未退出")
			}
			r, err := g.e.agent.Show(g.e.ctx, g.e.planner.p.Agent.ActorID, g.e.planner.p.Agent.RequestID())
			if err != nil || r.State != agents.Running {
				t.Fatal("本地取消被伪装成远端已经停止", err)
			}
		}
		if g.e.v.ID <= 1 {
			t.Fatal("H5未创建真实测试任务")
		}
		wantPosts := 1
		if g.intentMode {
			wantPosts = 0
		}
		if g.explicitReplay {
			wantPosts++
		}
		if g.creationPosts != wantPosts {
			t.Fatal("H5创建请求被自动重发或计数不符", g.creationPosts, wantPosts)
		}
		want := task.Succeeded
		calls, models := int32(1), 3
		settlements := 1
		if mode == "cancel" || mode == "receipt_unknown" {
			want, calls, models = task.Cancelled, 0, 0
			settlements = 0
		}
		if mode == "running_cancel" {
			want, calls, models, settlements = task.Cancelled, 1, 0, 0
		}
		v, err := g.e.tasks.Inspect(g.e.ctx, 1, g.e.v.ID)
		if err != nil || v.Status != want || g.e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || g.e.submits.Load() != calls || len(g.e.server.Requests()) != models {
			t.Fatal("H5结束时任务/冻结/调用不符", err)
		}
		if g.e.queryInt("SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'") != settlements || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_report_commits WHERE status=3") != settlements {
			t.Fatal("H5报告与成功结算未恰好一次，或取消仍交付")
		}
		requirePointAudit(t, g.e.pointsTestEnv, g.e.ledger)
		if g.intentMode {
			if g.intentPrepares != 1 || g.intentConfirms != 1 || g.intentCreator.calls.Load() != 1 || g.conversationPosts != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_tasks WHERE user_id=1 AND state='linked'") != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=1 AND kind='task'") != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=2") != 0 {
				t.Fatal("H5会话意图未单次关联原任务", g.intentPrepares, g.intentConfirms, g.intentCreator.calls.Load())
			}
			t.Log("实际会话准备/确认/原创建核各1次，唯一原任务引用、报告与测试账务按原流程验收；未走独立创建POST")
		}
		if os.Getenv("TM_TEST_AI_H5_ASSISTANT") == "local-fixture-only" {
			if g.streamReads < 2 || g.conversationPosts != 1 || g.conversationPatches != 1 || g.conversationArchives != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=1 AND archived=1 AND version=3 AND last_seq=0") != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_conversations WHERE user_id=2") != 0 {
				t.Fatal("会话浏览器操作或SSE续读未完整验收", g.streamReads, g.conversationPosts, g.conversationPatches, g.conversationArchives)
			}
			t.Log("会话浏览器新建/改名/归档各1次；SSE至少2次，未重复创建任务；测试账本审计通过")
		}
		if mode == "feedback" && (g.feedbackPosts != 1 || g.feedbackReads < 2 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedbacks WHERE status='resolved' AND version=3") != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_report_feedback_events") != 3) {
			t.Fatal("H5反馈受理、查询、处理审计未闭环")
		}
		if os.Getenv("TM_TEST_AI_H5_REPORT_QA") == "1" {
			if g.e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE skill='report_qa' AND task_id="+strconv.FormatInt(g.e.v.ID, 10)) != 1 || g.e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_calls WHERE state='recorded'") != 3 {
				t.Fatal("H5报告追问未恰好执行一次或未绑定原报告")
			}
		}
		return true
	default:
		t.Fatal("未知H5测试动作")
	}
	return false
}

func h5GatewayRequest(t *testing.T, g *h5ReportGateway, method, path, token, key, body string, status int) []byte {
	t.Helper()
	r, err := http.NewRequestWithContext(g.e.ctx, method, g.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Idempotency-Key", key)
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(r)
	if err != nil {
		t.Fatal("H5测试请求失败", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil || res.StatusCode != status {
		t.Fatal("H5测试响应不符", res.StatusCode, err)
	}
	return b
}

func TestIntegrationAIReportH5Gateway(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"success", "cancel", "library", "receipt_unknown", "running_cancel", "feedback"} {
				t.Run(mode, func(t *testing.T) {
					fixtureMode := "success"
					if mode == "receipt_unknown" {
						fixtureMode = "unknown"
					}
					if mode == "running_cancel" {
						fixtureMode = "agent_running"
					}
					e := newSubmissionHTTPEnvWithPlan(t, level, fixtureMode, func(e *submissionHTTPEnv) {
						if mode == "library" {
							prepareCompanyReportPlan(t, e, 0)
							prepareLibraryReportPlan(t, e, "pass")
						}
					})
					front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						if _, err := fmt.Fprint(w, "仅测试前端代理，不是真实H5验收"); err != nil {
							return
						}
					}))
					t.Cleanup(front.Close)
					origin := os.Getenv("TM_TEST_AI_H5_ORIGIN")
					interactive := origin != ""
					if !interactive {
						origin = front.URL
					} else {
						// 人工浏览器会话独立有界；不改变自动化120秒夹具、任务300秒预算或任何服务调用时限。
						browserContext, stop := context.WithTimeout(context.Background(), 10*time.Minute)
						t.Cleanup(stop)
						e.pointsTestEnv.ctx = browserContext
					}
					g := newH5ReportGateway(t, e, origin)
					if interactive {
						t.Log("实际H5测试入口（示例数据，交互会话最多10分钟，任务预算不变，不是公开服务）：", g.server.URL+"/__test__/")
						for {
							select {
							case job := <-g.actions:
								if g.process(t, job, mode) {
									return
								}
							case <-e.ctx.Done():
								t.Fatal("实际H5未在夹具期限内确认闭环")
							}
						}
					}
					page := h5GatewayRequest(t, g, "GET", "/__test__/", "", "", "", 200)
					if !strings.Contains(string(page), "仅测试") {
						t.Fatal("测试会话缺少显式标识")
					}
					mobile := h5GatewayRequest(t, g, "GET", "/__test__/mobile", "", "", "", 200)
					if !strings.Contains(string(mobile), `width="375"`) || !strings.Contains(string(mobile), "非移动设备验收") {
						t.Fatal("H5窄布局测试缺视口或边界说明")
					}
					h5GatewayRequest(t, g, "POST", "/__test__/withdraw", "", "", "", 403)
					h5GatewayRequest(t, g, "GET", "/__test__/finish", "", "", "", 403)
					h5GatewayRequest(t, g, "GET", "/", "", "", "", 200)
					token := e.web.token(t, 1, "client")
					foreign := e.web.token(t, 2, "client")
					h5GatewayRequest(t, g, "GET", "/api/v1/theme/search?kw="+url.QueryEscape("示例"), "", "", "", 200)
					h5GatewayRequest(t, g, "POST", "/api/v1/ai/tasks/quote", token, "", sqlSubmissionQuote, 200)
					var created dto.AITaskCreation
					if mode == "receipt_unknown" {
						body := h5GatewayRequest(t, g, "POST", "/api/v1/ai/tasks", token, e.key, sqlSubmissionBody, 503)
						if strings.Contains(string(body), "taskId") || len(g.actions) != 0 || e.newTaskCount(t) != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
							t.Fatal("创建未知暴露收据、丢失原冻结或触发执行")
						}
						created = decodeCreation(t, h5GatewayRequest(t, g, "GET", "/api/v1/ai/task-receipt", token, e.key, "", 200))
					} else {
						created = decodeCreation(t, h5GatewayRequest(t, g, "POST", "/api/v1/ai/tasks", token, e.key, sqlSubmissionBody, 200))
					}
					select {
					case job := <-g.actions:
						g.process(t, job, mode)
					case <-e.ctx.Done():
						t.Fatal("创建未进入原编排")
					}
					path := "/api/v1/ai/tasks/" + strconv.FormatInt(created.TaskID, 10)
					h5GatewayRequest(t, g, "GET", path, foreign, "", "", 404)
					if mode == "success" || mode == "library" || mode == "feedback" {
						body := h5GatewayRequest(t, g, "GET", path+"/report", token, "", "", 200)
						if mode == "library" {
							assertH5LibraryReport(t, body, e.reportEnv)
						} else {
							assertPublicReport(t, body, e.reportEnv)
						}
						g.process(t, h5ReportAction{kind: "withdraw"}, mode)
						h5GatewayRequest(t, g, "GET", path+"/report", token, "", "", 422)
						if mode == "feedback" {
							h5GatewayRequest(t, g, "GET", path+"/feedback", token, "", "", 200)
							h5GatewayRequest(t, g, "POST", path+"/feedback", token, "", `{"category":"fact"}`, 200)
							g.process(t, h5ReportAction{kind: "resolve-feedback"}, mode)
							body := h5GatewayRequest(t, g, "GET", path+"/feedback", token, "", "", 200)
							if !strings.Contains(string(body), `"status":"resolved"`) {
								t.Fatal("H5撤回后不能查询反馈终态")
							}
						}
					} else {
						if mode == "running_cancel" {
							v, err := e.tasks.Inspect(e.ctx, 1, created.TaskID)
							if err != nil || v.Status != task.Running || e.submits.Load() != 1 || e.server.Count() != 0 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
								t.Fatal("H5运行中取消必须先由真实worker提交测试Agent并保留冻结", err)
							}
							var progress struct {
								Data dto.AITaskProgress `json:"data"`
							}
							if json.Unmarshal(h5GatewayRequest(t, g, "GET", path, token, "", "", 200), &progress) != nil || progress.Data.ID != created.TaskID || progress.Data.Status != task.Running || progress.Data.CancelRequested || progress.Data.ReportAvailable {
								t.Fatal("H5未读取到真实运行中且未请求取消的任务")
							}
							h5GatewayRequest(t, g, "POST", path+"/cancel", foreign, "", "{}", 404)
						}
						h5GatewayRequest(t, g, "POST", path+"/cancel", token, "", "{}", 200)
						if mode == "running_cancel" {
							select {
							case <-g.workerDone:
								if g.workerErr != nil {
									t.Fatal("运行中取消worker失败", g.workerErr)
								}
							case <-time.After(5 * time.Second):
								t.Fatal("运行中取消未由原worker终结")
							}
							var progress struct {
								Data dto.AITaskProgress `json:"data"`
							}
							if json.Unmarshal(h5GatewayRequest(t, g, "GET", path, token, "", "", 200), &progress) != nil || progress.Data.Status != task.Cancelled || !progress.Data.CancelRequested || progress.Data.ReportAvailable {
								t.Fatal("H5取消后独立查询未确认原终态")
							}
							before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
							e.run(t)
							if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
								t.Fatal("取消终态worker重放再次写账")
							}
						}
						h5GatewayRequest(t, g, "GET", path+"/report", token, "", "", 422)
					}
					before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
					g.explicitReplay = true // 自动化显式验证原键重放；人工会话/重载不得补发POST。
					if replay := decodeCreation(t, h5GatewayRequest(t, g, "POST", "/api/v1/ai/tasks", token, e.key, sqlSubmissionBody, 200)); replay != created {
						t.Fatal("H5原键回执改变")
					}
					if len(g.actions) != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
						t.Fatal("H5重放重复执行或改账")
					}
					g.process(t, h5ReportAction{kind: "finish"}, mode)
				})
			}
		})
	}
}

func TestIntegrationAIReportH5Conversation(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"success", "bind_unknown"} {
				t.Run(mode, func(t *testing.T) {
					e := newSubmissionHTTPEnv(t, level, "success")
					front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						fmt.Fprint(w, "仅自动化代理占位，不是H5界面验收")
					}))
					t.Cleanup(front.Close)
					g := newH5ReportGateway(t, e, front.URL)
					g.intentMode = true
					if mode == "bind_unknown" {
						g.intentStore.failState = conversation.Linked
					}
					token, other := e.web.token(t, 1, "client"), e.web.token(t, 2, "client")
					var created struct {
						Data dto.AIConversationReceipt `json:"data"`
					}
					if json.Unmarshal(h5GatewayRequest(t, g, "POST", "/api/v1/ai/conversations", token, "h5_conversation_test_original", `{"title":"隔离会话任务验收"}`, 200), &created) != nil || created.Data.ID < 1 {
						t.Fatal("会话创建失败")
					}
					base := fmt.Sprintf("/api/v1/ai/conversations/%d", created.Data.ID)
					var prepared struct {
						Data dto.AIConversationTask `json:"data"`
					}
					if json.Unmarshal(h5GatewayRequest(t, g, "POST", base+"/task-intents", token, "h5_intent_test_original", sqlSubmissionBody, 200), &prepared) != nil || prepared.Data.State != conversation.Prepared || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || g.intentCreator.calls.Load() != 0 {
						t.Fatal("意图准备提前创建或冻结")
					}
					path := fmt.Sprintf("%s/task-intents/%d", base, prepared.Data.ID)
					h5GatewayRequest(t, g, "GET", path, other, "", "", 404)
					status := 200
					if mode == "bind_unknown" {
						status = 503
					}
					h5GatewayRequest(t, g, "POST", path+"/confirm", token, "", `{"confirmed":true}`, status)
					if mode == "bind_unknown" {
						if len(g.actions) != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks t JOIN tm_ai_conversation_tasks i ON t.idem_key=i.task_key AND t.user_id=i.user_id WHERE i.user_id=1") != 1 || e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
							t.Fatal("绑定回滚未保留唯一原任务/冻结")
						}
						var receipt struct {
							Data dto.AIConversationTask `json:"data"`
						}
						if json.Unmarshal(h5GatewayRequest(t, g, "GET", base+"/task-intent-receipt", token, "h5_intent_test_original", "", 200), &receipt) != nil || receipt.Data.State != conversation.Unconfirmed || len(g.actions) != 0 {
							t.Fatal("只读意图收据补造关联或创建")
						}
						h5GatewayRequest(t, g, "POST", path+"/reconcile", token, "", "{}", 200)
					}
					select {
					case action := <-g.actions:
						processMode := "success"
						if mode == "bind_unknown" {
							processMode = "cancel"
						}
						g.process(t, action, processMode)
					case <-e.ctx.Done():
						t.Fatal("会话未关联原任务")
					}
					taskPath := fmt.Sprintf("/api/v1/ai/tasks/%d", e.v.ID)
					finishMode := "success"
					if mode == "bind_unknown" {
						finishMode = "cancel"
						h5GatewayRequest(t, g, "POST", taskPath+"/cancel", token, "", "{}", 200)
						h5GatewayRequest(t, g, "GET", taskPath+"/report", token, "", "", 422)
					} else {
						assertPublicReport(t, h5GatewayRequest(t, g, "GET", taskPath+"/report", token, "", "", 200), e.reportEnv)
					}
					before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
					for n := 0; n < 2; n++ {
						h5GatewayRequest(t, g, "GET", path, token, "", "", 200)
						h5GatewayRequest(t, g, "GET", base+"/messages?after_seq=0&limit=20", token, "", "", 200)
					}
					if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before || len(g.actions) != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_tasks t JOIN tm_ai_conversation_tasks i ON t.idem_key=i.task_key AND t.user_id=i.user_id WHERE i.user_id=1") != 1 {
						t.Fatal("意图/消息重读重复写账或执行")
					}
					g.process(t, h5ReportAction{kind: "finish"}, finishMode)
				})
			}
		})
	}
}

func assertH5LibraryReport(t *testing.T, body []byte, e *reportEnv) {
	t.Helper()
	var got struct {
		Code int          `json:"code"`
		Data dto.AIReport `json:"data"`
	}
	if json.Unmarshal(body, &got) != nil || got.Code != 0 || got.Data.TaskID != e.v.ID || !got.Data.Disclosure.AIGenerated || len(got.Data.Sections) != 4 {
		t.Fatal("H5资料库报告契约不符")
	}
	for i, section := range got.Data.Sections[:3] {
		if !section.AIGenerated || len(section.Quotes) != 1 || section.Quotes[0].Text != e.snap.Sentences[i].Text {
			t.Fatal("H5资料库报告引文未回取")
		}
	}
	section := got.Data.Sections[3]
	if section.AIGenerated || len(section.Quotes) != 0 || len(section.Companies) != 2 || section.Companies[0].Code != "600001" || section.Companies[1].Code != "600002" {
		t.Fatal("H5公司事实冒充AI、丢行或未按代码排序")
	}
	for _, company := range section.Companies {
		if len(company.Evidence) != 1 || !company.Evidence[0].Trimmed || company.Evidence[0].SourceURL != e.snap.URL || company.Evidence[0].CollectedAt != e.snap.FetchedAt.UnixMilli() {
			t.Fatal("H5公司事实出处或节选标记丢失")
		}
	}
	for _, private := range []string{"test-only-agent-untrusted-marker", "ReviewerID", "LicenseRef", "CompanyDigest", `"change"`} {
		if strings.Contains(string(body), private) {
			t.Fatal("H5公司报告泄露内部或行情字段")
		}
	}
}

func TestH5TestOrigin(t *testing.T) {
	for _, raw := range []string{"https://127.0.0.1:5174", "http://example.test:5174", "http://localhost:5174", "http://127.0.0.1:0", "http://127.0.0.1:5174/other", "http://user@127.0.0.1:5174", "http://127.0.0.1:5174/?q=1"} {
		if _, err := h5TestOrigin(raw); err == nil {
			t.Fatal("非闭集H5测试源被接受")
		}
	}
	if _, err := h5TestOrigin("http://127.0.0.1:5174"); err != nil {
		t.Fatal(err)
	}
}
