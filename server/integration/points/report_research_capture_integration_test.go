//go:build integration

package points_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type reportCaptureModels struct {
	reportQAModels
	calls int
}

func (m *reportCaptureModels) ForPlan(ctx context.Context, proof qa.Proof) ([3]*provider.OpenAICompat, error) {
	m.calls++
	return m.reportQAModels.ForPlan(ctx, proof)
}

type reportCaptureReadControl struct {
	service.AIReportReadControl
	allowed atomic.Bool
}

type reportCaptureEvidence struct {
	service.AIReportQAEvidenceSource
	calls, elapsed, longest, deadlines atomic.Int64
}

func (s *reportCaptureEvidence) ForQA(ctx context.Context, uid, taskID int64) (qa.ReportEvidence, error) {
	start := time.Now()
	v, err := s.AIReportQAEvidenceSource.ForQA(ctx, uid, taskID)
	n := time.Since(start).Milliseconds()
	s.calls.Add(1)
	s.elapsed.Add(n)
	for old := s.longest.Load(); n > old; old = s.longest.Load() {
		if s.longest.CompareAndSwap(old, n) {
			break
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		s.deadlines.Add(1)
	}
	return v, err
}

func (c *reportCaptureReadControl) Recheck(ctx context.Context, p service.AIReportPublication) error {
	if !c.allowed.Load() {
		return finalreport.ErrUnavailable
	}
	return c.AIReportReadControl.Recheck(ctx, p)
}

// 只读适配原cmd/api，令牌仅在回环请求头；不替代该进程的报告/当前授权核验。
type reportCaptureAPIReader struct {
	base   string
	client *http.Client
	tokens map[int64]string
	gate   *reportCaptureReadControl
}

func (s *reportCaptureAPIReader) Read(ctx context.Context, uid, id int64) (dto.AIReport, error) {
	if !s.gate.allowed.Load() || s.tokens[uid] == "" || id <= 0 {
		return dto.AIReport{}, finalreport.ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, "GET", s.base+"/api/v1/ai/tasks/"+qaID(id)+"/report", nil)
	if err != nil {
		return dto.AIReport{}, finalreport.ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+s.tokens[uid])
	res, err := s.client.Do(req)
	if err != nil {
		return dto.AIReport{}, finalreport.ErrUnavailable
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 256*1024+1))
	if err != nil || len(raw) > 256*1024 || res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		return dto.AIReport{}, finalreport.ErrUnavailable
	}
	var out struct {
		Code int
		Msg  string
		Data dto.AIReport
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var trailing any
	if decoder.Decode(&out) != nil || decoder.Decode(&trailing) != io.EOF || out.Code != 0 || out.Data.TaskID != id || !s.gate.allowed.Load() {
		return dto.AIReport{}, finalreport.ErrUnavailable
	}
	return out.Data, nil
}

func reportCaptureEnvironment(t *testing.T, level sql.IsolationLevel, profile, api string) (*workflowEnv, *reportHTTPEnv, service.AIReportReader, *reportCaptureReadControl) {
	t.Helper()
	var e *workflowEnv
	var web *reportHTTPEnv
	var publication service.AIReportReadControl
	var policies service.AIReportReadPolicySource
	var fixture *dossierWorkflowFixture
	if profile == "five" {
		e = newWorkflowEnv(t, level, "pass")
		if _, err := e.worker(t, true).Run(e.ctx, 1, e.v.ID); err != nil {
			t.Fatal("原报告执行失败", err)
		}
		web = reportHTTPFixtureWithModel(t, e.reportEnv, "", true, "测试模型", "test-only-filing")
		publication = web.control
	} else {
		kind, mode := 1, "partial"
		if profile == "seven" {
			kind, mode = 2, "comparison_supported"
		}
		f := newDossierWorkflowFixture(t, level, kind, mode)
		fixture = f
		e, web = f.env, f.web
		if _, err := f.worker.Run(e.ctx, 1, f.id); err != nil {
			t.Fatal("原专题报告执行失败", err)
		}
		control, err := service.NewAILocalReportReadControl(f.admission)
		if err != nil {
			t.Fatal(err)
		}
		publication, policies = control, control
	}
	gate := &reportCaptureReadControl{AIReportReadControl: publication}
	gate.allowed.Store(true)
	base, err := service.NewAIReportService(e.repo, e.ledger, e.policy, false)
	if err != nil {
		t.Fatal(err)
	}
	var reader service.AIReportReader
	if policies == nil {
		reader, err = service.NewAIReportReadService(base, gate, true)
	} else {
		reader, err = service.NewAIReportReadServiceWithPolicySource(base, policies, gate, true)
	}
	if err != nil {
		t.Fatal(err)
	}
	if fixture != nil {
		// QA核与存证仍由父进程-race检查；复杂报告读取使用实际API的正常构建。
		localAPISchema(t, e)
		e.mustExec(`ALTER TABLE addon_member ADD COLUMN password varchar(255) NULL`)
		dsn := localAPIDSN(t, e)
		grantDossierProcessPermissions(t, e, dsn)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		fixture.runtime.Listen = listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "runtime.json")
		raw, err := json.Marshal(fixture.runtime)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 11 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		t.Cleanup(client.CloseIdleConnections)
		address := "http://" + fixture.runtime.Listen
		startLocalAPIConfigured(t, e, api, dsn, web.key, file, fixture.path, client, address, profile == "seven", "", false, "")
		reader = &reportCaptureAPIReader{base: address, client: client, tokens: map[int64]string{1: web.token(t, 1, "client"), 2: web.token(t, 2, "client")}, gate: gate}
	}
	r, err := reader.Read(e.ctx, 1, e.v.ID)
	if err != nil || profile == "five" && r.DossierReview != nil || profile != "five" && r.DossierReview == nil || profile == "seven" && r.DossierReview.Comparison == nil {
		t.Fatal("原五/六/七步当前报告读取不符", err)
	}
	if _, err := reader.Read(e.ctx, 2, e.v.ID); err == nil || e.queryInt(`SELECT COUNT(*) FROM tm_points_accounts WHERE user_id=2`) != 0 {
		t.Fatal("报告读取没有隔离会员或创建了额外积分账户")
	}
	return e, web, reader, gate
}

// 原报告执行/提交、会员JWT、问答写入及当前授权读取实际走SQL；模型为显著协议替身。
func TestIntegrationAIReportResearchCapture(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, _, _ := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, profile := range []string{"five", "six", "seven"} {
				t.Run(profile, func(t *testing.T) {
					e, web, reader, publication := reportCaptureEnvironment(t, level, profile, api)
					// 新组合夹具含三题提交及多批双遍采集；不修改SQL、模型或原API的业务时限。
					ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
					t.Cleanup(cancel)
					e.pointsTestEnv.ctx = ctx
					reportEvidence, err := service.NewAIReportQAEvidence(reader)
					if err != nil {
						t.Fatal(err)
					}
					evidence := &reportCaptureEvidence{AIReportQAEvidenceSource: reportEvidence}
					ev, err := evidence.ForQA(e.ctx, 1, e.v.ID)
					if err != nil {
						t.Fatal(err)
					}
					scope, err := qa.BuildReportScope(ev, time.Now())
					if err != nil || len(scope.Snapshots) < 3 {
						t.Fatal("原报告追问范围未按基础AI引文建立", err)
					}
					snapshot, sentence := scope.Snapshots[0], scope.Snapshots[0].Sentences[0]
					if profile == "five" {
						reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_conversations.sql")
					}
					reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_qa.sql")
					e.mustExec(`INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,'test-only-budget',0,10,20,1000000,2000000,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`)
					e.mustExec(`UPDATE tm_ai_qa_limits SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
					convs, err := service.NewAIConversationService(repository.NewAIConversationRepo(e.db))
					if err != nil {
						t.Fatal(err)
					}
					conv, err := convs.Create(e.ctx, 1, "test_report_capture_conversation", "明确标记的报告采集测试")
					if err != nil {
						t.Fatal(err)
					}
					steps := append(qaSuccessSteps(1), providertest.Success(`{"action":"refuse","reason":"no_evidence","citations":[]}`, provider.Usage{Prompt: 20, Completion: 10, Total: 30}), providertest.Drop())
					upstream := providertest.NewServer(t, steps...)
					direct, err := provider.NewDirect(provider.Config{Name: "工程报告追问协议", BaseURL: upstream.URL(), Model: "report-qa-capture-test", APIKey: "test-only", TimeoutMS: 20000}, true)
					if err != nil {
						t.Fatal(err)
					}
					model := qa.Model{Name: "report-qa-capture-test", FilingNo: "test-only-filing", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", BudgetRef: "test-only-budget", ProfileDigest: direct.DirectProfileDigest()}
					cfg := qa.Config{Selector: model, Entailment: model, Semantic: model, Limits: qa.Limits{MaxSources: 16, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second}}
					verifier, err := memberauth.NewVerifier(memberauth.Config{SigningKey: web.key, Issuer: "GVA", Audience: "GVA"})
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
					models := &reportCaptureModels{reportQAModels: reportQAModels{direct}}
					writes, err := service.NewAIQAServiceWithReports(repository.NewAIQARepo(e.db), reportQAPolicies{}, models, sessions, true, evidence, reportQAConfig{cfg})
					if err != nil {
						t.Fatal(err)
					}
					writeMux := http.NewServeMux()
					handler.NewAIQAHandler(writes).Register(writeMux, auth, true)
					writeHTTP := httptest.NewServer(writeMux)
					t.Cleanup(writeHTTP.Close)
					token := web.token(t, 1, "client")
					root, err := filepath.Abs(filepath.Join("..", "..", ".."))
					if err != nil {
						t.Fatal(err)
					}
					dir := t.TempDir()
					corpus, manifest := filepath.Join(dir, "cases.jsonl"), filepath.Join(dir, "binding.json")
					var corpusText bytes.Buffer
					bindings := []map[string]any{}
					for i, question := range []string{"报告怎样定义本题材？", "报告是否记载未提供的资料？", "报告中还记载哪些资料？"} {
						input := dto.AIQAInput{Version: int64(i + 1), SubjectType: "theme", SubjectID: ev.SubjectID, Skill: "report_qa", TaskID: e.v.ID, Question: question}
						body, err := json.Marshal(input)
						if err != nil {
							t.Fatal(err)
						}
						req, err := http.NewRequestWithContext(e.ctx, "POST", writeHTTP.URL+"/api/v1/ai/conversations/"+qaID(conv.ID)+"/questions", bytes.NewReader(body))
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Set("Authorization", "Bearer "+token)
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("Idempotency-Key", fmt.Sprintf("test_report_capture_question_%d", i))
						res, err := http.DefaultClient.Do(req)
						if err != nil {
							t.Fatal("原报告问题HTTP提交失败")
						}
						response, readErr := io.ReadAll(io.LimitReader(res.Body, 65537))
						res.Body.Close()
						want := 200
						if i == 2 {
							want = 503
						}
						accepted := readErr == nil && len(response) <= 65536 && res.StatusCode == want
						if !accepted && readErr == nil && len(response) <= 65536 && want == 200 && res.StatusCode == 503 {
							// 对齐原页面的提交未知恢复：只查原键收据，不重提问题或重跑模型。
							recoverRequest, err := http.NewRequestWithContext(e.ctx, "GET", writeHTTP.URL+"/api/v1/ai/conversations/"+qaID(conv.ID)+"/question-receipt", nil)
							if err != nil {
								t.Fatal(err)
							}
							recoverRequest.Header.Set("Authorization", "Bearer "+token)
							recoverRequest.Header.Set("Idempotency-Key", req.Header.Get("Idempotency-Key"))
							recovered, err := http.DefaultClient.Do(recoverRequest)
							if err != nil {
								t.Fatal("原键收据读取失败，不重提")
							}
							var receipt struct {
								Code int
								Data dto.AIQAStatus
							}
							decodeErr := json.NewDecoder(io.LimitReader(recovered.Body, 65536)).Decode(&receipt)
							recovered.Body.Close()
							state := "answered"
							if i == 1 {
								state = "refused"
							}
							accepted = decodeErr == nil && recovered.StatusCode == 200 && receipt.Code == 0 && receipt.Data.Seq == int64(i+1) && receipt.Data.State == state && receipt.Data.Skill == "report_qa" && receipt.Data.SubjectID == ev.SubjectID && receipt.Data.Question == question
							if accepted {
								t.Log("原提交503后经原键恢复已存终态；答案仍须当前授权读取，未重提模型")
							}
						}
						if !accepted {
							t.Fatalf("原报告问题HTTP及原键恢复不符：序号%d 状态%d 工厂%d 调用%d 问题%d 报告复核%d/累计%dms/最长%dms/超时%d", i+1, res.StatusCode, models.calls, upstream.Count(), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`), evidence.calls.Load(), evidence.elapsed.Load(), evidence.longest.Load(), evidence.deadlines.Load())
						}
						id := fmt.Sprintf("report-capture-%d", i+1)
						expected := map[string]any{"state": "refused", "answers": [][]string{}}
						if i == 0 {
							expected = map[string]any{"state": "answered", "answers": [][]string{{"a"}}}
						}
						row := map[string]any{"id": id, "split": "development", "tags": []string{"report-qa", "synthetic"}, "subject": ev.SubjectName, "question": question, "annotation": "synthetic-engineering-unreviewed", "sources": []map[string]any{{"id": "a", "unit": "report-original", "text": sentence.Text, "authorized": true}}, "expected": expected, "basis": "工程协议回归：原报告首条定义有直接依据，无证据问题应拒答；传输失败必须另计，非模型质量金标。"}
						encoded, err := json.Marshal(row)
						if err != nil {
							t.Fatal(err)
						}
						corpusText.Write(encoded)
						corpusText.WriteByte('\n')
						bindings = append(bindings, map[string]any{"id": id, "conversationId": conv.ID, "seq": i + 1, "subjectId": ev.SubjectID, "skill": "report_qa", "taskId": e.v.ID, "sources": []map[string]any{{"ref": "a", "snapshotId": snapshot.ID, "startByte": sentence.StartByte, "endByte": sentence.EndByte, "sourceType": snapshot.SourceCategory, "sourceUrl": snapshot.URL, "collectedAt": snapshot.FetchedAt.UnixMilli()}}})
					}
					if upstream.Count() != 5 || models.calls != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 3 {
						t.Fatal("原报告问题/模型数量不符")
					}
					before := []int{e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls`)}
					readOnly, err := service.NewAIQAServiceWithReportAccess(repository.NewAIQARepo(e.db), nil, nil, sessions, false, false, evidence, reportQAConfig{cfg})
					if err != nil {
						t.Fatal(err)
					}
					readMux := http.NewServeMux()
					handler.NewAIQAHandler(readOnly).Register(readMux, auth, false)
					handler.NewAIConversationHandler(convs).Register(readMux, auth)
					handler.NewAIReportReadHandler(reader).Register(readMux, auth)
					for seq := int64(1); seq <= 3; seq++ {
						v := readQAEvidenceHTTP(t, readMux, token, conv.ID, seq, true)
						assertQAEvidenceScope(t, v, scope, e.v.ID)
						if v.State != []string{"answered", "refused", "failed"}[seq-1] {
							t.Fatal("追问资料记录没有保留原终态")
						}
					}
					var reads, revoke atomic.Int64
					gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != "GET" {
							http.Error(w, "readonly", 405)
							return
						}
						if reads.Add(1) == 12 {
							if revoke.Load() == 1 {
								publication.allowed.Store(false)
							}
							if revoke.Load() == 2 {
								if _, err := e.db.ExecContext(r.Context(), `UPDATE tm_ai_sources SET enabled=0`); err != nil {
									t.Error("来源撤回失败")
									http.Error(w, "test-only", 500)
									return
								}
							}
							if revoke.Load() == 3 {
								if _, err := e.db.ExecContext(r.Context(), `UPDATE tm_ai_dossier_support_uses SET enabled=0`); err != nil {
									t.Error("专题用途撤回失败")
									http.Error(w, "test-only", 500)
									return
								}
							}
							if revoke.Load() == 4 {
								if _, err := e.db.ExecContext(r.Context(), `UPDATE tm_ai_dossier_publications SET enabled=0`); err != nil {
									t.Error("专题发布撤回失败")
									http.Error(w, "test-only", 500)
									return
								}
							}
							if revoke.Load() == 5 {
								if _, err := e.db.ExecContext(r.Context(), `UPDATE tm_ai_dossier_grants SET enabled=0`); err != nil {
									t.Error("会员专题授权撤回失败")
									http.Error(w, "test-only", 500)
									return
								}
							}
						}
						readMux.ServeHTTP(w, r)
					}))
					t.Cleanup(gateway.Close)
					if err := os.WriteFile(corpus, corpusText.Bytes(), 0600); err != nil {
						t.Fatal(err)
					}
					material, err := exec.CommandContext(e.ctx, "node", filepath.Join(root, "scripts/dev/research-eval.cjs"), "materials", corpus, "development").Output()
					if err != nil {
						t.Fatal("工程物料生成失败")
					}
					var meta struct{ DatasetDigest string }
					if err := json.Unmarshal(material, &meta); err != nil {
						t.Fatal(err)
					}
					writeBinding := func() {
						t.Helper()
						raw, err := json.Marshal(map[string]any{"schema": "tm-stock-qa-capture-v2", "datasetDigest": meta.DatasetDigest, "split": "development", "baseURL": gateway.URL, "modelVersion": "loopback-report-only", "promptVersion": "report-capture-test-v1", "bindings": bindings})
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(manifest, raw, 0600); err != nil {
							t.Fatal(err)
						}
					}
					writeBinding()
					run := func(bearer string, want bool) []byte {
						t.Helper()
						cmd := exec.CommandContext(e.ctx, "node", filepath.Join(root, "scripts/dev/research-qa-capture.cjs"), corpus, manifest)
						for _, v := range os.Environ() {
							if !strings.HasPrefix(v, "TM_STOCK_EVAL_TOKEN=") {
								cmd.Env = append(cmd.Env, v)
							}
						}
						cmd.Env = append(cmd.Env, "TM_STOCK_EVAL_TOKEN="+bearer)
						out, err := cmd.Output()
						if (err == nil) != want {
							if exit, ok := err.(*exec.ExitError); ok && captureDiagnosticPattern.Match(exit.Stderr) {
								t.Log(strings.TrimSpace(string(exit.Stderr)))
							}
							t.Fatal("报告回执采集退出状态不符；凭据和正文隐藏")
						}
						if !want && len(out) > 0 {
							t.Fatal("拒绝后仍输出半批回执")
						}
						return out
					}
					out := run(token, true)
					if reads.Load() != 22 {
						t.Fatal("未完成两遍原任务和报告读取", reads.Load())
					}
					for _, secret := range []string{token, "taskId", "conversationId", "snapshotId", "sourceUrl", sentence.Text} {
						if bytes.Contains(out, []byte(secret)) {
							t.Fatal("导出泄漏私有报告或定位")
						}
					}
					record := filepath.Join(dir, "run.json")
					if err := os.WriteFile(record, out, 0600); err != nil {
						t.Fatal(err)
					}
					graded, err := exec.CommandContext(e.ctx, "node", filepath.Join(root, "scripts/dev/research-eval.cjs"), "score", corpus, record).Output()
					var results struct {
						Counts struct{ Samples, Answered, Refused, Failed, CorrectAnswers, CorrectRefusals int }
					}
					if err != nil || json.Unmarshal(graded, &results) != nil || results.Counts.Samples != 3 || results.Counts.Answered != 1 || results.Counts.Refused != 1 || results.Counts.Failed != 1 || results.Counts.CorrectAnswers != 1 || results.Counts.CorrectRefusals != 1 {
						t.Fatal("工程报告回执没有保留正确回答、拒答和实际失败")
					}
					bindings[0]["taskId"] = e.v.ID + 1
					writeBinding()
					run(token, false)
					bindings[0]["taskId"] = e.v.ID
					writeBinding()
					modes := []int64{1, 2}
					if profile != "five" {
						modes = append(modes, 3, 4, 5)
					}
					for _, mode := range modes {
						reads.Store(0)
						revoke.Store(mode)
						run(token, false)
						readQAEvidenceHTTP(t, readMux, token, conv.ID, 1, false)
						publication.allowed.Store(true)
						e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
						if profile != "five" {
							e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
							e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1`)
							e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=1`)
						}
					}
					revoke.Store(0)
					run(web.token(t, 2, "client"), false)
					run(web.token(t, 1, "admin"), false)
					readQAEvidenceHTTP(t, readMux, web.token(t, 2, "client"), conv.ID, 1, false)
					readQAEvidenceHTTP(t, readMux, web.token(t, 1, "admin"), conv.ID, 1, false)
					readQAEvidenceHTTP(t, readMux, token, conv.ID, 4, false)
					wantCore := map[string]int{"five": 3, "six": 4, "seven": 5}[profile]
					if upstream.Count() != 5 || models.calls != 3 || e.server.Count() != wantCore || e.submits.Load() != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 3 || before[0] != e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) || before[1] != e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`) || before[2] != e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) || before[3] != e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls`) {
						t.Fatal("采集/撤权/错绑/越权增加模型、任务、消息、报告或账务")
					}
					t.Log("原JWT/SQL报告追问双遍回执与计分通过；当前撤权、任务错绑和账号隔离拒绝且零执行/账务副作用，非真实模型质量")
				})
			}
		})
	}
}
