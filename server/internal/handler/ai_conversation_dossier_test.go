package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
)

type referenceHandlerStub struct {
	calls int
	err   error
}

func (s *referenceHandlerStub) Save(_ context.Context, token string, uid, cid, expected int64, selected dossier.Selection) (dto.AIConversationDossier, error) {
	s.calls++
	if token != "test-member" || uid != 1 || cid != 3 || expected != 1 || selected.ThemeID != 7 {
		return dto.AIConversationDossier{}, errors.New("仅测试：请求错绑")
	}
	return dto.AIConversationDossier{ID: 5, ConversationID: cid, ThemeID: selected.ThemeID, Version: selected.Version}, s.err
}
func (s *referenceHandlerStub) Receipt(context.Context, string, int64, int64, dossier.Selection) (dto.AIConversationDossier, error) {
	s.calls++
	return dto.AIConversationDossier{ID: 5}, s.err
}
func (s *referenceHandlerStub) List(context.Context, string, int64, int64, int64) (dto.AIConversationDossierPage, error) {
	s.calls++
	return dto.AIConversationDossierPage{Notice: "仅测试：私有元数据"}, s.err
}
func (s *referenceHandlerStub) Read(context.Context, string, int64, int64, int64) (dto.AIConversationDossierView, error) {
	s.calls++
	return dto.AIConversationDossierView{Dossier: dto.AIDossierView{Notice: "仅测试：私有原文"}}, s.err
}

func TestConversationDossierHTTPProtocol(t *testing.T) {
	const base = "/api/v1/ai/conversations/3/dossiers"
	const body = `{"themeId":7,"version":1,"compareVersion":0,"expectedVersion":1}`
	for _, c := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"保存单版", "POST", base, body, 200}, {"保存双版", "POST", base, strings.Replace(body, `"compareVersion":0`, `"compareVersion":2`, 1), 200},
		{"列表", "GET", base, "", 200}, {"分页", "GET", base + "?beforeId=9", "", 200}, {"打开引用", "GET", base + "/5", "", 200},
		{"原组合收据", "GET", "/api/v1/ai/conversations/3/dossier-receipt?themeId=7&version=1&compareVersion=0", "", 200},
		{"缺对照字段", "POST", base, `{"themeId":7,"version":1,"expectedVersion":1}`, 400},
		{"未知身份字段", "POST", base, strings.Replace(body, `"themeId":7`, `"userId":2,"themeId":7`, 1), 400},
		{"重复字段", "POST", base, strings.Replace(body, `"themeId":7`, `"themeId":7,"themeId":8`, 1), 400},
		{"额外来源", "POST", base, strings.Replace(body, `"themeId":7`, `"url":"https://example.test/","themeId":7`, 1), 400},
		{"小数版本", "POST", base, strings.Replace(body, `"version":1`, `"version":1.5`, 1), 400},
		{"逆序对照", "POST", base, strings.Replace(body, `"compareVersion":0`, `"compareVersion":1`, 1), 400},
		{"超界编号", "POST", base, strings.Replace(body, `"themeId":7`, `"themeId":9007199254740992`, 1), 400},
		{"POST夹查询", "POST", base + "?userId=2", body, 400}, {"读取带正文", "GET", base, "{}", 400},
		{"重复游标", "GET", base + "?beforeId=9&beforeId=8", "", 400}, {"未知查询", "GET", base + "?limit=100", "", 400},
		{"零游标", "GET", base + "?beforeId=0", "", 400}, {"前导零引用", "GET", base + "/05", "", 400},
		{"引用夹参数", "GET", base + "/5?themeId=7", "", 400}, {"缺收据选择", "GET", "/api/v1/ai/conversations/3/dossier-receipt?themeId=7&version=1", "", 400},
		{"HEAD拒绝", "HEAD", base, "", 400}, {"不开放删除", "DELETE", base + "/5", "", 405},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &referenceHandlerStub{}
			auth, _ := middleware.NewMemberAuth(qaHandlerAuth{})
			mux := http.NewServeMux()
			NewAIConversationDossierHandler(s).Register(mux, auth)
			r := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			r.Header.Set("Authorization", "Bearer test-member")
			if c.method == http.MethodPost {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != c.status || (s.calls == 1) != (c.status == 200) {
				t.Fatal("接口闭集边界不符", w.Code, s.calls, w.Body.String())
			}
			if c.status != 405 && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("私有结果可缓存")
			}
		})
	}
}
func TestConversationDossierHTTPErrors(t *testing.T) {
	for _, c := range []struct {
		name   string
		err    error
		status int
	}{
		{"撤权", dossier.ErrNotFound, 404}, {"他人会话", conversation.ErrNotFound, 404}, {"归档版本变化", conversation.ErrConflict, 409},
		{"数量上限", dossier.ErrReferenceLimit, 409}, {"身份失效", memberauth.ErrUnauthorized, 401}, {"内部未知", errors.New("仅测试：内部原文和错误"), 503},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &referenceHandlerStub{err: c.err}
			auth, _ := middleware.NewMemberAuth(qaHandlerAuth{})
			mux := http.NewServeMux()
			NewAIConversationDossierHandler(s).Register(mux, auth)
			r := httptest.NewRequest("GET", "/api/v1/ai/conversations/3/dossiers/5", nil)
			r.Header.Set("Authorization", "Bearer test-member")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != c.status || strings.Contains(w.Body.String(), "仅测试") || strings.Contains(w.Body.String(), `"reference"`) || strings.Contains(w.Body.String(), `"dossier"`) {
				t.Fatal("错误泄露部分内容", w.Code, w.Body.String())
			}
		})
	}
}
