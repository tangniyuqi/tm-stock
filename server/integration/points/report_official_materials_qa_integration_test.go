//go:build integration

package points_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestIntegrationAIReportOfficialMaterialQA(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	verifyOfficialMaterialQA(t, "")
}

// 同一原API、数据库和问题分别验回环协议及真实供应商；不把两种证据混为质量评测。
func verifyOfficialMaterialQA(t *testing.T, liveEndpoint string) {
	t.Helper()
	bundle := loadOfficialSamples(t)
	var file config.AILocalQA
	var qaPath string
	var snapshots map[string]sources.Snapshot
	var upstream *providertest.Server
	p := newDossierProcessWithQA(t, 4*time.Minute, false, func(f *localPublicationFixture, dsn string) string {
		e := f.env
		prepareLocalQASchema(t, f, dsn)
		snapshots, _, _ = importOfficialSampleSnapshots(t, e, bundle)
		endpoint := liveEndpoint
		if endpoint == "" {
			steps := []providertest.Step{}
			for _, key := range []string{"uav-regulation-761", "storage-manufacturing-plan", "distributed-pv-notice"} {
				steps = append(steps, qaSuccessSteps(snapshots[key].ID)...)
			}
			steps = append(steps, providertest.Success(`{"action":"refuse","reason":"no_evidence","citations":[]}`, provider.Usage{Prompt: 20, Completion: 30, Total: 50}))
			steps = append(steps, providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
				// 首次真实HTTP回包前撤回文件，下一阶段必须停，不重发原调用。
				changed := file
				changed.Revision = "revoked-during-call"
				raw, err := json.Marshal(changed)
				if err != nil || os.WriteFile(qaPath, raw, 0600) != nil {
					t.Error("运行中撤回夹具写入失败")
					w.WriteHeader(500)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-Id", "official-qa-revoked-call")
				content := fmt.Sprintf(`{"action":"answer","reason":"none","citations":[{"snapshot_id":%d,"sentence_ids":[1]}]}`, snapshots["uav-regulation-761"].ID)
				response := map[string]any{"id": "official-qa-revoked-call", "model": "deepseek-flash", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": content}}}, "usage": map[string]int{"prompt_tokens": 20, "completion_tokens": 30, "total_tokens": 50}}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error("运行中撤回夹具回包失败")
				}
			}))
			upstream = providertest.NewServer(t, steps...)
			endpoint = upstream.URL()
		}
		m := config.AILocalQAModel{Endpoint: config.AILocalExecutionModel{Name: "正式资料开发问答", BaseURL: endpoint, Model: "deepseek-flash", TimeoutMS: 20000}, FilingNo: "local-eval:development-not-production-filing", ApprovalRef: "local-eval:official-qa-model", QualityRef: "local-eval:trial-not-quality-approved", BudgetRef: "local-eval:official-qa-budget", JSONSchemaFallback: true, ReasoningEffort: "none"}
		client, err := provider.NewDirect(m.ProviderConfig(), true)
		if err != nil {
			t.Fatal(err)
		}
		m.ProfileDigest = client.DirectProfileDigest()
		manifest, err := os.ReadFile(filepath.Join("..", "..", "..", "eval", "research", "official-materials", "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		file = config.AILocalQA{Version: 2, InstanceID: f.runtime.InstanceID, Revision: "official-v2", Scope: "official-materials-development-only", IssuedAtMS: time.Now().UnixMilli() - 1000, ExpiresAtMS: f.file.ExpiresAtMS, Members: []int64{1}, Subjects: []int64{1001, 1002, 1003}, Models: []config.AILocalQAModel{m, m, m}, AllowedDomains: []string{"www.caac.gov.cn", "www.miit.gov.cn", "www.nea.gov.cn"}, Limits: config.AILocalQALimits{MaxSources: 4, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, TimeoutMS: 20000}, MaterialUse: &config.AILocalQAMaterialUse{DatasetDigest: task.StageDigest(manifest), ApprovalRef: "local-eval:official-documents-development-use", Purpose: "bounded-material-qa", Bindings: []config.AILocalQAMaterialBinding{}}}
		cfg, err := file.QAConfig()
		if err != nil {
			t.Fatal(err)
		}
		policy, err := cfg.PolicyDigest()
		if err != nil {
			t.Fatal(err)
		}
		for i, theme := range bundle.Themes {
			id := int64(1001 + i)
			e.mustExec(`INSERT INTO addon_quant_theme(id,name,status,description) VALUES(?,?,1,?)`, id, theme.Name, "正式机关文件开发节选；未独立人工审核。"+theme.Scope)
			used := map[string]bool{}
			for _, card := range theme.Cards {
				used[card.Document] = true
				for _, field := range card.Fields {
					used[field.Document] = true
				}
			}
			refs := []qa.EvidenceProof{}
			for key := range used {
				s, ok := snapshots[key]
				if !ok {
					t.Fatal("专题快照缺失")
				}
				refs = append(refs, qa.EvidenceProof{ID: s.ID, Hash: s.Hash})
			}
			slices.SortFunc(refs, func(a, b qa.EvidenceProof) int {
				if a.ID < b.ID {
					return -1
				}
				if a.ID > b.ID {
					return 1
				}
				return 0
			})
			approval := fmt.Sprintf("local-eval:official-scope-%d", id)
			file.MaterialUse.Bindings = append(file.MaterialUse.Bindings, config.AILocalQAMaterialBinding{SubjectID: id, ScopeApproval: approval, Evidence: refs})
			raw, err := json.Marshal(refs)
			if err != nil {
				t.Fatal(err)
			}
			e.mustExec(`INSERT INTO tm_ai_qa_scopes(subject_type,subject_id,subject_name,approval_ref,policy_digest,evidence_json,valid_until,enabled,version,created_at,updated_at) VALUES('theme',?,?,?,?,?,?,0,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, theme.Name, approval, policy, raw, time.UnixMilli(file.ExpiresAtMS).UTC().Format("2006-01-02 15:04:05.000"))
		}
		e.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
		e.mustExec(`INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,?,0,10,20,1000000,2000000,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.BudgetRef)
		e.mustExec(`UPDATE tm_ai_qa_limits SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
		qaPath = filepath.Join(t.TempDir(), "official-qa.json")
		writeOfficialQAFile(t, qaPath, file)
		return qaPath
	})
	e := p.f.env
	tokens, _ := localPasswordTokens(t, e.ctx, p.login, p.client)
	call := func(method, path, key, token, body string, want int) json.RawMessage {
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
			t.Fatal("正式资料原API请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 262145))
		if err != nil || len(raw) > 262144 {
			t.Fatal("正式资料响应过大")
		}
		var out struct {
			Code int
			Data json.RawMessage
		}
		if json.Unmarshal(raw, &out) != nil || res.StatusCode != want || (out.Code == 0) != (want == 200) || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("正式资料HTTP契约不符 %s %s got=%d want=%d", method, path, res.StatusCode, want)
		}
		if want != 200 && len(out.Data) > 0 && string(out.Data) != "null" {
			t.Fatal("拒绝后泄漏资料")
		}
		return out.Data
	}
	var conv struct{ ID int64 }
	if json.Unmarshal(call("POST", "/api/v1/ai/conversations", "official_qa_conversation_01", tokens[0], `{"title":"正式机关文件开发问答试跑"}`, 200), &conv) != nil || conv.ID <= 0 {
		t.Fatal("原会话未创建")
	}
	root := fmt.Sprintf("/api/v1/ai/conversations/%d", conv.ID)
	questions := []string{"所选资料如何定义无人驾驶航空器？", "所选资料中，新型储能制造业的定义是什么？", "所选通知是否修订并印发了《分布式光伏发电开发建设管理办法》？", "本次所选资料记载的分布式光伏项目装机总量是多少？"}
	evidenceRows := []dto.AIQAEvidence{}
	for i, question := range questions {
		id := int64(1001 + i)
		if i == 3 {
			id = 1003
		}
		body, err := json.Marshal(dto.AIQAInput{Version: int64(i + 1), SubjectType: "theme", SubjectID: id, Skill: "material_qa", Question: question})
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("official_qa_question_%02d", i)
		var status dto.AIQAStatus
		if json.Unmarshal(call("POST", root+"/questions", key, tokens[0], string(body), 200), &status) != nil || status.Seq != int64(i+1) || (i < 3 && status.State != "answered") || (i == 3 && status.State != "refused") {
			t.Fatalf("正式资料问题未取得预期终态：%d state=%s", i, status.State)
		}
		call("POST", root+"/questions", key, tokens[0], string(body), 200)
		path := fmt.Sprintf("%s/questions/%d", root, status.Seq)
		var answer dto.AIQAAnswer
		if json.Unmarshal(call("GET", path+"/answer", "", tokens[0], "", 200), &answer) != nil || answer.State != status.State || i < 3 && len(answer.Quotes) == 0 || i == 3 && len(answer.Quotes) != 0 {
			t.Fatal("实际答案缺引文或拒答仍带引文")
		}
		for _, q := range answer.Quotes {
			found := false
			for _, s := range snapshots {
				if s.ID == q.SnapshotID && s.URL == q.SourceURL && q.StartByte >= 0 && q.EndByte <= len(s.Body) && q.StartByte < q.EndByte && s.Body[q.StartByte:q.EndByte] == q.Excerpt {
					found = true
				}
			}
			if !found {
				t.Fatal("答案引用不是实际限定快照原文")
			}
		}
		var evidence dto.AIQAEvidence
		if json.Unmarshal(call("GET", path+"/evidence", "", tokens[0], "", 200), &evidence) != nil || evidence.PromptVersion != qa.Version || evidence.SubjectID != id || evidence.TaskID != 0 {
			t.Fatal("原候选/调用核验缺失")
		}
		binding := file.MaterialUse.Bindings[int(id-1001)]
		if len(evidence.Sources) != len(binding.Evidence) || len(evidence.Calls) != 3 && i < 3 || len(evidence.Calls) != 1 && i == 3 {
			t.Fatal("候选范围或调用数不符")
		}
		for _, src := range evidence.Sources {
			found := false
			for _, s := range snapshots {
				if s.ID == src.SnapshotID && s.Hash == src.Digest && s.URL == src.SourceURL && len(s.Sentences) == len(src.Sentences) {
					found = true
					for n, line := range src.Sentences {
						if line.Text != s.Sentences[n].Text || line.StartByte != s.Sentences[n].StartByte || line.EndByte != s.Sentences[n].EndByte {
							t.Fatal("完整候选原句不符")
						}
					}
				}
			}
			if !found {
				t.Fatal("完整候选绑定不符")
			}
		}
		for _, c := range evidence.Calls {
			if c.Status != "confirmed" || c.ModelName != "deepseek-flash" || c.InputTokens == nil || c.OutputTokens == nil || c.CostMicros != nil {
				t.Fatal("实际调用/用量/未知账单不符")
			}
		}
		evidenceRows = append(evidenceRows, evidence)
		call("GET", path+"/answer", "", tokens[1], "", 404)
		call("GET", path+"/evidence", "", tokens[1], "", 404)
		call("GET", path+"/evidence", "", "", "", 401)
	}
	path := root + "/questions/1"
	e.mustExec(`UPDATE tm_ai_sources SET enabled=0 WHERE domain='www.caac.gov.cn'`)
	call("GET", path+"/answer", "", tokens[0], "", 503)
	call("GET", path+"/evidence", "", tokens[0], "", 503)
	e.mustExec(`UPDATE tm_ai_sources SET enabled=1 WHERE domain='www.caac.gov.cn'`)
	e.mustExec(`UPDATE tm_ai_qa_scopes SET approval_ref='local-eval:revoked-scope',version=version+1,updated_at=UTC_TIMESTAMP(3) WHERE subject_id=1001`)
	call("GET", path+"/evidence", "", tokens[0], "", 503)
	e.mustExec(`UPDATE tm_ai_qa_scopes SET approval_ref=?,version=version+1,updated_at=UTC_TIMESTAMP(3) WHERE subject_id=1001`, file.MaterialUse.Bindings[0].ScopeApproval)
	file.Revision = "revoked-v2"
	writeOfficialQAFile(t, qaPath, file)
	call("GET", path+"/answer", "", tokens[0], "", 503)
	call("GET", path+"/evidence", "", tokens[0], "", 503)
	call("POST", root+"/questions", "official_qa_revoked_01", tokens[0], `{"version":5,"subjectType":"theme","subjectId":1001,"skill":"material_qa","question":"所选资料如何定义无人驾驶航空器？"}`, 503)
	file.Revision = "official-v2"
	writeOfficialQAFile(t, qaPath, file)
	call("GET", path+"/answer", "", tokens[0], "", 200)
	// 数据库给出另一个当前合法快照也不能扩大文件限定的题材范围。
	replacement, err := json.Marshal(file.MaterialUse.Bindings[2].Evidence)
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(file.MaterialUse.Bindings[0].Evidence)
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(`UPDATE tm_ai_qa_scopes SET evidence_json=?,version=version+1,updated_at=UTC_TIMESTAMP(3) WHERE subject_id=1001`, replacement)
	questionBody := `{"version":5,"subjectType":"theme","subjectId":1001,"skill":"material_qa","question":"所选资料如何定义无人驾驶航空器？"}`
	call("POST", root+"/questions", "official_qa_wrong_scope_01", tokens[0], questionBody, 503)
	e.mustExec(`UPDATE tm_ai_qa_scopes SET evidence_json=?,version=version+1,updated_at=UTC_TIMESTAMP(3) WHERE subject_id=1001`, original)
	var other struct{ ID int64 }
	if json.Unmarshal(call("POST", "/api/v1/ai/conversations", "official_qa_other_member_01", tokens[1], `{"title":"开发范围外账号"}`, 200), &other) != nil || other.ID <= 0 {
		t.Fatal("范围外账号原会话创建失败")
	}
	call("POST", fmt.Sprintf("/api/v1/ai/conversations/%d/questions", other.ID), "official_qa_other_question_01", tokens[1], strings.Replace(questionBody, `"version":5`, `"version":1`, 1), 503)
	requests, calls := 4, 10
	if upstream != nil {
		call("POST", root+"/questions", "official_qa_inflight_revoke_01", tokens[0], questionBody, 503)
		var failed dto.AIQAStatus
		if json.Unmarshal(call("GET", root+"/questions/5", "", tokens[0], "", 200), &failed) != nil || failed.State != "failed" {
			t.Fatal("执行中撤回未保留原失败状态")
		}
		writeOfficialQAFile(t, qaPath, file)
		var proof dto.AIQAEvidence
		if json.Unmarshal(call("GET", root+"/questions/5/evidence", "", tokens[0], "", 200), &proof) != nil || proof.State != "failed" || len(proof.Calls) != 1 || proof.Calls[0].Status != "confirmed" {
			t.Fatal("执行中撤回丢失已确认调用或继续发送")
		}
		requests, calls = 5, 11
	}
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != requests || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls`) != calls || upstream != nil && len(upstream.Requests()) != calls {
		t.Fatal("重放/读取/撤回增加了问答或模型副作用")
	}
	p.verifyNoGeneration(t)
	if liveEndpoint != "" {
		output := os.Getenv("TM_TEST_AI_OFFICIAL_QA_OUTPUT")
		if filepath.Dir(output) != os.TempDir() || !strings.HasPrefix(filepath.Base(output), "tm-stock-official-qa-live-") {
			t.Fatal("真实试跑输出必须是本次临时目录")
		}
		b, err := json.MarshalIndent(struct {
			Classification string             `json:"classification"`
			DatasetDigest  string             `json:"dataset_digest"`
			Questions      []string           `json:"questions"`
			Evidence       []dto.AIQAEvidence `json:"evidence"`
		}{"real-provider-official-passages-development-not-human-gold", file.MaterialUse.DatasetDigest, questions, evidenceRows}, "", "  ")
		if err != nil || os.WriteFile(filepath.Join(output, "qa-api-evidence.json"), b, 0600) != nil {
			t.Fatal("真实问答证据写入失败")
		}
		t.Log("TM_STOCK_OFFICIAL_QA_LIVE_VERIFIED")
	}
}

func writeOfficialQAFile(t *testing.T, path string, c config.AILocalQA) {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
