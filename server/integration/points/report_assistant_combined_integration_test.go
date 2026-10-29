//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 组合验收沿用报告夹具的同一资料、会员、会话及账本，仅模型响应是回环替身。
func prepareH5CombinedScope(t *testing.T, e *submissionHTTPEnv, cfg qa.Config) h5ReportQAPolicies {
	t.Helper()
	cfg.AllowedDomains = []string{e.source.Domain}
	cfg.Limits.MaxSources = 4
	now := time.Now().UTC().Truncate(time.Millisecond)
	scope := qa.Scope{SubjectType: "theme", SubjectID: 1, SubjectName: "示例题材", ApprovalRef: "test-only-scope", ValidUntil: now.Add(time.Hour), Snapshots: []sources.Snapshot{e.snap}}
	plan, err := qa.NewPlan(qa.Identity{UserID: 1, ConversationID: 1, MessageSeq: 1}, conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "示例题材是什么？"}, scope, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := json.Marshal(plan.Proof().Evidence)
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(`INSERT INTO tm_ai_qa_scopes(subject_type,subject_id,subject_name,approval_ref,policy_digest,evidence_json,valid_until,enabled,version,created_at,updated_at) VALUES('theme',1,?,?,?,?,?,0,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, scope.SubjectName, scope.ApprovalRef, plan.Proof().PolicyDigest, evidence, scope.ValidUntil.Format("2006-01-02 15:04:05.000"))
	e.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
	return h5ReportQAPolicies{config: cfg, digest: plan.Proof().PolicyDigest}
}

func TestIntegrationAIReportAssistantCombined(t *testing.T) {
	// 本根组从不装配真实模型；外部凭据不能改变它的执行范围。
	t.Setenv("TM_TEST_AI_H5_COMBINED", "local-fixture-only")
	t.Setenv("TM_TEST_AI_H5_REPORT_QA", "1")
	t.Setenv("TM_TEST_AI_H5_INTENTS", "local-fixture-only")
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "success")
			front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "仅测试代理，非实际浏览器") }))
			t.Cleanup(front.Close)
			origin := os.Getenv("TM_TEST_AI_H5_ORIGIN")
			interactive := origin != ""
			if !interactive {
				origin = front.URL
			} else {
				ctx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
				t.Cleanup(stop)
				e.pointsTestEnv.ctx = ctx
			}
			g := newH5ReportGateway(t, e, origin)
			if interactive {
				t.Log("同一会话资料问答→轻度任务→报告→反馈；仅测试模型：", g.server.URL+"/__test__/")
				for {
					select {
					case job := <-g.actions:
						if g.process(t, job, "feedback") {
							return
						}
					case <-e.ctx.Done():
						t.Fatal("组合浏览器未完成，不记通过")
					}
				}
			}
			token, other := e.web.token(t, 1, "client"), e.web.token(t, 2, "client")
			call := func(method, path, key, body string, status int) []byte {
				return h5GatewayRequest(t, g, method, path, token, key, body, status)
			}
			conv := conversationData[dto.AIConversationReceipt](t, call("POST", "/api/v1/ai/conversations", "combined_conversation_01", `{"title":"组合验收"}`, 200))
			path := fmt.Sprintf("/api/v1/ai/conversations/%d", conv.ID)
			before := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			question := `{"version":1,"subjectType":"theme","subjectId":1,"skill":"material_qa","question":"示例题材是什么？"}`
			call("POST", path+"/questions", "combined_question_01", question, 200)
			answer := conversationData[dto.AIQAAnswer](t, call("GET", path+"/questions/1/answer", "", "", 200))
			if len(answer.Quotes) != 1 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
				t.Fatal("资料问答缺引文或意外改账")
			}
			call("POST", path+"/questions", "combined_question_01", question, 200)
			h5GatewayRequest(t, g, "GET", path+"/questions/1/answer", other, "", "", 404)
			intent := conversationData[dto.AIConversationTask](t, call("POST", path+"/task-intents", "combined_intent_01", sqlSubmissionBody, 200))
			if e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != before {
				t.Fatal("保存报价意图意外改账")
			}
			call("POST", fmt.Sprintf("%s/task-intents/%d/confirm", path, intent.ID), "", `{"confirmed":true}`, 200)
			select {
			case job := <-g.actions:
				g.process(t, job, "feedback")
			case <-e.ctx.Done():
				t.Fatal("原任务未进入worker")
			}
			taskPath := fmt.Sprintf("/api/v1/ai/tasks/%d", e.v.ID)
			assertPublicReport(t, call("GET", taskPath+"/report", "", "", 200), e.reportEnv)
			call("GET", taskPath+"/report", "", "", 200)
			call("GET", taskPath+"/feedback", "", "", 200)
			call("POST", taskPath+"/feedback", "", `{"category":"fact"}`, 200)
			g.process(t, h5ReportAction{kind: "resolve-feedback"}, "feedback")
			call("GET", taskPath+"/feedback", "", "", 200)
			g.process(t, h5ReportAction{kind: "withdraw"}, "feedback")
			call("GET", taskPath+"/report", "", "", 422)
			call("GET", path+"/questions/1/answer", "", "", 503)
			h5GatewayRequest(t, g, "GET", taskPath+"/feedback", other, "", "", 404)
			g.process(t, h5ReportAction{kind: "finish"}, "feedback")
		})
	}
}
