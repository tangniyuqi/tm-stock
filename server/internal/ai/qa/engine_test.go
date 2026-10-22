package qa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const selectedJSON = `{"action":"answer","reason":"none","citations":[{"snapshot_id":7,"sentence_ids":[1]}]}`
const passedJSON = `{"verdict":"pass","reason":"verified"}`

var testUsage = provider.Usage{Prompt: 20, Completion: 30, Total: 50}

// 仅测试执行核：此内存记录器不作为持久配额、来源授权或生产装配的证明。
type testControl struct {
	mu             sync.Mutex
	begun, revoked bool
	fail           string
	checks         int
	revokeAt       int
	starts         []Call
	results        []CallResult
	receipts       []Receipt
	onStart        func()
	onComplete     func()
}

func (c *testControl) Begin(_ context.Context, _ Proof) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail == "begin" || c.begun {
		return ErrUnavailable
	}
	c.begun = true
	return nil
}
func (c *testControl) Recheck(_ context.Context, _ Proof) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	if c.fail == "recheck" || c.revoked || c.revokeAt > 0 && c.checks >= c.revokeAt {
		return ErrUnavailable
	}
	return nil
}
func (c *testControl) StartCall(_ context.Context, v Call) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail == "start" || !c.begun || v.Step != len(c.starts)+1 || len(c.starts) >= 3 {
		return ErrUnavailable
	}
	c.starts = append(c.starts, v)
	if c.onStart != nil {
		c.onStart()
	}
	return nil
}
func (c *testControl) FinishCall(_ context.Context, _ Call, r CallResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail == "finish" {
		return ErrUnavailable
	}
	c.results = append(c.results, r)
	return nil
}
func (c *testControl) Complete(_ context.Context, _ Proof, r Receipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail == "complete" {
		return ErrUnavailable
	}
	c.receipts = append(c.receipts, r)
	if c.onComplete != nil {
		c.onComplete()
	}
	return nil
}
func (c *testControl) revoke() { c.mu.Lock(); defer c.mu.Unlock(); c.revoked = true }

func successfulSteps() []providertest.Step {
	return []providertest.Step{providertest.Success(selectedJSON, testUsage), providertest.Success(passedJSON, testUsage), providertest.Success(passedJSON, testUsage)}
}

func fixture(t *testing.T, steps ...providertest.Step) (*Engine, *Plan, *testControl, *providertest.Server) {
	t.Helper()
	srv := providertest.NewServer(t, steps...)
	direct, err := provider.NewDirect(provider.Config{Enabled: true, Name: "问答测试", BaseURL: srv.URL(), Model: "qa-test-model", APIKey: "test-only", TimeoutMS: 3000}, true)
	if err != nil {
		t.Fatal(err)
	}
	c := &testControl{}
	e, err := NewEngine(direct, direct, direct, c)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	expires := now.Add(time.Hour)
	s, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试授权", LicenseRef: "测试存档", LicenseExpires: &expires, Enabled: true}, "https://example.test/qa", sources.MediaPlain, []byte("示例资料：该目录包含测试设备分类。第二条列明示例设备的部件名称。第三条列明测试发布日期。"), now, 16384)
	if err != nil {
		t.Fatal(err)
	}
	s.ID = 7
	m := Model{Name: "qa-test-model", FilingNo: "测试备案", ApprovalRef: "测试审批", QualityRef: "测试质量记录", BudgetRef: "测试预算", ProfileDigest: direct.DirectProfileDigest()}
	cfg := Config{Selector: m, Entailment: m, Semantic: m, AllowedDomains: []string{"example.test"}, Limits: Limits{MaxSources: 4, MaxSentences: 32, MaxEvidenceBytes: 16384, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 3 * time.Second}}
	in := conversation.MessageInput{Kind: "question", Skill: "material_qa", SubjectType: "theme", SubjectID: 8, Question: "示例目录包含什么分类？"}
	p, err := NewPlan(Identity{11, 12, 1}, in, Scope{SubjectType: "theme", SubjectID: 8, SubjectName: "示例分类", ApprovalRef: "仅测试范围", ValidUntil: now.Add(30 * time.Minute), Snapshots: []sources.Snapshot{s}}, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	return e, p, c, srv
}

func TestEngineLiteralAnswerAndAudit(t *testing.T) {
	e, p, c, srv := fixture(t, successfulSteps()...)
	v, err := e.Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.View(context.Background(), p, v)
	if err != nil {
		t.Fatal(err)
	}
	s := p.scope.Snapshots[0]
	if out.State != "answered" || out.ModelName != p.cfg.Selector.Name || out.FilingNo != p.cfg.Selector.FilingNo || len(out.Quotes) != 1 {
		t.Fatalf("交付结构错误：%+v", out)
	}
	q := out.Quotes[0]
	if q.Text != s.Sentences[0].Text || q.Text != s.Body[q.Start:q.End] || q.SourceURL != s.URL || q.SourceType != s.SourceCategory || q.CollectedAt != s.FetchedAt.UnixMilli() {
		t.Fatal("未逐字回取和追溯")
	}
	if srv.Count() != 3 || len(c.starts) != 3 || len(c.results) != 3 || len(c.receipts) != 1 || c.receipts[0].Calls != 3 {
		t.Fatal("调用/收据未一一对应")
	}
	for n, v := range c.starts {
		if v.InputDigest != p.digest || v.Step != n+1 || !digest(v.RequestDigest) || v.MaxTokens != 13024 || v.ProfileDigest != p.cfg.Selector.ProfileDigest {
			t.Fatal("预约未绑定配置及最坏预算")
		}
		if c.results[n].Usage != testUsage || c.results[n].Estimated || c.results[n].Failure != "" || !digest(c.results[n].ResponseDigest) {
			t.Fatal("实际用量存证失真")
		}
	}
	for n, request := range srv.Requests() {
		var body struct {
			Model          string `json:"model"`
			MaxTokens      int    `json:"max_tokens"`
			Messages       []struct{ Role, Content string }
			ResponseFormat struct{ Type string } `json:"response_format"`
		}
		if json.Unmarshal(request.Body, &body) != nil || body.Model != "qa-test-model" || body.MaxTokens != 1024 || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" || body.ResponseFormat.Type != "json_schema" {
			t.Fatal("未使用受限结构化调用")
		}
		var prompt struct {
			Data promptData `json:"data"`
		}
		if json.Unmarshal([]byte(body.Messages[1].Content), &prompt) != nil || prompt.Data.Question != p.input.Question || len(prompt.Data.Sources) != 1 || len(prompt.Data.Sources[0].Sentences) != len(s.Sentences) {
			t.Fatal("未发送完整限定候选与原问题")
		}
		if n > 0 && len(prompt.Data.Picks) != 1 {
			t.Fatal("独立核验未收到编号")
		}
		for _, hidden := range []string{s.URL, s.LicenseRef, p.scope.ApprovalRef, "conversation_id", "user_id", "api_key"} {
			if strings.Contains(body.Messages[1].Content, hidden) {
				t.Fatal("提示词泄露内部授权或身份")
			}
		}
	}
	if _, err := e.Run(context.Background(), p); err == nil || srv.Count() != 3 {
		t.Fatal("同一问题自动重复调用")
	}
	// 可变交付副本不能改变已核验结果。
	out.Quotes[0].Text = "改写"
	again, err := e.View(context.Background(), p, v)
	if err != nil || again.Quotes[0].Text != q.Text {
		t.Fatal("交付副本可污染结果")
	}
	c.revoke()
	if out, err := e.View(context.Background(), p, v); err == nil || len(out.Quotes) != 0 {
		t.Fatal("撤权后仍交付缓存")
	}
}

func TestEngineClosedRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Plan)
		steps  []providertest.Step
		calls  int
		reason string
	}{
		{"空资料", func(p *Plan) { p.scope.Snapshots = nil }, nil, 0, "no_evidence"},
		{"红线问题", func(p *Plan) { p.input.Question = "请告诉我买卖点" }, nil, 0, "request_not_allowed"},
		{"无证据", nil, []providertest.Step{providertest.Success(`{"action":"refuse","reason":"no_evidence","citations":[]}`, testUsage)}, 1, "no_evidence"},
		{"超范围", nil, []providertest.Step{providertest.Success(`{"action":"refuse","reason":"out_of_scope","citations":[]}`, testUsage)}, 1, "out_of_scope"},
		{"禁止问题", nil, []providertest.Step{providertest.Success(`{"action":"refuse","reason":"request_not_allowed","citations":[]}`, testUsage)}, 1, "request_not_allowed"},
		{"相关性不确定", nil, []providertest.Step{providertest.Success(selectedJSON, testUsage), providertest.Success(`{"verdict":"uncertain","reason":"uncertain"}`, testUsage)}, 2, "unverified"},
		{"相关性不支持", nil, []providertest.Step{providertest.Success(selectedJSON, testUsage), providertest.Success(`{"verdict":"block","reason":"unsupported"}`, testUsage)}, 2, "unverified"},
		{"语义拒绝", nil, []providertest.Step{providertest.Success(selectedJSON, testUsage), providertest.Success(passedJSON, testUsage), providertest.Success(`{"verdict":"block","reason":"request_not_allowed"}`, testUsage)}, 3, "request_not_allowed"},
		{"独立核验超范围", nil, []providertest.Step{providertest.Success(selectedJSON, testUsage), providertest.Success(`{"verdict":"block","reason":"out_of_scope"}`, testUsage)}, 2, "out_of_scope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p, c, srv := fixture(t, tc.steps...)
			if tc.change != nil {
				tc.change(p)
			}
			v, err := e.Run(context.Background(), p)
			if err != nil {
				t.Fatal(err)
			}
			out, err := e.View(context.Background(), p, v)
			if err != nil || out.State != "refused" || out.Template != refusalTemplate(tc.reason) || len(out.Quotes) != 0 || out.ModelName != "" || srv.Count() != tc.calls || len(c.receipts) != 1 || c.receipts[0].Calls != tc.calls {
				t.Fatalf("拒答错误：%+v %v", out, err)
			}
		})
	}
}

func TestEngineFailuresNeverPublishOrRetry(t *testing.T) {
	for _, stage := range []string{"begin", "recheck", "start", "finish", "complete"} {
		t.Run(stage, func(t *testing.T) {
			e, p, c, srv := fixture(t, successfulSteps()...)
			c.fail = stage
			v, err := e.Run(context.Background(), p)
			if err == nil || v != nil {
				t.Fatal("存证/许可失败仍签发结果")
			}
			want := 0
			if stage == "finish" {
				want = 1
			}
			if stage == "complete" {
				want = 3
			}
			if srv.Count() != want {
				t.Fatalf("调用数 %d != %d", srv.Count(), want)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		step    providertest.Step
		failure string
	}{
		{"限流", providertest.RateLimited("1"), "response_unknown"},
		{"断连", providertest.Drop(), "response_unknown"},
		{"重定向", providertest.Redirect(302, "https://example.test/not-called"), "response_unknown"},
		{"缺少用量", providertest.SuccessNoUsage(selectedJSON), "invalid_metadata"},
		{"用量不一致", providertest.Success(selectedJSON, provider.Usage{Prompt: 20, Completion: 30, Total: 99}), "invalid_metadata"},
		{"输出超预算", providertest.Success(selectedJSON, provider.Usage{Prompt: 20, Completion: 1025, Total: 1045}), "invalid_metadata"},
		{"畸形响应", providertest.Malformed(providertest.TruncatedJSON), "response_unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p, c, srv := fixture(t, tc.step)
			v, err := e.Run(context.Background(), p)
			if err == nil || v != nil || srv.Count() != 1 || len(c.results) != 1 || c.results[0].Failure != tc.failure || len(c.receipts) != 0 {
				t.Fatalf("异常未关闭：%v %+v", err, c.results)
			}
			if _, err = e.Run(context.Background(), p); err == nil || srv.Count() != 1 {
				t.Fatal("未知调用被自动重发")
			}
		})
	}
}

func TestEngineCurrentAuthorizationAtEveryBoundary(t *testing.T) {
	// 每个边界都逐次撤权；这里只验证控制器被执行核调用，不冒充真实授权仓储。
	for cut := 1; cut <= 12; cut++ {
		t.Run(fmt.Sprint(cut), func(t *testing.T) {
			e, p, c, _ := fixture(t, successfulSteps()...)
			c.revokeAt = cut
			v, err := e.Run(context.Background(), p)
			if err == nil || v != nil {
				t.Fatal("中途撤权仍签发")
			}
			if len(c.results) > 0 && c.results[len(c.results)-1].Failure == "not_sent" && len(c.starts) != len(c.results) {
				t.Fatal("未发送预约未配对")
			}
		})
	}
	t.Run("完成存证后撤权", func(t *testing.T) {
		e, p, c, _ := fixture(t, successfulSteps()...)
		c.onComplete = func() { c.revoked = true }
		if v, err := e.Run(context.Background(), p); err == nil || v != nil || len(c.receipts) != 1 {
			t.Fatal("完成后的撤权未拦截")
		}
	})
	t.Run("预约后撤权且审计未知", func(t *testing.T) {
		e, p, c, srv := fixture(t)
		c.onStart = func() { c.revoked = true; c.fail = "finish" }
		if v, err := e.Run(context.Background(), p); err == nil || v != nil || srv.Count() != 0 {
			t.Fatal("预约后撤权仍发送")
		}
	})
}

func TestEngineCancellationAndDeadline(t *testing.T) {
	t.Run("调用前取消", func(t *testing.T) {
		e, p, c, srv := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if v, err := e.Run(ctx, p); !errors.Is(err, context.Canceled) || v != nil || c.begun || srv.Count() != 0 {
			t.Fatal("取消后仍执行")
		}
	})
	t.Run("在途取消仅一次", func(t *testing.T) {
		e, p, c, srv := fixture(t, providertest.Hang())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { _, err := e.Run(ctx, p); done <- err }()
		if !srv.WaitCount(1, time.Second) {
			t.Fatal("请求未到达")
		}
		cancel()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("取消仍成功")
			}
		case <-time.After(time.Second):
			t.Fatal("未有界退出")
		}
		if srv.Count() != 1 || len(c.results) != 1 || c.results[0].Failure != "response_unknown" || len(c.receipts) != 0 {
			t.Fatal("在途取消未保留未知调用")
		}
	})
	t.Run("整体期限", func(t *testing.T) {
		e, p, c, srv := fixture(t, providertest.Hang())
		p.cfg.Limits.Timeout = 100 * time.Millisecond
		if v, err := e.Run(context.Background(), p); err == nil || v != nil || srv.Count() != 1 || len(c.results) != 1 {
			t.Fatal("整体期限失效")
		}
	})
	t.Run("预约后取消不发送", func(t *testing.T) {
		e, p, c, srv := fixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c.onStart = cancel
		if v, err := e.Run(ctx, p); err == nil || v != nil || srv.Count() != 0 || len(c.results) != 1 || c.results[0].Failure != "not_sent" {
			t.Fatal("取消后发送或丢预约")
		}
	})
}

func TestEngineConcurrentSameQuestionOnlyOneExecution(t *testing.T) {
	e, p, c, srv := fixture(t, successfulSteps()...)
	var wg sync.WaitGroup
	done := make(chan error, 16)
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := e.Run(context.Background(), p); done <- err }()
	}
	wg.Wait()
	close(done)
	success := 0
	for err := range done {
		if err == nil {
			success++
		}
	}
	if success != 1 || srv.Count() != 3 || len(c.results) != 3 || len(c.receipts) != 1 {
		t.Fatal("相同执行权产生重复调用")
	}
}

func TestEngineResultIsolationAndInvalidConstruction(t *testing.T) {
	e, p, _, _ := fixture(t, successfulSteps()...)
	if _, err := NewEngine(nil, e.models[1], e.models[2], e.control); err == nil {
		t.Fatal("接受空模型")
	}
	if _, err := NewEngine(e.models[0], e.models[1], e.models[2], nil); err == nil {
		t.Fatal("接受空控制器")
	}
	ordinary, err := provider.New(provider.Config{Enabled: true, Name: "非直连测试", BaseURL: "https://example.test/v1", Model: "qa-test-model", APIKey: "test-only", TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEngine(ordinary, e.models[1], e.models[2], e.control); err == nil {
		t.Fatal("接受非直连模型")
	}
	for _, bad := range []*Plan{nil, {}} {
		if _, err := e.Run(context.Background(), bad); err == nil {
			t.Fatal("接受空计划")
		}
	}
	if _, err := e.Run(nil, p); err == nil {
		t.Fatal("接受空上下文")
	}
	var empty *Engine
	if _, err := empty.Run(context.Background(), p); err == nil {
		t.Fatal("接受空执行核")
	}
	if _, err := empty.View(context.Background(), p, nil); err == nil {
		t.Fatal("空执行核可读")
	}
	if _, err := e.View(nil, p, nil); err == nil {
		t.Fatal("空结果可读")
	}
	v, err := e.Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Outcome){func(v *Outcome) { v.planDigest = strings.Repeat("0", 64) }, func(v *Outcome) { v.policy = strings.Repeat("1", 64) }, func(v *Outcome) { v.citations[0].SentenceIDs[0] = 2 }, func(v *Outcome) { v.calls = 1 }, func(v *Outcome) { v.state = "answered-other" }} {
		copy := *v
		copy.citations = clonePicks(v.citations)
		mutate(&copy)
		if _, err := e.View(context.Background(), p, &copy); err == nil {
			t.Fatal("篡改结果仍可读")
		}
	}
	other := *p
	other.digest = strings.Repeat("2", 64)
	if _, err := e.View(context.Background(), &other, v); err == nil {
		t.Fatal("跨问题复用结果")
	}
	for _, item := range []any{*p, p, p.scope, p.Proof(), *v, v, Receipt{InputDigest: p.digest}} {
		printed := fmt.Sprintf("%v %+v %#v", item, item, item)
		for _, private := range []string{p.input.Question, p.scope.Snapshots[0].Body, p.digest, p.scope.ApprovalRef} {
			if strings.Contains(printed, private) {
				t.Fatal("格式化日志泄露私有输入")
			}
		}
	}
}
