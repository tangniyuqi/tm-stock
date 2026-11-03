//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

type localProcessQA struct {
	path     string
	file     config.AILocalQA
	server   *providertest.Server
	external bool
	wire     bool
}

func TestIntegrationAIReportLocalQAProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					verifyLocalAPIWorker(t, level, kind, api, worker, login, localProcessCase{qa: true})
				})
			}
		})
	}
}
func localQAProcedure(e *workflowEnv, body bool) {
	column := "action_statement"
	if !body {
		column = "''"
	}
	e.mustExec(`CREATE PROCEDURE tm_ai_local_qa_trigger_metadata() SQL SECURITY DEFINER READS SQL DATA SELECT trigger_name,event_object_table,event_manipulation,action_timing,` + column + ` FROM information_schema.triggers WHERE trigger_schema='` + e.database + `' AND event_object_table IN ('tm_ai_qa_scopes','tm_ai_qa_limits','tm_ai_qa_quotas','tm_ai_qa_requests','tm_ai_qa_calls')`)
}
func prepareLocalQASchema(t *testing.T, f *localPublicationFixture, dsn string) {
	t.Helper()
	e := f.env
	reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_qa.sql")
	localQAProcedure(e, true)
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("本轮运行账号无效")
	}
	for _, table := range []string{"tm_ai_qa_scopes", "tm_ai_qa_limits", "tm_ai_qa_quotas", "tm_ai_qa_requests", "tm_ai_qa_calls"} {
		e.mustExec(`GRANT SELECT,INSERT,UPDATE,DELETE ON ` + e.database + `.` + table + ` TO '` + cfg.User + `'@'127.0.0.1'`)
	}
	e.mustExec(`GRANT EXECUTE ON PROCEDURE ` + e.database + `.tm_ai_local_qa_trigger_metadata TO '` + cfg.User + `'@'127.0.0.1'`)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("本轮问答账号无效")
	}
	defer db.Close()
	verify := repository.NewAILocalQASchemaVerifier(db, f.runtime.InstanceID)
	if err := verify.Verify(e.ctx); err != nil {
		t.Fatal("受限问答探针未通过", err)
	}
	e.mustExec(`DROP PROCEDURE tm_ai_local_qa_trigger_metadata`)
	if err := verify.Verify(e.ctx); err == nil {
		t.Fatal("缺过程未拒绝")
	}
	localQAProcedure(e, false)
	e.mustExec(`GRANT EXECUTE ON PROCEDURE ` + e.database + `.tm_ai_local_qa_trigger_metadata TO '` + cfg.User + `'@'127.0.0.1'`)
	if err := verify.Verify(e.ctx); err == nil {
		t.Fatal("伪造空定义未拒绝")
	}
	e.mustExec(`DROP PROCEDURE tm_ai_local_qa_trigger_metadata`)
	localQAProcedure(e, true)
	e.mustExec(`GRANT EXECUTE ON PROCEDURE ` + e.database + `.tm_ai_local_qa_trigger_metadata TO '` + cfg.User + `'@'127.0.0.1'`)
	if err := verify.Verify(e.ctx); err != nil {
		t.Fatal("恢复原定义未通过", err)
	}
}

func prepareLocalProcessQA(t *testing.T, f *localPublicationFixture, dsn string, endpoints ...string) *localProcessQA {
	t.Helper()
	prepareLocalQASchema(t, f, dsn)
	e := f.env
	snap := e.planner.p.Snapshots[0]
	steps := append(qaSuccessSteps(snap.ID), qaSuccessSteps(1)...)
	upstream := providertest.NewServer(t, steps...)
	m := config.AILocalQAModel{Endpoint: config.AILocalExecutionModel{Name: "本地问答协议", BaseURL: upstream.URL(), Model: "qa-local-process", TimeoutMS: 20000}, FilingNo: "local-eval:filing", ApprovalRef: "local-eval:model", QualityRef: "local-eval:quality", BudgetRef: "local-eval:qa-budget"}
	if len(endpoints) > 1 {
		t.Fatal("每次问答仅允许一个明确本地模型出口")
	}
	external := len(endpoints) == 1
	if external {
		if _, err := h5TestOrigin(strings.TrimSuffix(endpoints[0], "/v1")); err != nil || !strings.HasSuffix(endpoints[0], "/v1") {
			t.Fatal("真实问答仅接受原试跑的回环出口")
		}
		m.Endpoint = config.AILocalExecutionModel{Name: "本次真实问答桥", BaseURL: endpoints[0], Model: os.Getenv("TM_TEST_AI_REPORT_MODEL"), TimeoutMS: 20000}
	}
	direct, err := provider.NewDirect(m.ProviderConfig(), true)
	if err != nil {
		t.Fatal(err)
	}
	m.ProfileDigest = direct.DirectProfileDigest()
	c := config.AILocalQA{Version: 1, InstanceID: f.runtime.InstanceID, Revision: "v1", Scope: "synthetic-loopback-only", IssuedAtMS: time.Now().UnixMilli() - 1000, ExpiresAtMS: f.file.ExpiresAtMS, Members: []int64{1}, Subjects: []int64{1}, Models: []config.AILocalQAModel{m, m, m}, AllowedDomains: append([]string(nil), f.spec.AllowedDomains...), Limits: config.AILocalQALimits{MaxSources: 4, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, TimeoutMS: 20000}}
	qaCfg, err := c.QAConfig()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := qaCfg.PolicyDigest()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := json.Marshal([]qa.EvidenceProof{{ID: snap.ID, Hash: snap.Hash}})
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(`INSERT INTO tm_ai_qa_scopes(subject_type,subject_id,subject_name,approval_ref,policy_digest,evidence_json,valid_until,enabled,version,created_at,updated_at) VALUES('theme',1,?,?,?,?,?,0,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, f.spec.SubjectName, "local-eval:qa-scope", digest, evidence, time.UnixMilli(c.ExpiresAtMS).UTC().Format("2006-01-02 15:04:05.000"))
	e.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
	e.mustExec(`INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,?,0,10,20,1000000,2000000,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.BudgetRef)
	e.mustExec(`UPDATE tm_ai_qa_limits SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
	path := filepath.Join(t.TempDir(), "qa.json")
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return &localProcessQA{path: path, file: c, server: upstream, external: external}
}
func verifyLocalProcessQA(t *testing.T, f *localPublicationFixture, q *localProcessQA, client *http.Client, base, root string, taskID int64, tokens []string) {
	t.Helper()
	e := f.env
	before := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
	beforeAgent, beforeModel := e.submits.Load(), len(e.server.Requests())
	beforeTasks := e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`)
	call := func(method, path, key, token, body string, want int) json.RawMessage {
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
		// 与前端MemberApi.aiQuestion一致：提交28秒、读取10秒；服务端及模型期限不变。
		qaClient := *client
		qaClient.Timeout = 10 * time.Second
		if method == http.MethodPost {
			qaClient.Timeout = 28 * time.Second
		}
		started := time.Now()
		res, err := qaClient.Do(r)
		if err != nil {
			var network net.Error
			// 只保留固定分类与相对路由，不能输出含凭据的请求或原始网络错误。
			t.Fatalf("实际问答HTTP请求失败：method=%s path=%s deadline=%t canceled=%t network_timeout=%t fixture_canceled=%t elapsed_ms=%d mock_qa_calls=%d", method, path, errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled), errors.As(err, &network) && network.Timeout(), e.ctx.Err() != nil, time.Since(started).Milliseconds(), q.server.Count())
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(raw) > 65536 || res.StatusCode != want {
			t.Logf("原问答失败审计：requests=%d calls=%d reserved=%d invalid_metadata=%d invalid_protocol=%d failed=%d", e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls WHERE state='reserved'`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls WHERE JSON_UNQUOTE(JSON_EXTRACT(result_json,'$.Failure'))='invalid_metadata'`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls WHERE JSON_UNQUOTE(JSON_EXTRACT(result_json,'$.Failure'))='invalid_protocol'`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests WHERE state='failed'`))
			t.Fatalf("问答HTTP不符：%s %s got=%d want=%d", method, path, res.StatusCode, want)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("问答响应允许缓存")
		}
		for _, secret := range tokens {
			if strings.Contains(string(raw), secret) {
				t.Fatal("问答响应泄漏凭据")
			}
		}
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &envelope) != nil || (want == 200) != (envelope.Code == 0) {
			t.Fatal("问答响应契约不符")
		}
		return envelope.Data
	}
	for i, skill := range []string{"material_qa", "report_qa"} {
		body := fmt.Sprintf(`{"version":%d,"subjectType":"theme","subjectId":1,"skill":"%s","question":"所选资料的主题是什么？"`, i+2, skill)
		if skill == "report_qa" {
			body += fmt.Sprintf(`,"taskId":%d`, taskID)
		}
		body += "}"
		key := fmt.Sprintf("local_process_qa_question_%d", i)
		var status dto.AIQAStatus
		if json.Unmarshal(call("POST", root+"/questions", key, tokens[0], body, 200), &status) != nil || status.State != "answered" || status.Seq != int64(i+2) {
			t.Fatalf("实际问答未交付：skill=%s answered=%t refused=%t failed=%t seq=%d", skill, status.State == "answered", status.State == "refused", status.State == "failed", status.Seq)
		}
		call("POST", root+"/questions", key, tokens[0], body, 200)
		path := fmt.Sprintf("%s/questions/%d/answer", root, status.Seq)
		var answer dto.AIQAAnswer
		if json.Unmarshal(call("GET", path, "", tokens[0], "", 200), &answer) != nil || len(answer.Quotes) < 1 || len(answer.Quotes) > 4 || answer.Template == "" || !answer.AIGenerated {
			t.Fatal("实际问答缺逐字引文或模板")
		}
		call("GET", path, "", tokens[0], "", 200)
		call("GET", path, "", tokens[1], "", 404)
		call("GET", path, "", "", "", 401)
		evidencePath := fmt.Sprintf("%s/questions/%d/evidence", root, status.Seq)
		for repeat := 0; repeat < 2; repeat++ {
			var evidence dto.AIQAEvidence
			if json.Unmarshal(call("GET", evidencePath, "", tokens[0], "", 200), &evidence) != nil || evidence.Seq != status.Seq || evidence.Skill != skill || evidence.SubjectID != 1 || len(evidence.Sources) == 0 || len(evidence.Calls) != 3 || evidence.PromptVersion != qa.Version || skill == "material_qa" && evidence.TaskID != 0 || skill == "report_qa" && evidence.TaskID != taskID {
				t.Fatal("原API候选资料/调用/任务绑定不符")
			}
			for _, row := range evidence.Calls {
				if row.Status != "confirmed" || row.InputTokens == nil || row.OutputTokens == nil || row.CostMicros != nil {
					t.Fatal("实际用量或未知账单投影不符")
				}
			}
		}
		call("GET", evidencePath, "", tokens[1], "", 404)
		call("GET", evidencePath, "", "", "", 401)
	}
	e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
	call("GET", root+"/questions/2/answer", "", tokens[0], "", 503)
	call("GET", root+"/questions/3/answer", "", tokens[0], "", 503)
	call("GET", root+"/questions/2/evidence", "", tokens[0], "", 503)
	call("GET", root+"/questions/3/evidence", "", tokens[0], "", 503)
	e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
	call("GET", root+"/questions/2/answer", "", tokens[0], "", 200)
	call("GET", root+"/questions/3/answer", "", tokens[0], "", 200)
	call("GET", root+"/questions/2/evidence", "", tokens[0], "", 200)
	call("GET", root+"/questions/3/evidence", "", tokens[0], "", 200)
	var originalEvidence string
	if err := e.db.QueryRowContext(e.ctx, `SELECT evidence_json FROM tm_ai_qa_scopes WHERE subject_type='theme' AND subject_id=1`).Scan(&originalEvidence); err != nil {
		t.Fatal(err)
	}
	e.mustExec(`UPDATE tm_ai_qa_scopes SET evidence_json='[]',version=version+1,updated_at=UTC_TIMESTAMP(3)`)
	var refused dto.AIQAStatus
	if json.Unmarshal(call("POST", root+"/questions", "local_process_qa_no_evidence", tokens[0], `{"version":4,"subjectType":"theme","subjectId":1,"skill":"material_qa","question":"示例资料有什么记载？"}`, 200), &refused) != nil || refused.State != "refused" || refused.Seq != 4 {
		t.Fatal("无证据未固定拒答")
	}
	e.mustExec(`UPDATE tm_ai_qa_scopes SET evidence_json=?,version=version+1,updated_at=UTC_TIMESTAMP(3)`, originalEvidence)
	if q.external {
		// 导出前沿原会员接口重新读取两题的全部候选和调用；不复用撤权前缓存。
		rows := make([]dto.AIQAEvidence, 0, 2)
		for _, seq := range []int64{2, 3} {
			var evidence dto.AIQAEvidence
			path := fmt.Sprintf("%s/questions/%d/evidence", root, seq)
			if json.Unmarshal(call("GET", path, "", tokens[0], "", 200), &evidence) != nil || evidence.Seq != seq || len(evidence.Calls) != 3 {
				t.Fatal("导出前原问答当前权限复读失败")
			}
			rows = append(rows, evidence)
		}
		output := os.Getenv("TM_TEST_AI_FOLLOWUP_OUTPUT")
		info, err := os.Lstat(output)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || filepath.Dir(output) != os.TempDir() || !regexp.MustCompile(`^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$`).MatchString(filepath.Base(output)) {
			t.Fatal("联合试跑证据仅能写入本次独占临时目录")
		}
		raw, err := json.MarshalIndent(struct {
			Classification string             `json:"classification"`
			TaskID         int64              `json:"taskId"`
			ReadAtMS       int64              `json:"readAtMs"`
			Evidence       []dto.AIQAEvidence `json:"evidence"`
		}{"synthetic-live-current-read-evidence-not-human-gold", taskID, time.Now().UnixMilli(), rows}, "", "  ")
		if err != nil {
			t.Fatal("联合问答证据编码失败")
		}
		file, err := os.OpenFile(filepath.Join(output, "followup-api-evidence.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("联合问答证据不允许覆盖或写入失败")
		}
		_, writeErr := file.Write(raw)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal("联合问答证据未确认落盘")
		}
	}
	wantMockQA := 6
	if q.external {
		wantMockQA = 0
	}
	if q.server.Count() != wantMockQA || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls`) != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != before || len(e.server.Requests()) != beforeModel || e.submits.Load() != beforeAgent || e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`) != beforeTasks {
		t.Fatal("问答重放增加调用、记录或改账")
	}
	if q.wire && !q.external {
		verifyFollowupWire(t, q.server.Requests())
	}
	q.file.Revision = "withdrawn"
	raw, err := json.Marshal(q.file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(q.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	call("GET", root+"/questions/2/answer", "", tokens[0], "", 503)
	call("GET", root+"/questions/3/answer", "", tokens[0], "", 503)
	if q.server.Count() != wantMockQA {
		t.Fatal("配置撤回引发额外调用")
	}
	call("GET", root+"/questions/2/evidence", "", tokens[0], "", 503)
	call("GET", root+"/questions/3/evidence", "", tokens[0], "", 503)
	if q.external {
		t.Log("TM_STOCK_LIVE_QA_FOLLOWUP_VERIFIED")
	} else {
		t.Log("实际API资料问答/报告追问、原存证复读、越权与本地配置撤回拒读通过；回环模型六次、无新增任务/积分流水")
	}
}

// 共享协议样例来自原API真实发送的合成HTTP字节，不是人工质量金标。
func verifyFollowupWire(t *testing.T, recorded []providertest.Recorded) {
	t.Helper()
	requests := make([]json.RawMessage, 0, len(recorded))
	for _, row := range recorded {
		if row.AuthDigest != "" || !json.Valid(row.Body) {
			t.Fatal("问答协议样例含凭据或无效正文")
		}
		requests = append(requests, json.RawMessage(row.Body))
	}
	expected, err := os.ReadFile(filepath.Join("..", "..", "..", "eval", "agents", "local-followup-wire.json"))
	var fixture []json.RawMessage
	if err != nil || json.Unmarshal(expected, &fixture) != nil || len(fixture) != 6 || len(requests) != len(fixture) {
		t.Fatal("原Go问答协议样例缺失或不完整")
	}
	for i, row := range requests {
		var got, want any
		if json.Unmarshal(row, &got) != nil || json.Unmarshal(fixture[i], &want) != nil || !reflect.DeepEqual(got, want) {
			// 失败时保留实际合成请求供跨语言复核；不自动更新样例或把采集记作通过。
			capture, err := os.MkdirTemp("", "tm-stock-qa-wire-mismatch-")
			if err == nil {
				raw, encodeErr := json.Marshal(requests)
				if encodeErr == nil && os.WriteFile(filepath.Join(capture, "requests.json"), raw, 0600) == nil {
					t.Logf("实际合成问答协议已隔离保存：%s", capture)
				}
			}
			t.Fatalf("原Go实际问答HTTP请求与共同样例不符：step=%d", i+1)
		}
	}
}
