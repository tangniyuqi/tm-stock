package feedbackconsole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
)

type testIdentity struct {
	active bool
	err    error
}

func (s testIdentity) Active(ctx context.Context, uid int64) (bool, error) {
	if uid != 7 {
		panic("错误后台身份")
	}
	if s.err != nil {
		return false, s.err
	}
	return s.active, ctx.Err()
}
func testConfig(origin string) config.FeedbackConsoleConfig {
	return config.FeedbackConsoleConfig{Enabled: true, Upstream: origin, WebOrigin: "http://127.0.0.1:8080", Reviewers: []config.FeedbackConsoleReviewer{{UserID: 7, ActorID: 70, Token: strings.Repeat("t", 40)}}}
}
func testRecord() Record {
	return Record{ID: 1, TaskID: 9, Category: "fact", Status: "received", Version: 1, CreatedAt: 1792480000000, UpdatedAt: 1792480000000}
}
func TestFeedbackConsoleClient(t *testing.T) {
	var calls atomic.Int32
	current := testRecord()
	events := []Event{{Version: 1, Status: "received", At: current.CreatedAt}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 40) || r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" {
			t.Error("内部身份未隔离")
		}
		var data any
		switch {
		case r.URL.Path == "/internal/v1/ai-feedback":
			data = Page{Items: []Record{current}}
		case r.Method == "POST" && r.URL.Path == "/internal/v1/ai-feedback/1/decisions":
			var d Decision
			if json.NewDecoder(r.Body).Decode(&d) != nil {
				t.Error("坏提交")
			}
			if current.Version != d.ExpectedVersion {
				w.WriteHeader(409)
				fmt.Fprint(w, `{"code":40904,"msg":"内部文字不应展示"}`)
				return
			}
			current.Version++
			current.Status = d.Status
			current.Resolution = d.Resolution
			events = append(events, Event{Version: current.Version, ActorID: 70, Status: current.Status, Resolution: current.Resolution, At: current.UpdatedAt})
			data = current
		case r.URL.Path == "/internal/v1/ai-feedback/1":
			data = Detail{current, events}
		default:
			t.Error("越界端点", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "", "data": data})
	}))
	defer srv.Close()
	client, err := New(testConfig(srv.URL), testIdentity{active: true})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if _, err = client.List(ctx, 8, 0, 20); !errors.Is(err, ErrForbidden) || calls.Load() != 0 {
		t.Fatal("未登记触达上游")
	}
	if p, err := client.List(ctx, 7, 0, 20); err != nil || len(p.Items) != 1 {
		t.Fatal(p, err)
	}
	if d, err := client.Detail(ctx, 7, 1); err != nil || d.Item.Version != 1 {
		t.Fatal(d, err)
	}
	for _, d := range []Decision{{1, "processing", ""}, {2, "resolved", "reviewed"}} {
		r, err := client.Decide(ctx, 7, 1, d)
		if err != nil || r.Version != d.ExpectedVersion+1 {
			t.Fatal(r, err)
		}
	}
	if d, err := client.Detail(ctx, 7, 1); err != nil || len(d.Events) != 3 || d.Events[1].ActorID != 70 {
		t.Fatal(d, err)
	}
	if _, err := client.Decide(ctx, 7, 1, Decision{1, "processing", ""}); !errors.Is(err, ErrConflict) {
		t.Fatal("冲突未透传固定类型", err)
	}
	before := calls.Load()
	for _, fn := range []func() error{func() error { _, e := client.List(ctx, 7, -1, 20); return e }, func() error { _, e := client.List(ctx, 7, 0, 101); return e }, func() error { _, e := client.Detail(ctx, 7, 0); return e }, func() error { _, e := client.Decide(ctx, 7, 1, Decision{1, "resolved", "reviewed"}); return e }} {
		if !errors.Is(fn(), ErrInvalid) {
			t.Fatal("坏参数放行")
		}
	}
	if before != calls.Load() {
		t.Fatal("无效输入触达内部服务")
	}
	for _, identity := range []testIdentity{{active: false}, {err: ErrUnconfirmed}} {
		c, err := New(testConfig(srv.URL), identity)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.List(ctx, 7, 0, 20)
		c.Close()
		if err == nil || before != calls.Load() {
			t.Fatal("当前用户检查未失败关闭")
		}
	}
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := client.List(ended, 7, 0, 20); err == nil {
		t.Fatal("取消请求未生效")
	}
	if _, err := New(testConfig(srv.URL), nil); err == nil {
		t.Fatal("缺当前身份核验仍启动")
	}
}
func TestFeedbackConsoleMalformedAndTransport(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   error
	}{
		{200, `{"code":0,"code":0,"msg":"","data":{}}`, ErrUnconfirmed},
		{200, `{"code":0,"msg":"","data":{"items":[],"nextAfterId":0,"hasMore":false,"secret":"private"}}`, ErrUnconfirmed},
		{200, `{"code":0,"msg":"","data":{"items":null,"nextAfterId":0,"hasMore":false}}`, ErrUnconfirmed},
		{200, `{"code":0,"msg":"","data":{"items":[],"nextAfterId":0,"hasMore":true}}`, ErrUnconfirmed},
		{200, strings.Repeat("x", 65537), ErrUnconfirmed},
		{404, `{"code":40403,"msg":"secret"}`, ErrNotFound}, {401, `{"code":40100,"msg":"secret"}`, ErrForbidden}, {503, `{"code":0,"msg":"secret","data":{}}`, ErrUnconfirmed},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
		c, err := New(testConfig(srv.URL), testIdentity{active: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.List(context.Background(), 7, 0, 20)
		c.Close()
		srv.Close()
		if !errors.Is(err, tc.want) {
			t.Fatal("坏回包放行", err)
		}
	}
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	from := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer from.Close()
	c, err := New(testConfig(from.URL), testIdentity{active: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.List(context.Background(), 7, 0, 20)
	c.Close()
	if err == nil || targetCalls.Load() != 0 {
		t.Fatal("跟随重定向")
	}
	from.Close()
	c, err = New(testConfig(from.URL), testIdentity{active: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.List(context.Background(), 7, 0, 20)
	c.Close()
	if err == nil {
		t.Fatal("传输失败假成功")
	}
	for _, raw := range []string{`{"expectedVersion":1,"status":"processing","resolution":""}`, `{"expectedVersion":2,"status":"dismissed","resolution":"unconfirmed"}`} {
		if _, err := DecodeDecision([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{"expectedVersion":1,"status":"processing","resolution":"","actorId":7}`, `{"expectedVersion":1,"status":"processing","status":"resolved","resolution":""}`, `{"ExpectedVersion":1,"status":"processing","resolution":""}`, `{"expectedVersion":null,"status":"processing","resolution":""}`, `{} {}`, `{"expectedVersion":1,"status":"resolved","resolution":"reviewed"}`} {
		if _, err := DecodeDecision([]byte(raw)); err == nil {
			t.Fatal("坏决定JSON放行")
		}
	}
}
func TestFeedbackConsoleCurrentAdmin(t *testing.T) {
	db := testutil.NewMemoryDBWithoutGlobal(t)
	if err := db.Exec("CREATE TABLE sys_users(id INTEGER PRIMARY KEY,enable INTEGER,deleted_at DATETIME)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sys_users VALUES(7,1,NULL),(8,2,NULL),(9,1,CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	s := NewAdminIdentity(db)
	for _, tc := range []struct {
		id   int64
		want bool
	}{{7, true}, {8, false}, {9, false}, {10, false}} {
		active, err := s.Active(context.Background(), tc.id)
		if err != nil || active != tc.want {
			t.Fatal(tc.id, active, err)
		}
	}
	if _, err := NewAdminIdentity(nil).Active(context.Background(), 7); err == nil {
		t.Fatal("缺数据库放行")
	}
	if err := db.Exec("DROP TABLE sys_users").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Active(context.Background(), 7); err == nil {
		t.Fatal("查询失败放行")
	}
}

func TestFeedbackConsoleDecisionProof(t *testing.T) {
	for _, mode := range []string{"wrong_actor", "stale_history", "missing_history", "wrong_task"} {
		t.Run(mode, func(t *testing.T) {
			var posts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				item := testRecord()
				item.Version = 2
				item.Status = "processing"
				var out any = item
				if r.Method == "POST" {
					posts.Add(1)
				} else {
					events := []Event{{Version: 1, Status: "received", At: item.CreatedAt}, {Version: 2, ActorID: 70, Status: "processing", At: item.UpdatedAt}}
					switch mode {
					case "wrong_actor":
						events[1].ActorID = 71
					case "stale_history":
						item = testRecord()
						events = events[:1]
					case "missing_history":
						events = events[:1]
					case "wrong_task":
						item.TaskID = 10
					}
					out = Detail{item, events}
				}
				json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "", "data": out})
			}))
			defer srv.Close()
			c, err := New(testConfig(srv.URL), testIdentity{active: true})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			result, err := c.Decide(context.Background(), 7, 1, Decision{1, "processing", ""})
			if !errors.Is(err, ErrUnconfirmed) || result.ID != 0 || posts.Load() != 1 {
				t.Fatal("缺正确审计仍返回已确认或自动重试", result, err, posts.Load())
			}
		})
	}
}
