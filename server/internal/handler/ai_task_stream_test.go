package handler

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type httpStreamReader struct {
	calls   atomic.Int32
	fault   error
	ongoing bool
}

func (s *httpStreamReader) Events(ctx context.Context, uid, id, after int64, limit int) (dto.AITaskEventPage, error) {
	s.calls.Add(1)
	if uid != 42 || id != 9 {
		return dto.AITaskEventPage{}, points.ErrNotFound
	}
	if s.fault != nil {
		return dto.AITaskEventPage{}, s.fault
	}
	if after > 2 {
		return dto.AITaskEventPage{}, points.ErrInvalid
	}
	p := dto.AITaskEventPage{Items: []dto.AITaskEvent{}, UpperSeq: 2, NextAfterSeq: after}
	for n := after + 1; n <= 2; n++ {
		e := dto.AITaskEvent{Seq: n, Type: "created", Status: 1, At: 1000 + n}
		if n == 2 {
			e.Type, e.Status = "succeeded", 3
			if s.ongoing {
				e.Type, e.Status = "claimed", 2
			}
		}
		p.Items = append(p.Items, e)
		p.NextAfterSeq = n
	}
	return p, ctx.Err()
}
func (s *httpStreamReader) Progress(_ context.Context, uid, id int64) (dto.AITaskProgress, error) {
	if uid != 42 || id != 9 {
		return dto.AITaskProgress{}, points.ErrNotFound
	}
	status := 3
	if s.ongoing {
		status = 2
	}
	return dto.AITaskProgress{ID: id, EventSeq: 2, Status: status}, nil
}

type httpStreamAuth func(context.Context, string) (int64, error)

func (f httpStreamAuth) Authenticate(ctx context.Context, token string) (int64, error) {
	return f(ctx, token)
}
func streamHTTPMux(t *testing.T, reader *httpStreamReader, authenticate httpStreamAuth) *http.ServeMux {
	t.Helper()
	if authenticate == nil {
		authenticate = func(context.Context, string) (int64, error) { return 42, nil }
	}
	s, err := service.NewAITaskStreamService(reader, authenticate)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(authenticate)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewAITaskStreamHandler(s).Register(mux, auth)
	return mux
}

func TestAITaskStreamRealHTTPAndResume(t *testing.T) {
	reader := &httpStreamReader{}
	server := httptest.NewServer(streamHTTPMux(t, reader, nil))
	defer server.Close()
	for _, after := range []string{"0", "1", "2"} {
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/ai/tasks/9/stream?after_seq="+after, nil)
		req.Header.Set("Authorization", "Bearer test-only")
		req.Header.Set("Last-Event-ID", after)
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Accel-Buffering") != "no" {
			t.Fatal(res.StatusCode, res.Header)
		}
		if !strings.HasPrefix(text, ": connected\n\n") {
			t.Fatal(text)
		}
		if strings.Contains(text, "\"reportAvailable\"") || strings.Contains(text, "test-only") || strings.Contains(text, "userId") {
			t.Fatal("泄漏不在白名单的信息")
		}
		if after == "0" && strings.Count(text, "event: progress") != 2 || after == "1" && strings.Count(text, "event: progress") != 1 || after == "2" && strings.Contains(text, "event:") {
			t.Fatal("续读重复/遗漏", text)
		}
		if after != "2" && !strings.Contains(text, "id: 2\nevent: progress\ndata: {\"seq\":2,\"type\":\"succeeded\",\"status\":3,\"step\":0,\"errorCode\":\"\",\"at\":1002}\n\n") {
			t.Fatal(text)
		}
	}
}

func TestAITaskStreamHTTPRejectsBeforeStreaming(t *testing.T) {
	for _, tc := range []struct {
		name, path, method, auth, body string
		headers                        []string
		code                           int
		fault                          error
	}{
		{name: "他人任务", path: "/api/v1/ai/tasks/8/stream", code: 404},
		{name: "非法编号", path: "/api/v1/ai/tasks/09/stream", code: 400},
		{name: "过大编号", path: "/api/v1/ai/tasks/9007199254740992/stream", code: 400},
		{name: "游标越过上界", path: "?after_seq=3", code: 400},
		{name: "负数", path: "?after_seq=-1", code: 400},
		{name: "前导零", path: "?after_seq=01", code: 400},
		{name: "空值", path: "?after_seq=", code: 400},
		{name: "重复", path: "?after_seq=0&after_seq=0", code: 400},
		{name: "未知", path: "?token=$TM_TEST_TOKEN", code: 400},
		{name: "畸形转义", path: "?after_seq=%zz", code: 400},
		{name: "空问号", path: "?", code: 400},
		{name: "无身份", auth: "none", code: 401},
		{name: "头游标矛盾", path: "?after_seq=0", headers: []string{"1"}, code: 400},
		{name: "重复头", headers: []string{"1", "1"}, code: 400},
		{name: "空头", headers: []string{""}, code: 400},
		{name: "坏头", headers: []string{"01"}, code: 400},
		{name: "HEAD", method: "HEAD", code: 400},
		{name: "请求体", body: "{}", code: 400},
		{name: "内部错误", fault: errors.New("测试内部敏感错误"), code: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &httpStreamReader{fault: tc.fault}
			server := httptest.NewServer(streamHTTPMux(t, reader, nil))
			defer server.Close()
			path := tc.path
			if !strings.HasPrefix(path, "/") {
				path = "/api/v1/ai/tasks/9/stream" + path
			}
			method := tc.method
			if method == "" {
				method = "GET"
			}
			req, err := http.NewRequest(method, server.URL+path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if tc.auth != "none" {
				req.Header.Set("Authorization", "Bearer test-only")
			}
			for _, h := range tc.headers {
				req.Header.Add("Last-Event-ID", h)
			}
			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.code || strings.Contains(string(raw), "event:") || strings.Contains(string(raw), "敏感") {
				t.Fatal(res.StatusCode, string(raw))
			}
			if tc.code == 401 && reader.calls.Load() != 0 {
				t.Fatal("缺身份触达事件")
			}
		})
	}
}

type httpStreamMemberReader struct {
	enabled atomic.Bool
	calls   atomic.Int32
}

func (s *httpStreamMemberReader) EnabledMember(_ context.Context, uid int64) (bool, error) {
	s.calls.Add(1)
	return uid == 42 && s.enabled.Load(), nil
}

func TestAITaskStreamRealJWTRevocation(t *testing.T) {
	key := strings.Repeat("k", 40)
	verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	members := &httpStreamMemberReader{}
	members.enabled.Store(true)
	sessions, err := service.NewMemberSessionService(verifier, members)
	if err != nil {
		t.Fatal(err)
	}
	reader := &httpStreamReader{ongoing: true}
	s, err := service.NewAITaskStreamService(reader, sessions)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewAITaskStreamHandler(s).Register(mux, auth)
	server := httptest.NewServer(mux)
	defer server.Close()
	makeToken := func(kind string, expiry time.Time) string {
		p := jwt.MapClaims{"ID": 42, "UserType": kind, "iss": "GVA", "aud": []string{"GVA"}, "exp": expiry.Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, p).SignedString([]byte(key))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	for _, token := range []string{makeToken("admin", time.Now().Add(time.Hour)), makeToken("client", time.Now().Add(-time.Minute))} {
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/ai/tasks/9/stream", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 || reader.calls.Load() != 0 || members.calls.Load() != 0 {
			t.Fatal("无效令牌触达数据")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/v1/ai/tasks/9/stream", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken("client", time.Now().Add(time.Hour)))
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := bufio.NewReader(res.Body)
	line, err := buf.ReadString('\n')
	if err != nil || line != ": connected\n" {
		t.Fatal(line, err)
	}
	members.enabled.Store(false)
	raw, err := io.ReadAll(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "event: error\ndata: {\"code\":40100}") || reader.calls.Load() != 1 || members.calls.Load() < 3 {
		t.Fatal(string(raw), reader.calls.Load(), members.calls.Load())
	}
}

type streamDeadlineRecorder struct {
	*httptest.ResponseRecorder
	writes, failAt int
	deadlines      []time.Time
}

func (w *streamDeadlineRecorder) SetWriteDeadline(d time.Time) error {
	w.deadlines = append(w.deadlines, d)
	return nil
}
func (w *streamDeadlineRecorder) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseRecorder.Write(p)
}

func TestAITaskStreamWriterAndDirectAccess(t *testing.T) {
	for _, failAt := range []int{1, 2, 3} {
		reader := &httpStreamReader{}
		mux := streamHTTPMux(t, reader, nil)
		req := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/stream", nil)
		req.Header.Set("Authorization", "Bearer test-only")
		w := &streamDeadlineRecorder{ResponseRecorder: httptest.NewRecorder(), failAt: failAt}
		mux.ServeHTTP(w, req)
		if w.writes != failAt || strings.Contains(w.Body.String(), "event: error") {
			t.Fatal("写失败后重写", w.writes, w.Body.String())
		}
		if len(w.deadlines) < 2 || !w.deadlines[len(w.deadlines)-1].IsZero() {
			t.Fatal("写期限未清理")
		}
	}
	mux := streamHTTPMux(t, &httpStreamReader{}, nil)
	req := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/stream", nil)
	req.Header.Set("Authorization", "Bearer test-only")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal("无写期限宿主放行")
	}
	w = httptest.NewRecorder()
	NewAITaskStreamHandler(nil).stream(w, req)
	if w.Code != 401 {
		t.Fatal("绕过中间件")
	}
	auth, _ := middleware.NewMemberAuth(httpStreamAuth(func(context.Context, string) (int64, error) { return 42, nil }))
	mux = http.NewServeMux()
	NewAITaskStreamHandler(nil).Register(mux, auth)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal("空服务放行")
	}
}

func TestAITaskStreamTransportCursor(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   int64
		bad    bool
	}{{"0", 0, false}, {"2", 2, false}, {"", 0, true}, {"+1", 0, true}, {"9007199254740992", 0, true}, {"1,2", 0, true}} {
		req := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/stream", nil)
		req.Header.Set("Last-Event-ID", tc.header)
		after, err := streamAfter(req)
		if after != tc.want || (err != nil) != tc.bad {
			t.Fatal(tc, after, err)
		}
	}
	for _, change := range []func(*http.Request){func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }, func(r *http.Request) { r.TransferEncoding = []string{"chunked"} }} {
		req := httptest.NewRequest("GET", "/api/v1/ai/tasks/9/stream", nil)
		change(req)
		if _, err := streamAfter(req); err == nil {
			t.Fatal("异常正文放行")
		}
	}
}
