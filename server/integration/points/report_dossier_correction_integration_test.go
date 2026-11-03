//go:build integration

package points_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func TestIntegrationAIReportDossierCorrections(t *testing.T) {
	p := newDossierProcessWithQA(t, 6*time.Minute, false, nil, true)
	e := p.f.env
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_correction_events`) != 0 {
		t.Fatal("启动探针留下更正记录")
	}
	dsn, prepare, operator := dossierManagementTestDSN(t, p)
	account, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("管理连接解析失败")
	}
	e.mustExec("GRANT SELECT,INSERT ON " + e.database + ".tm_ai_dossier_correction_events TO '" + account.User + "'@'127.0.0.1'")
	base, keys := startDossierManagementTestProcess(t, p, dsn, prepare, true)
	tokens, _ := localPasswordTokens(t, e.ctx, p.login, p.client)
	call := func(t *testing.T, host, key, method, path string, body any, want int) []byte {
		t.Helper()
		var input io.Reader
		if body != nil {
			raw, er := json.Marshal(body)
			if er != nil {
				t.Fatal("更正请求编码失败")
			}
			input = bytes.NewReader(raw)
		}
		req, er := http.NewRequestWithContext(e.ctx, method, host+path, input)
		if er != nil {
			t.Fatal("更正请求创建失败")
		}
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, er := p.client.Do(req)
		if er != nil {
			t.Fatal("更正HTTP请求未确认")
		}
		defer res.Body.Close()
		raw, er := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if er != nil || res.StatusCode != want || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("更正HTTP状态不符", res.StatusCode, want)
		}
		if want != 200 && (bytes.Contains(raw, []byte("样本主体甲")) || bytes.Contains(raw, []byte("reviewReference")) || bytes.Contains(raw, []byte("items"))) {
			t.Fatal("拒读泄漏正文或更正数量")
		}
		return raw
	}
	const internal = "/internal/v1/ai-dossier-corrections"
	const pairQuery = "?themeId=1&beforeVersion=1&afterVersion=2"
	const public = "/api/v1/ai/themes/1/dossier-corrections?beforeVersion=1&afterVersion=2"
	decodePublic := func(t *testing.T, state string) {
		t.Helper()
		raw := call(t, p.base, tokens[0], "GET", public, nil, 200)
		var out struct {
			Code int                      `json:"code"`
			Data dto.AIDossierCorrections `json:"data"`
		}
		if json.Unmarshal(raw, &out) != nil || out.Code != 0 || out.Data.Notice != dto.AIDossierCorrectionNotice || out.Data.State != state {
			t.Fatal("更正公开范围不符")
		}
		for _, private := range []string{"actorId", "reviewReference", "requestId", "previousDigest"} {
			if bytes.Contains(raw, []byte(private)) {
				t.Fatal("更正公开响应泄漏审核记录")
			}
		}
		if state == "reviewed_declaration" {
			if len(out.Data.Items) != 1 || !strings.Contains(out.Data.Items[0].Statement.Quote.Text, "更正声明") || out.Data.Items[0].Before.Version != 1 || out.Data.Items[0].After.Version != 2 {
				t.Fatal("更正未绑定双方和声明原文")
			}
		} else if len(out.Data.Items) != 0 {
			t.Fatal("无有效审核返回旧内容")
		}
	}
	var preview struct {
		Code int                            `json:"code"`
		Data dto.AIDossierCorrectionInspect `json:"data"`
	}
	if json.Unmarshal(call(t, base, keys[0], "GET", internal+pairQuery, nil, 200), &preview) != nil || preview.Code != 0 || preview.Data.SourceState != "available" || preview.Data.Preview == nil || len(preview.Data.Preview.Versions) != 2 {
		t.Fatal("更正候选预览不可用")
	}
	r := dossier.CorrectionRequest{RequestID: "test_correction_review_001", Object: dossier.CorrectionPair{ThemeID: 1, BeforeVersion: 1, AfterVersion: 2}, BeforeDigest: preview.Data.BeforeDigest, AfterDigest: preview.Data.AfterDigest, Enabled: true, ValidUntil: time.Now().Add(10 * time.Minute).UnixMilli(), Declarations: []dossier.CorrectionDeclaration{{Before: dossier.CorrectionLocator{Key: "progress_a"}, After: dossier.CorrectionLocator{Key: "progress_a"}, StatementKey: "progress_a"}}, ReviewReference: "test-fixture:synthetic-explicit-declaration"}
	t.Run("原登录与独立审核读取", func(t *testing.T) {
		decodePublic(t, "unverified")
		call(t, base, keys[1], "GET", internal+pairQuery, nil, 403)
		call(t, p.base, tokens[1], "GET", public, nil, 404)
		call(t, p.base, "", "GET", public, nil, 401)
		call(t, p.base, keys[0], "GET", public, nil, 401)
		call(t, base, keys[0], "POST", internal, r, 200)
		decodePublic(t, "reviewed_declaration")
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_management_events`) != 0 || e.queryInt(`SELECT SUM(revision) FROM tm_ai_dossier_management_heads`) != 0 {
			t.Fatal("审核擅自接管或改变发布状态")
		}
	})
	t.Run("原请求与异体角色隔离", func(t *testing.T) {
		call(t, base, keys[0], "POST", internal, r, 200)
		call(t, base, keys[0], "GET", internal+"/requests/"+r.RequestID, nil, 200)
		call(t, base, keys[2], "GET", internal+"/requests/"+r.RequestID, nil, 404)
		changed := r
		changed.ReviewReference = "test-fixture:different"
		call(t, base, keys[0], "POST", internal, changed, 409)
		call(t, base, keys[2], "POST", internal, r, 409)
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_correction_events`) != 1 {
			t.Fatal("重放重复落库")
		}
	})
	t.Run("双方许可与来源撤回恢复", func(t *testing.T) {
		for _, q := range []string{`UPDATE tm_ai_dossier_grants SET enabled=0 WHERE user_id=1 AND version_no=1`, `UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=2`, `UPDATE tm_ai_sources SET enabled=0`} {
			e.mustExec(q)
			call(t, p.base, tokens[0], "GET", public, nil, 404)
			e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=1;UPDATE tm_ai_dossier_publications SET enabled=1;UPDATE tm_ai_sources SET enabled=1`)
			decodePublic(t, "reviewed_declaration")
		}
	})
	t.Run("失效来源可撤回且旧请求不恢复", func(t *testing.T) {
		e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
		next := r
		next.RequestID = "test_correction_block_002"
		next.ExpectedRevision = 1
		call(t, base, keys[0], "POST", internal, next, 404)
		var unavailable struct {
			Data dto.AIDossierCorrectionInspect `json:"data"`
		}
		if json.Unmarshal(call(t, base, keys[0], "GET", internal+pairQuery, nil, 200), &unavailable) != nil || unavailable.Data.Preview != nil {
			t.Fatal("失效预览泄漏正文")
		}
		next.RequestID = "test_correction_revoke_002"
		next.Enabled = false
		next.ValidUntil = 0
		next.Declarations = []dossier.CorrectionDeclaration{}
		call(t, base, keys[0], "POST", internal, next, 200)
		e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
		decodePublic(t, "unverified")
		call(t, base, keys[0], "POST", internal, r, 200)
		decodePublic(t, "unverified")
	})
	t.Run("并发修订恰好一次", func(t *testing.T) {
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for _, id := range []string{"test_correction_concurrent_a", "test_correction_concurrent_b"} {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				next := r
				next.RequestID = id
				next.ExpectedRevision = 2
				raw, er := json.Marshal(next)
				if er != nil {
					codes <- 0
					return
				}
				req, er := http.NewRequestWithContext(e.ctx, "POST", base+internal, bytes.NewReader(raw))
				if er != nil {
					codes <- 0
					return
				}
				req.Header.Set("Authorization", "Bearer "+keys[0])
				req.Header.Set("Content-Type", "application/json")
				res, er := p.client.Do(req)
				if er != nil {
					codes <- 0
					return
				}
				defer res.Body.Close()
				codes <- res.StatusCode
			}(id)
		}
		wg.Wait()
		close(codes)
		n := map[int]int{}
		for code := range codes {
			n[code]++
		}
		if n[200] != 1 || n[409] != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_correction_events`) != 3 {
			t.Fatal("并发更正未互斥")
		}
		decodePublic(t, "reviewed_declaration")
	})
	t.Run("审计权限故障原子拒绝", func(t *testing.T) {
		e.mustExec("REVOKE INSERT ON " + e.database + ".tm_ai_dossier_correction_events FROM '" + account.User + "'@'127.0.0.1'")
		next := r
		next.RequestID = "test_correction_no_insert"
		next.ExpectedRevision = 3
		call(t, base, keys[0], "POST", internal, next, 503)
		e.mustExec("GRANT INSERT ON " + e.database + ".tm_ai_dossier_correction_events TO '" + account.User + "'@'127.0.0.1'")
		e.mustExec("REVOKE SELECT ON " + e.database + ".tm_ai_dossier_correction_events FROM '" + p.apiUser + "'@'127.0.0.1'")
		call(t, p.base, tokens[0], "GET", public, nil, 503)
		e.mustExec("GRANT SELECT ON " + e.database + ".tm_ai_dossier_correction_events TO '" + p.apiUser + "'@'127.0.0.1'")
		decodePublic(t, "reviewed_declaration")
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_correction_events`) != 3 {
			t.Fatal("失败产生部分记录")
		}
	})
	t.Run("不可变保护及同名失效拒绝", func(t *testing.T) {
		if _, er := operator.ExecContext(e.ctx, `UPDATE tm_ai_dossier_correction_events SET actor_id=actor_id`); er == nil {
			t.Fatal("运行账号可修改审核")
		}
		if _, er := e.db.ExecContext(e.ctx, `DELETE FROM tm_ai_dossier_correction_events`); er == nil {
			t.Fatal("审计可删除")
		}
		e.mustExec(`DROP TRIGGER tm_ai_dossier_correction_no_update;CREATE TRIGGER tm_ai_dossier_correction_no_update BEFORE UPDATE ON tm_ai_dossier_correction_events FOR EACH ROW SET @correction_noop=1`)
		if repository.NewAIDossierCorrectionSchemaVerifier(e.db).Verify(e.ctx) == nil {
			t.Fatal("同名失效保护被放行")
		}
		reportDDL(t, e.pointsTestEnv, "20261103_create_tm_ai_dossier_corrections.sql")
		if er := repository.NewAIDossierCorrectionSchemaVerifier(e.db).Verify(e.ctx); er != nil {
			t.Fatal("恢复保护失败", er)
		}
		decodePublic(t, "reviewed_declaration")
	})
	p.verifyNoGeneration(t)
	p.login.finish(t, e.ctx)
}
