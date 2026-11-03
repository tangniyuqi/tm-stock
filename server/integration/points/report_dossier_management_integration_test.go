//go:build integration

package points_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func dossierManagementTestDSN(t *testing.T, p *dossierProcessFixture) (string, string, *sql.DB) {
	t.Helper()
	e := p.f.env
	dsn := localProcessDSN(t, e, nil, false)
	runtime, e1 := mysql.ParseDSN(dsn)
	owner, e2 := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if e1 != nil || e2 != nil {
		t.Fatal("专题管理测试连接无效")
	}
	owner.DBName = e.database
	// 标识符来自本例登记的隔离库及生成账号，不接受业务输入，不打印SQL/口令。
	for _, table := range []string{"tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_ai_dossier_management_heads", "tm_ai_dossier_management_events"} {
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO '" + runtime.User + "'@'127.0.0.1'")
	}
	e.mustExec("GRANT SELECT(id,status,deleted_at) ON " + e.database + ".addon_member TO '" + runtime.User + "'@'127.0.0.1'")
	for _, table := range []string{"tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_ai_dossier_management_heads"} {
		e.mustExec("GRANT INSERT,UPDATE ON " + e.database + "." + table + " TO '" + runtime.User + "'@'127.0.0.1'")
	}
	e.mustExec("GRANT INSERT ON " + e.database + ".tm_ai_dossier_management_events TO '" + runtime.User + "'@'127.0.0.1'")
	for _, table := range []string{"tm_ai_dossier_management_heads", "tm_ai_dossier_management_events"} {
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO '" + p.apiUser + "'@'127.0.0.1'")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("受限专题管理连接失败")
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("专题管理连接关闭失败")
		}
	})
	return dsn, owner.FormatDSN(), db
}
func startDossierManagementTestProcess(t *testing.T, p *dossierProcessFixture, dsn, prepare string) (string, []string) {
	t.Helper()
	binary := buildLocalFeedbackAdmin(t)
	e := p.f.env
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	reviewers := []config.AIAdminReviewer{}
	for _, actor := range []int64{71, 72, 73} {
		var seed [32]byte
		if _, err = rand.Read(seed[:]); err != nil {
			t.Fatal(err)
		}
		key := hex.EncodeToString(seed[:])
		keys = append(keys, key)
		reviewers = append(reviewers, config.AIAdminReviewer{ActorID: actor, Token: key})
	}
	raw, err := json.Marshal(reviewers)
	if err != nil {
		t.Fatal("专题管理身份编码失败")
	}
	ctx, cancel := context.WithCancel(e.ctx)
	command := exec.CommandContext(ctx, binary)
	command.Env = append(localWorkerEnvironment(dsn, ""), "TM_AI_ADMIN_ADDR="+addr, "TM_AI_ADMIN_REVIEWERS="+string(raw), "TM_AI_DOSSIER_MANAGEMENT_ADMIN_ENABLED=true", "TM_AI_DOSSIER_MANAGEMENT_ACTORS=71,73", "TM_AI_DOSSIER_MANAGEMENT_PREPARE_DSN="+prepare, "TM_AI_L0_ENABLED=true")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err = command.Start(); err != nil {
		cancel()
		t.Fatal("实际专题管理服务未启动")
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ended := false
	t.Cleanup(func() {
		cancel()
		if !ended {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("专题管理进程未退出")
			}
		}
	})
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-done:
			ended = true
			t.Fatal("实际专题管理服务提前结束")
		case <-deadline.C:
			t.Fatal("专题管理服务就绪超时")
		case <-tick.C:
			req, err := http.NewRequestWithContext(e.ctx, "GET", "http://"+addr+"/internal/v1/ai-dossier-management?kind=publication&userId=0&themeId=1&version=1", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+keys[0])
			res, err := p.client.Do(req)
			if err != nil {
				continue
			}
			if err = res.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if res.StatusCode == 200 {
				return "http://" + addr, keys
			}
		}
	}
}
func TestIntegrationAIReportDossierManagement(t *testing.T) {
	p := newDossierProcessFixture(t, 6*time.Minute)
	e := p.f.env
	reportDDL(t, e.pointsTestEnv, "20261103_create_tm_ai_dossier_management.sql")
	if err := repository.NewAIDossierManagementSchemaVerifier(e.db).Verify(e.ctx); err != nil {
		t.Fatal("管理结构/实际DML探针失败", err)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_ai_dossier_management_events") != 0 {
		t.Fatal("结构探针留下业务审计")
	}
	dsn, prepare, operator := dossierManagementTestDSN(t, p)
	if err := repository.NewAIDossierManagementRepo(operator).VerifyAccess(e.ctx); err != nil {
		t.Fatal("受限管理权限失败", err)
	}
	base, keys := startDossierManagementTestProcess(t, p, dsn, prepare)
	tokens, _ := localPasswordTokens(t, e.ctx, p.login, p.client)
	call := func(t *testing.T, key, method, path string, body any, want int) []byte {
		t.Helper()
		var input io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal("请求编码失败")
			}
			input = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(e.ctx, method, base+path, input)
		if err != nil {
			t.Fatal(err)
		}
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := p.client.Do(req)
		if err != nil {
			t.Fatal("专题管理HTTP结果未确认")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if err != nil || res.StatusCode != want || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("专题管理HTTP状态不符", res.StatusCode, want)
		}
		if want != 200 && (bytes.Contains(raw, []byte("样本主体甲")) || bytes.Contains(raw, []byte("reviewReference"))) {
			t.Fatal("拒绝响应泄漏候选或内部引用")
		}
		return raw
	}
	public := func(t *testing.T, token string, want int) {
		t.Helper()
		req, err := http.NewRequestWithContext(e.ctx, "GET", p.base+"/api/v1/ai/themes/1/dossiers/1", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := p.client.Do(req)
		if err != nil {
			t.Fatal("会员原文请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if err != nil || res.StatusCode != want {
			t.Fatal("会员读取状态不符", res.StatusCode, want)
		}
		if want != 200 && (bytes.Contains(raw, []byte("样本主体甲")) || bytes.Contains(raw, []byte("sourceRelations"))) {
			t.Fatal("撤回后泄漏原文或关系")
		}
	}
	path := "/internal/v1/ai-dossier-management"
	query := "?kind=publication&userId=0&themeId=1&version=1"
	preview := conversationData[dto.AIDossierManagementInspect](t, call(t, keys[0], "GET", path+query, nil, 200))
	request := dossier.ManagementRequest{RequestID: "test_management_publish_001", Object: dossier.ManagementObject{Kind: "publication", ThemeID: 1, Version: 1}, VersionDigest: preview.VersionDigest, Enabled: true, ValidUntil: time.Now().Add(10 * time.Minute).UnixMilli(), ReviewReference: "test-fixture:management-http"}
	t.Run("独立角色与候选预览", func(t *testing.T) {
		if preview.Managed || preview.Revision != 0 || preview.SourceState != "available" || preview.Preview == nil {
			t.Fatal("候选预览不符")
		}
		call(t, keys[1], "GET", path+query, nil, 403)
		call(t, "", "GET", path+query, nil, 401)
		call(t, tokens[0], "GET", path+query, nil, 401)
		call(t, keys[0], "GET", "/internal/v1/ai-sources/1", nil, 404)
	})
	if !t.Run("发布接管与原请求重放", func(t *testing.T) {
		first := conversationData[dto.AIDossierManagementReceipt](t, call(t, keys[0], "POST", path, request, 200))
		again := conversationData[dto.AIDossierManagementReceipt](t, call(t, keys[0], "POST", path, request, 200))
		if first != again || first.Revision != 1 {
			t.Fatal("发布未恰好一次")
		}
		public(t, tokens[0], 200)
		call(t, keys[2], "POST", path, request, 409)
		changed := request
		changed.Enabled = false
		changed.ValidUntil = 0
		call(t, keys[0], "POST", path, changed, 409)
		call(t, keys[0], "GET", path+"/requests/"+request.RequestID, nil, 200)
		call(t, keys[2], "GET", path+"/requests/"+request.RequestID, nil, 404)
		if e.queryInt("SELECT COUNT(*) FROM tm_ai_dossier_management_events") != 1 {
			t.Fatal("重放新增审计")
		}
	}) {
		return
	}
	if !t.Run("来源撤回拒批但可撤回发布", func(t *testing.T) {
		e.mustExec("UPDATE tm_ai_sources SET enabled=0 WHERE domain='dossier.example.test'")
		value := conversationData[dto.AIDossierManagementInspect](t, call(t, keys[0], "GET", path+query, nil, 200))
		if value.Preview != nil || value.SourceState != "unavailable" {
			t.Fatal("来源停用后仍返回预览")
		}
		blocked := request
		blocked.RequestID = "test_management_blocked_002"
		blocked.ExpectedRevision = 1
		call(t, keys[0], "POST", path, blocked, 404)
		revoke := request
		revoke.RequestID = "test_management_revoke_002"
		revoke.ExpectedRevision = 1
		revoke.Enabled = false
		revoke.ValidUntil = 0
		call(t, keys[0], "POST", path, revoke, 200)
		public(t, tokens[0], 404)
		call(t, keys[0], "POST", path, request, 200)
		public(t, tokens[0], 404)
		e.mustExec("UPDATE tm_ai_sources SET enabled=1 WHERE domain='dossier.example.test'")
		restore := request
		restore.RequestID = "test_management_restore_003"
		restore.ExpectedRevision = 2
		call(t, keys[0], "POST", path, restore, 200)
		public(t, tokens[0], 200)
	}) {
		return
	}
	t.Run("授予与撤回第二会员", func(t *testing.T) {
		public(t, tokens[1], 404)
		grant := request
		grant.RequestID = "test_management_member_001"
		grant.Object.Kind = "member_read"
		grant.Object.UserID = 2
		grant.ValidUntil -= 60000
		grant.ExpectedRevision = 0
		call(t, keys[0], "POST", path, grant, 200)
		public(t, tokens[1], 200)
		public(t, tokens[0], 200)
		grant.RequestID = "test_management_member_002"
		grant.ExpectedRevision = 1
		grant.Enabled = false
		grant.ValidUntil = 0
		call(t, keys[0], "POST", path, grant, 200)
		public(t, tokens[1], 404)
	})
	t.Run("并发修订仅一次", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan int, 2)
		for i, id := range []string{"test_management_race_a", "test_management_race_b"} {
			wg.Add(1)
			go func(i int, id string) {
				defer wg.Done()
				r := request
				r.RequestID = id
				r.ExpectedRevision = 3
				raw, err := json.Marshal(r)
				if err != nil {
					results <- 0
					return
				}
				req, err := http.NewRequestWithContext(e.ctx, "POST", base+path, bytes.NewReader(raw))
				if err != nil {
					results <- 0
					return
				}
				req.Header.Set("Authorization", "Bearer "+keys[i*2])
				req.Header.Set("Content-Type", "application/json")
				res, err := p.client.Do(req)
				if err != nil {
					results <- 0
					return
				}
				res.Body.Close()
				results <- res.StatusCode
			}(i, id)
		}
		wg.Wait()
		close(results)
		ok, conflict := 0, 0
		for status := range results {
			if status == 200 {
				ok++
			}
			if status == 409 {
				conflict++
			}
		}
		if ok != 1 || conflict != 1 {
			t.Fatal("并发修订未串行化", ok, conflict)
		}
	})
	t.Run("运行账号无业务及审计删除权限", func(t *testing.T) {
		for _, q := range []string{"SELECT password FROM addon_member", "UPDATE tm_ai_sources SET enabled=enabled", "UPDATE tm_points_accounts SET updated_at=updated_at", "DELETE FROM tm_ai_dossier_management_events WHERE 1=0", "DELETE FROM tm_ai_dossier_management_heads WHERE 1=0", "UPDATE tm_ai_dossier_versions SET digest=digest"} {
			_, err := operator.ExecContext(e.ctx, q)
			var denied *mysql.MySQLError
			if !errors.As(err, &denied) || (denied.Number != 1142 && denied.Number != 1143) {
				t.Fatal("管理账号越出明确表权限")
			}
		}
	})
	t.Run("审计写权限失败原子回滚", func(t *testing.T) {
		account, err := mysql.ParseDSN(dsn)
		if err != nil {
			t.Fatal("管理账号解析失败")
		}
		before := e.queryInt("SELECT COUNT(*) FROM tm_ai_dossier_management_events")
		e.mustExec("REVOKE INSERT ON " + e.database + ".tm_ai_dossier_management_events FROM '" + account.User + "'@'127.0.0.1'")
		defer e.mustExec("GRANT INSERT ON " + e.database + ".tm_ai_dossier_management_events TO '" + account.User + "'@'127.0.0.1'")
		pending := request
		pending.RequestID = "test_management_audit_denied"
		pending.ExpectedRevision = 4
		call(t, keys[0], "POST", path, pending, 503)
		if e.queryInt("SELECT COUNT(*) FROM tm_ai_dossier_management_events") != before || e.queryInt("SELECT revision FROM tm_ai_dossier_management_heads WHERE kind='publication' AND theme_id=1 AND version_no=1") != 4 {
			t.Fatal("失败审计留下部分修订或许可")
		}
		public(t, tokens[0], 200)
	})
	t.Run("会员读取依赖撤回恢复", func(t *testing.T) {
		before := e.queryInt("SELECT COUNT(*) FROM tm_ai_dossier_management_events")
		e.mustExec("REVOKE SELECT ON " + e.database + ".tm_ai_dossier_management_events FROM '" + p.apiUser + "'@'127.0.0.1'")
		public(t, tokens[0], 503)
		e.mustExec("GRANT SELECT ON " + e.database + ".tm_ai_dossier_management_events TO '" + p.apiUser + "'@'127.0.0.1'")
		public(t, tokens[0], 200)
		if e.queryInt("SELECT COUNT(*) FROM tm_ai_dossier_management_events") != before {
			t.Fatal("复读新增审计")
		}
	})
	t.Run("许可旁路及失效保护拒绝", func(t *testing.T) {
		_, err := e.db.ExecContext(e.ctx, "UPDATE tm_ai_dossier_publications SET approval_ref='legacy-bypass' WHERE theme_id=1 AND version_no=1")
		var blocked *mysql.MySQLError
		if !errors.As(err, &blocked) || blocked.Number != 1644 {
			t.Fatal("已管理许可可剥离审计引用")
		}
		e.mustExec("DROP TRIGGER tm_ai_dossier_managed_publication_update")
		e.mustExec("CREATE TRIGGER tm_ai_dossier_managed_publication_update BEFORE UPDATE ON tm_ai_dossier_publications FOR EACH ROW SET @tm_management_probe=1")
		if repository.NewAIDossierManagementSchemaVerifier(e.db).Verify(e.ctx) == nil {
			t.Fatal("仅同名的失效保护被接受")
		}
		reportDDL(t, e.pointsTestEnv, "20261103_create_tm_ai_dossier_management.sql")
		if err := repository.NewAIDossierManagementSchemaVerifier(e.db).Verify(e.ctx); err != nil {
			t.Fatal("保护恢复后不能通过")
		}
		public(t, tokens[0], 200)
	})
	p.verifyNoGeneration(t)
	p.login.finishAs(t, e.ctx, false)
}
