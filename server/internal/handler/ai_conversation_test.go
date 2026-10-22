package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type conversationHTTPStub struct {
	calls int
	err   error
}

func (s *conversationHTTPStub) Create(_ context.Context, uid int64, key, title string) (dto.AIConversationReceipt, error) {
	s.calls++
	return dto.AIConversationReceipt{ID: 9, CreatedAt: 1000}, s.err
}
func (s *conversationHTTPStub) Receipt(_ context.Context, uid int64, key string) (dto.AIConversationReceipt, error) {
	s.calls++
	return dto.AIConversationReceipt{ID: 9, CreatedAt: 1000}, s.err
}
func (s *conversationHTTPStub) Detail(context.Context, int64, int64) (dto.AIConversation, error) {
	s.calls++
	return dto.AIConversation{ID: 9, Title: "测试会话", Version: 1}, s.err
}
func (s *conversationHTTPStub) List(context.Context, int64, int64, bool, int) (dto.AIConversationPage, error) {
	s.calls++
	return dto.AIConversationPage{Items: []dto.AIConversationSummary{}}, s.err
}
func (s *conversationHTTPStub) Rename(context.Context, int64, int64, int64, string) (dto.AIConversation, error) {
	s.calls++
	return dto.AIConversation{ID: 9, Title: "测试会话", Version: 2}, s.err
}
func (s *conversationHTTPStub) Archive(context.Context, int64, int64, int64) (dto.AIConversation, error) {
	s.calls++
	return dto.AIConversation{ID: 9, Title: "测试会话", Version: 2, Archived: true}, s.err
}
func (s *conversationHTTPStub) Messages(context.Context, int64, int64, int64, int) (dto.AIConversationMessages, error) {
	s.calls++
	return dto.AIConversationMessages{ConversationID: 9, Items: []dto.AIConversationMessage{}}, s.err
}

func TestAIConversationHTTPStrictInputs(t *testing.T) {
	st := &conversationHTTPStub{}
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	mux := http.NewServeMux()
	NewAIConversationHandler(st).Register(mux, auth)
	for _, tc := range []struct {
		name, method, path, body string
		code                     int
	}{
		{"新建", "POST", "/conversations", `{"title":"测试会话"}`, 200},
		{"列表", "GET", "/conversations", "", 200}, {"分页", "GET", "/conversations?before_id=9&limit=1&archived=1", "", 200},
		{"核查", "GET", "/conversation-receipt", "", 200}, {"详情", "GET", "/conversations/9", "", 200},
		{"改名", "PATCH", "/conversations/9", `{"title":"测试会话","expectedVersion":1}`, 200},
		{"归档", "POST", "/conversations/9/archive", `{"expectedVersion":1}`, 200},
		{"历史", "GET", "/conversations/9/messages?after_seq=0&limit=20", "", 200},
		{"拒绝消息写入", "POST", "/conversations/9/messages", `{}`, 405},
		{"拒绝删除", "DELETE", "/conversations/9", "", 405},
		{"HEAD", "HEAD", "/conversations/9", "", 400},
		{"编号前导零", "GET", "/conversations/09", "", 400}, {"负编号", "GET", "/conversations/-1", "", 400},
		{"零游标", "GET", "/conversations?before_id=0", "", 400}, {"负游标", "GET", "/conversations?before_id=-1", "", 400},
		{"未知参数", "GET", "/conversations?user_id=42", "", 400}, {"重复参数", "GET", "/conversations?limit=1&limit=1", "", 400},
		{"大页", "GET", "/conversations?limit=101", "", 400}, {"零页", "GET", "/conversations?limit=0", "", 400},
		{"错误归档筛选", "GET", "/conversations?archived=true", "", 400},
		{"畸形转义", "GET", "/conversations?limit=%zz", "", 400}, {"空问号", "GET", "/conversations?", "", 400},
		{"历史负游标", "GET", "/conversations/9/messages?after_seq=-1", "", 400},
		{"历史大页", "GET", "/conversations/9/messages?limit=101", "", 400},
		{"读取正文", "GET", "/conversations/9", `{}`, 400},
		{"重复字段", "POST", "/conversations", `{"title":"甲","title":"乙"}`, 400},
		{"未知字段", "POST", "/conversations", `{"title":"甲","user_id":42}`, 400},
		{"大小写字段", "POST", "/conversations", `{"Title":"甲"}`, 400},
		{"null字段", "POST", "/conversations", `{"title":null}`, 400},
		{"尾随对象", "POST", "/conversations", `{"title":"甲"}{}`, 400},
		{"缺版本", "PATCH", "/conversations/9", `{"title":"甲"}`, 400},
		{"错误版本类型", "POST", "/conversations/9/archive", `{"expectedVersion":"1"}`, 400},
		{"写入查询", "POST", "/conversations?x=1", `{"title":"甲"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/api/v1/ai"+tc.path, strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer test-only")
			r.Header.Set("Idempotency-Key", "conversation_http_test")
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			before := st.calls
			mux.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.code == 400 && st.calls != before {
				t.Fatal("坏输入触达service")
			}
			if tc.code == 200 && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("私有响应可缓存")
			}
		})
	}
	for _, fault := range []struct {
		err  error
		code int
	}{{conversation.ErrInvalid, 400}, {conversation.ErrNotFound, 404}, {conversation.ErrConflict, 409}, {errors.New("内部敏感详情"), 503}} {
		st.err = fault.err
		r := httptest.NewRequest("GET", "/api/v1/ai/conversations/9", nil)
		r.Header.Set("Authorization", "Bearer test-only")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != fault.code || strings.Contains(w.Body.String(), "敏感") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestAIConversationHTTPIdentityAndKey(t *testing.T) {
	st := &conversationHTTPStub{}
	auth, _ := middleware.NewMemberAuth(taskMemberStub{})
	mux := http.NewServeMux()
	NewAIConversationHandler(st).Register(mux, auth)
	for _, key := range []string{"", "short", "space key is invalid"} {
		r := httptest.NewRequest("POST", "/api/v1/ai/conversations", strings.NewReader(`{"title":"测试会话"}`))
		r.Header.Set("Authorization", "Bearer test-only")
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 400 || st.calls != 0 {
			t.Fatal(w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/ai/conversations", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 401 || st.calls != 0 {
		t.Fatal("缺身份放行")
	}
	w = httptest.NewRecorder()
	NewAIConversationHandler(st).serve(w, r)
	if w.Code != 401 {
		t.Fatal("绕过中间件")
	}
	mux = http.NewServeMux()
	NewAIConversationHandler(nil).Register(mux, auth)
	r.Header.Set("Authorization", "Bearer test-only")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("空依赖放行")
	}
}
