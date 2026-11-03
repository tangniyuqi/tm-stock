//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type researchCaptureCase struct {
	ID, Subject, Question string
	Sources               []struct {
		ID, Text   string
		Authorized bool
	}
}

var captureDiagnosticPattern = regexp.MustCompile(`^问答回执采集失败（[a-z_]+），未输出评测记录。\n$`)

// 仅按模型协议脚本跑真实问答链；不读取expected/basis，错误答案交给独立计分器识别。
func TestIntegrationAIConversationsResearchCapture(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	corpus := filepath.Join(root, "eval", "research", "cases.jsonl")
	raw, err := os.ReadFile(corpus)
	if err != nil {
		t.Fatal(err)
	}
	var cases []researchCaptureCase
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var c researchCaptureCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, c)
	}
	if len(cases) != 17 {
		t.Fatal("固定问题集被改动，须重核运行记录")
	}
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newQASQL(t, level, "capture-only")
			src := repository.NewAISourceRepo(x.db)
			now := time.Now().UTC().Truncate(time.Millisecond)
			scopes := make([]qa.Scope, len(cases))
			bindings := make([]map[string]any, 0, len(cases))
			var steps []providertest.Step
			for i, c := range cases {
				scope := qa.Scope{SubjectType: "theme", SubjectID: int64(100 + i), SubjectName: c.Subject, ApprovalRef: fmt.Sprintf("only-test:research-%d", i), ValidUntil: now.Add(time.Hour)}
				x.mustExec(`INSERT INTO addon_quant_theme(id,name,status) VALUES(?,?,1)`, scope.SubjectID, scope.SubjectName)
				locators := make([]map[string]any, 0)
				for _, s := range c.Sources {
					if !s.Authorized {
						continue
					}
					snap, err := sources.BuildSnapshot(x.source, "https://qa.example.test/research/"+c.ID+"/"+s.ID, sources.MediaPlain, []byte(s.Text), now, 65536)
					if err != nil || snap.Body != s.Text {
						t.Fatal("工程原文必须与真实快照规范化逐字一致", c.ID, err)
					}
					snap, err = src.SaveSnapshot(x.ctx, snap)
					if err != nil {
						t.Fatal(err)
					}
					scope.Snapshots = append(scope.Snapshots, snap)
					locators = append(locators, map[string]any{"ref": s.ID, "snapshotId": snap.ID, "startByte": 0, "endByte": len([]byte(s.Text)), "sourceType": snap.SourceCategory, "sourceUrl": snap.URL, "collectedAt": snap.FetchedAt.UnixMilli()})
				}
				scopes[i] = scope
				bindings = append(bindings, map[string]any{"id": c.ID, "conversationId": x.conv.ID, "seq": i + 1, "subjectId": scope.SubjectID, "sources": locators})
				if len(scope.Snapshots) == 0 {
					continue
				}
				u := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
				if c.ID == "planned-versus-actual" {
					steps = append(steps, providertest.Drop())
					continue
				}
				if c.ID == "injected-source" {
					steps = append(steps, providertest.Success(`{"action":"refuse","reason":"request_not_allowed","citations":[]}`, u))
					continue
				}
				pick := qa.Citation{SnapshotID: scope.Snapshots[0].ID}
				for _, sentence := range scope.Snapshots[0].Sentences {
					pick.SentenceIDs = append(pick.SentenceIDs, sentence.Number)
				}
				selected, err := json.Marshal(map[string]any{"action": "answer", "reason": "none", "citations": []qa.Citation{pick}})
				if err != nil {
					t.Fatal(err)
				}
				steps = append(steps, providertest.Success(string(selected), u), providertest.Success(`{"verdict":"pass","reason":"verified"}`, u), providertest.Success(`{"verdict":"pass","reason":"verified"}`, u))
			}
			upstream := providertest.NewServer(t, steps...)
			x.provider = upstream
			x.direct, err = provider.NewDirect(provider.Config{Enabled: true, Name: "工程采集协议", BaseURL: upstream.URL(), Model: "qa-sql-test", APIKey: "test-only", TimeoutMS: 20000}, true)
			if err != nil {
				t.Fatal(err)
			}
			x.cfg.Selector.ProfileDigest = x.direct.DirectProfileDigest()
			x.cfg.Entailment = x.cfg.Selector
			x.cfg.Semantic = x.cfg.Selector
			for i, scope := range scopes {
				in := conversation.MessageInput{Kind: "question", Skill: "material_qa", SubjectType: "theme", SubjectID: scope.SubjectID, Question: cases[i].Question}
				p, err := qa.NewPlan(qa.Identity{UserID: 1, ConversationID: x.conv.ID, MessageSeq: int64(i + 1)}, in, scope, x.cfg, now)
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					x.plan = p
				}
				evidence, err := json.Marshal(p.Proof().Evidence)
				if err != nil {
					t.Fatal(err)
				}
				x.mustExec(`INSERT INTO tm_ai_qa_scopes(subject_type,subject_id,subject_name,approval_ref,policy_digest,evidence_json,valid_until,enabled,version,created_at,updated_at) VALUES('theme',?,?,?,?,?,?,0,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, scope.SubjectID, scope.SubjectName, scope.ApprovalRef, p.Proof().PolicyDigest, evidence, scope.ValidUntil.Format("2006-01-02 15:04:05.000"))
			}
			x.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=1,version=version+1,updated_at=UTC_TIMESTAMP(3) WHERE subject_id>=100`)
			x.mustExec(`UPDATE tm_ai_qa_limits SET user_requests=50,instance_requests=100,version=version+1,updated_at=UTC_TIMESTAMP(3)`)
			models := &qaHTTPModels{x: x}
			svc := qaHTTPService(t, x, true, models)
			qaHTTPRoutes(t, x, svc, true)
			token := x.token(t, 1, "client", x.key)
			for i, c := range cases {
				body, err := json.Marshal(dto.AIQAInput{Version: int64(i + 1), SubjectType: "theme", SubjectID: scopes[i].SubjectID, Skill: "material_qa", Question: c.Question})
				if err != nil {
					t.Fatal(err)
				}
				want := 200
				if c.ID == "planned-versus-actual" {
					want = 503
				}
				conversationRequest(t, x.cancelHTTPEnv, "POST", fmt.Sprintf("/conversations/%d/questions", x.conv.ID), fmt.Sprintf("test_research_case_%03d", i), token, string(body), want)
			}
			if upstream.Count() != 44 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 17 {
				t.Fatal("模型脚本、实际失败或持久问题数量不符", upstream.Count())
			}
			beforeModel, beforeFactory, beforePoints := upstream.Count(), models.count, x.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
			// 采集阶段连模型工厂都不装配，仅保留原当前授权读取器。
			readOnly := qaHTTPService(t, x, false, nil)
			qaHTTPRoutes(t, x, readOnly, false)
			for i, scope := range scopes {
				v := readQAEvidenceHTTP(t, x.server.Config.Handler, token, x.conv.ID, int64(i+1), true)
				assertQAEvidenceScope(t, v, scope, 0)
			}
			var reads atomic.Int64
			var revoke atomic.Bool
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := reads.Add(1)
				if revoke.Load() && n == 34 {
					if _, err := x.db.ExecContext(r.Context(), `UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, x.source.ID); err != nil {
						t.Error("采集中撤回失败")
						http.Error(w, "test-only", 500)
						return
					}
				}
				if r.Method != "GET" {
					http.Error(w, "readonly", 405)
					return
				}
				x.server.Config.Handler.ServeHTTP(w, r)
			}))
			t.Cleanup(gateway.Close)
			dir := t.TempDir()
			manifest := filepath.Join(dir, "binding.json")
			// 资料摘要从同一离线工具取得，不用测试副本重实现摘要算法。
			materialCmd := exec.CommandContext(x.ctx, "node", filepath.Join(root, "scripts", "dev", "research-eval.cjs"), "materials", corpus, "development")
			material, err := materialCmd.Output()
			if err != nil {
				t.Fatal("物料命令失败")
			}
			var materialMeta struct{ DatasetDigest string }
			if err := json.Unmarshal(material, &materialMeta); err != nil {
				t.Fatal(err)
			}
			config, err := json.Marshal(map[string]any{"schema": "tm-stock-qa-capture-v1", "datasetDigest": materialMeta.DatasetDigest, "split": "development", "baseURL": gateway.URL, "modelVersion": "loopback-script-only-v1", "promptVersion": x.plan.Proof().PolicyDigest, "bindings": bindings})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, config, 0600); err != nil {
				t.Fatal(err)
			}
			run := func(bearer string, want bool) []byte {
				t.Helper()
				cmd := exec.CommandContext(x.ctx, "node", filepath.Join(root, "scripts", "dev", "research-qa-capture.cjs"), corpus, manifest)
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
					t.Fatal("采集命令退出状态不符，凭据与原文不输出", want)
				}
				if !want && len(out) > 0 {
					t.Fatal("拒绝后仍输出部分成绩")
				}
				return out
			}
			out := run(token, true)
			var exported struct {
				Kind, Mode string
				Results    []struct {
					State      string
					CostMicros *int64
				}
			}
			if err := json.Unmarshal(out, &exported); err != nil || exported.Kind != "api_capture" || exported.Mode != "stored_qa" || len(exported.Results) != 17 {
				t.Fatal("未取得规范化实际回执", err)
			}
			for _, s := range []string{token, "conversationId", "snapshotId", "sourceUrl", "仅测试"} {
				if strings.Contains(string(out), s) {
					t.Fatal("导出泄漏私有定位或正文")
				}
			}
			if reads.Load() != 66 {
				t.Fatal("没有完成全部问题双遍核对", reads.Load())
			}
			record := filepath.Join(dir, "run.json")
			if err := os.WriteFile(record, out, 0600); err != nil {
				t.Fatal(err)
			}
			graded, err := exec.CommandContext(x.ctx, "node", filepath.Join(root, "scripts", "dev", "research-eval.cjs"), "score", corpus, record).Output()
			if err != nil {
				t.Fatal("实际回执不能离线评分")
			}
			var report struct {
				Counts struct{ Samples, Answered, Refused, Failed, CorrectAnswers, CorrectRefusals, IncorrectAnswers int }
			}
			if err := json.Unmarshal(graded, &report); err != nil || report.Counts.Samples != 17 || report.Counts.Answered != 14 || report.Counts.Refused != 2 || report.Counts.Failed != 1 || report.Counts.CorrectAnswers != 6 || report.Counts.CorrectRefusals != 2 || report.Counts.IncorrectAnswers != 8 {
				t.Fatal("模型替身的错误或实际失败没有计入", err, report.Counts)
			}
			reads.Store(0)
			revoke.Store(true)
			run(token, false)
			revoke.Store(false)
			x.mustExec(`UPDATE tm_ai_sources SET enabled=1 WHERE id=?`, x.source.ID)
			run(x.token(t, 2, "client", x.key), false)
			if upstream.Count() != beforeModel || models.count != beforeFactory || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 17 || x.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != beforePoints {
				t.Fatal("只读采集增加模型、问题或积分流水")
			}
			t.Log("17题原JWT/会员/服务/存证→双遍采集→离线评分通过；有意保留脚本错误与失败，不是真实模型成绩")
		})
	}
}
