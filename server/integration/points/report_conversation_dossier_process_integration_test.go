//go:build integration

package points_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func TestIntegrationAIReportConversationDossierProcess(t *testing.T) {
	p := newDossierProcessFixture(t, 3*time.Minute, true)
	e := p.f.env
	tokens, _ := localPasswordTokens(t, e.ctx, p.login, p.client)
	call := func(token, method, path, key, body string, want int) []byte {
		t.Helper()
		r, err := http.NewRequestWithContext(e.ctx, method, p.base+path, strings.NewReader(body))
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
		res, err := p.client.Do(r)
		if err != nil {
			t.Fatal("独立API引用请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 1048577))
		if err != nil || len(raw) > 1048576 || res.StatusCode != want || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("独立API引用响应不符", res.StatusCode, want)
		}
		if want != 200 && (strings.Contains(string(raw), "样本主体甲") || strings.Contains(string(raw), `"reference"`) || strings.Contains(string(raw), `"versions"`)) {
			t.Fatal("引用拒绝响应泄露正文或元数据")
		}
		return raw
	}
	created := conversationData[dto.AIConversationReceipt](t, call(tokens[0], "POST", "/api/v1/ai/conversations", "test_reference_process_create", `{"title":"仅测试：引用恢复"}`, 200))
	base := "/api/v1/ai/conversations/" + strconv.FormatInt(created.ID, 10)
	body := `{"themeId":1,"version":1,"compareVersion":0,"expectedVersion":1}`
	var first, pair dto.AIConversationDossier
	t.Run("原登录保存与组合重放", func(t *testing.T) {
		first = conversationData[dto.AIConversationDossier](t, call(tokens[0], "POST", base+"/dossiers", "", body, 200))
		again := conversationData[dto.AIConversationDossier](t, call(tokens[0], "POST", base+"/dossiers", "", body, 200))
		if first.ID == 0 || first != again || first.ConversationID != created.ID || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 1 {
			t.Fatal("正式API未按原组合去重")
		}
	})
	t.Run("目录收据与当前原文", func(t *testing.T) {
		page := conversationData[dto.AIConversationDossierPage](t, call(tokens[0], "GET", base+"/dossiers", "", "", 200))
		if len(page.Items) != 1 || page.Items[0] != first || page.Notice != dto.AIConversationDossierNotice {
			t.Fatal("正式API引用目录错误")
		}
		receipt := conversationData[dto.AIConversationDossier](t, call(tokens[0], "GET", base+"/dossier-receipt?themeId=1&version=1&compareVersion=0", "", "", 200))
		if receipt != first {
			t.Fatal("正式API原组合收据错误")
		}
		view := conversationData[dto.AIConversationDossierView](t, call(tokens[0], "GET", base+"/dossiers/"+strconv.FormatInt(first.ID, 10), "", "", 200))
		if view.Reference != first || len(view.Dossier.Versions) != 1 || view.Dossier.Versions[0].Cards[0].Quote.SourceURL != "https://dossier.example.test/synthetic-only" {
			t.Fatal("正式API引用没有重读原文")
		}
	})
	t.Run("双版保存与摘录对照", func(t *testing.T) {
		pair = conversationData[dto.AIConversationDossier](t, call(tokens[0], "POST", base+"/dossiers", "", strings.Replace(body, `"compareVersion":0`, `"compareVersion":2`, 1), 200))
		view := conversationData[dto.AIConversationDossierView](t, call(tokens[0], "GET", base+"/dossiers/"+strconv.FormatInt(pair.ID, 10), "", "", 200))
		if len(view.Dossier.Versions) != 2 || view.Dossier.Comparison == nil {
			t.Fatal("正式API双版引用没有对照")
		}
	})
	t.Run("跨会员与匿名拒绝", func(t *testing.T) {
		for _, token := range []string{tokens[1], ""} {
			want := 404
			if token == "" {
				want = 401
			}
			call(token, "GET", base+"/dossiers", "", "", want)
			call(token, "GET", base+"/dossiers/"+strconv.FormatInt(pair.ID, 10), "", "", want)
		}
	})
	t.Run("撤权拒读但本人收据可查", func(t *testing.T) {
		e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=2`)
		call(tokens[0], "GET", base+"/dossiers/"+strconv.FormatInt(pair.ID, 10), "", "", 404)
		receipt := conversationData[dto.AIConversationDossier](t, call(tokens[0], "GET", base+"/dossier-receipt?themeId=1&version=1&compareVersion=2", "", "", 200))
		if receipt != pair {
			t.Fatal("撤权后不能核查原保存")
		}
		call(tokens[0], "POST", base+"/dossiers", "", `{"themeId":1,"version":2,"compareVersion":0,"expectedVersion":1}`, 404)
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_dossiers`) != 2 {
			t.Fatal("拒绝保存仍落库")
		}
		e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1 WHERE version_no=2`)
	})
	t.Run("归档只读与原消息账务不变", func(t *testing.T) {
		call(tokens[0], "POST", base+"/archive", "", `{"expectedVersion":1}`, 200)
		call(tokens[0], "POST", base+"/dossiers", "", `{"themeId":1,"version":2,"compareVersion":0,"expectedVersion":2}`, 409)
		call(tokens[0], "GET", base+"/dossiers/"+strconv.FormatInt(first.ID, 10), "", "", 200)
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 0 {
			t.Fatal("引用改写原聊天消息")
		}
		p.verifyNoGeneration(t)
	})
	p.login.finishAs(t, e.ctx, false)
}
