//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type qaHTTPPolicies struct{ x *qaSQLFixture }

func (p qaHTTPPolicies) ForPolicy(_ context.Context, h string) (qa.Config, error) {
	if h != p.x.plan.Proof().PolicyDigest {
		return qa.Config{}, qa.ErrUnavailable
	}
	return p.x.cfg, nil
}

type qaHTTPModels struct {
	x     *qaSQLFixture
	count int
	hook  func(context.Context) error
}

func (p *qaHTTPModels) ForPlan(ctx context.Context, proof qa.Proof) ([3]*provider.OpenAICompat, error) {
	p.count++
	if p.hook != nil {
		if err := p.hook(ctx); err != nil {
			return [3]*provider.OpenAICompat{}, err
		}
	}
	if proof.PolicyDigest != p.x.plan.Proof().PolicyDigest {
		return [3]*provider.OpenAICompat{}, qa.ErrUnavailable
	}
	return [3]*provider.OpenAICompat{p.x.direct, p.x.direct, p.x.direct}, nil
}

type qaUnknownAccess struct {
	qa.AccessStore
	unknown *qaSQLUnknown
}

func (s qaUnknownAccess) WithinQA(ctx context.Context, write bool, fn func(qa.Tx) error) error {
	return s.unknown.WithinQA(ctx, write, fn)
}

type qaWrongPolicy struct{ x *qaSQLFixture }

func (s qaWrongPolicy) ForPolicy(context.Context, string) (qa.Config, error) {
	c := s.x.cfg
	c.Selector.Name = "other-model"
	return c, nil
}
func TestIntegrationAIConversationsQAHTTPBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"protocol", "unknown", "quota", "token_expire", "member_off", "stale_scope", "empty", "policy_mismatch", "queue_unknown"} {
				t.Run(mode, func(t *testing.T) {
					x := newQASQL(t, level, mode)
					models := &qaHTTPModels{x: x}
					svc := qaHTTPService(t, x, true, models)
					qaHTTPRoutes(t, x, svc, true)
					token := x.token(t, 1, "client", x.key)
					path := fmt.Sprintf("/conversations/%d", x.conv.ID)
					input := dto.AIQAInput{Version: 1, SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: x.input.Question}
					body, err := json.Marshal(input)
					if err != nil {
						t.Fatal(err)
					}
					want := 503
					expectedCalls := 1
					expectedQuestions := 1
					state := "failed"
					switch mode {
					case "quota":
						x.mustExec(`UPDATE tm_ai_qa_limits SET user_tokens=1,version=version+1,updated_at=UTC_TIMESTAMP(3)`)
						want = 429
						expectedCalls = 0
					case "token_expire":
						want = 401
					case "member_off":
						want = 401
						expectedCalls = 2
					case "stale_scope":
						models.hook = func(ctx context.Context) error {
							_, err := x.db.ExecContext(ctx, `UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, x.source.ID)
							return err
						}
						expectedCalls = 0
						expectedQuestions = 0
					case "empty":
						x.mustExec(`UPDATE tm_ai_qa_scopes SET evidence_json='[]',version=version+1,updated_at=UTC_TIMESTAMP(3)`)
						want = 200
						expectedCalls = 0
						state = "refused"
					case "policy_mismatch":
						v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: x.key, PreviousKey: x.old, Issuer: "GVA", Audience: "GVA"})
						if err != nil {
							t.Fatal(err)
						}
						session, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(x.db))
						if err != nil {
							t.Fatal(err)
						}
						svc, err = service.NewAIQAService(x.repo, qaWrongPolicy{x}, models, session, true)
						if err != nil {
							t.Fatal(err)
						}
						qaHTTPRoutes(t, x, svc, true)
						expectedCalls = 0
						expectedQuestions = 0
					case "queue_unknown":
						v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: x.key, PreviousKey: x.old, Issuer: "GVA", Audience: "GVA"})
						if err != nil {
							t.Fatal(err)
						}
						session, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(x.db))
						if err != nil {
							t.Fatal(err)
						}
						store := qaUnknownAccess{AccessStore: x.repo, unknown: &qaSQLUnknown{Store: x.repo, proof: x.plan.Proof(), target: "queued"}}
						svc, err = service.NewAIQAService(store, qaHTTPPolicies{x}, models, session, true)
						if err != nil {
							t.Fatal(err)
						}
						qaHTTPRoutes(t, x, svc, true)
						expectedCalls = 0
						state = "queued"
					}
					if mode == "token_expire" {
						// 尽量靠近发请求签发；首个测试上游延迟5.1秒，明确覆盖执行中途失效。
						token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"ID": 1, "UserType": "client", "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(5 * time.Second).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte(x.key))
						if err != nil {
							t.Fatal(err)
						}
					}
					conversationRequest(t, x.cancelHTTPEnv, "POST", path+"/questions", "test_qa_http_question_01", token, string(body), want)
					if x.provider.Count() != expectedCalls || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != expectedQuestions || x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != expectedQuestions {
						t.Fatal("拒绝后执行/保存次数错误", mode, x.provider.Count())
					}
					if expectedQuestions == 1 {
						if mode == "member_off" {
							x.mustExec(`UPDATE addon_member SET status=1 WHERE id=1`)
						}
						fresh := x.token(t, 1, "client", x.key)
						r := conversationData[dto.AIQAStatus](t, conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/question-receipt", "test_qa_http_question_01", fresh, "", 200))
						if r.State != state {
							t.Fatal("原状态不符", mode, r.State)
						}
						if mode == "empty" {
							answer := conversationData[dto.AIQAAnswer](t, conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/questions/1/answer", "", fresh, "", 200))
							if answer.AIGenerated || len(answer.Quotes) != 0 {
								t.Fatal("零模型拒答冒充生成")
							}
						} else {
							conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/questions/1/answer", "", fresh, "", 503)
						}
						count := models.count
						conversationRequest(t, x.cancelHTTPEnv, "POST", path+"/questions", "test_qa_http_question_01", fresh, string(body), 200)
						if models.count != count || x.provider.Count() != expectedCalls {
							t.Fatal("故障重放再次执行")
						}
					}
					x.uncharged(t)
				})
			}
		})
	}
}
func qaHTTPService(t *testing.T, x *qaSQLFixture, writes bool, models service.AIQAProviders) *service.AIQAService {
	t.Helper()
	v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: x.key, PreviousKey: x.old, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(x.db))
	if err != nil {
		t.Fatal(err)
	}
	s, err := service.NewAIQAService(x.repo, qaHTTPPolicies{x}, models, sessions, writes)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func qaHTTPRoutes(t *testing.T, x *qaSQLFixture, s *service.AIQAService, writes bool) {
	t.Helper()
	mux := http.NewServeMux()
	handler.NewAIQAHandler(s).Register(mux, x.auth, writes)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	x.server = server
}
func TestIntegrationAIConversationsQAHTTP(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newQASQL(t, level, "success")
			beforeMessages := x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`)
			if err := repository.NewAIQASchemaVerifier(x.db).Verify(x.ctx); err != nil {
				var statement string
				if inspect := x.db.QueryRowContext(x.ctx, `SELECT action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name='tm_ai_qa_scope_no_delete'`).Scan(&statement); inspect == nil {
					t.Logf("固定DDL定义诊断：%q", statement)
				}
				t.Fatal("问答保护正向检查", err)
			}
			if x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != beforeMessages || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 0 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_quotas`) != 0 {
				t.Fatal("探针未全部回滚")
			}
			models := &qaHTTPModels{x: x}
			svc := qaHTTPService(t, x, true, models)
			qaHTTPRoutes(t, x, svc, true)
			path := fmt.Sprintf("/conversations/%d", x.conv.ID)
			token := x.token(t, 1, "client", x.key)
			other := x.token(t, 2, "client", x.key)
			input := dto.AIQAInput{Version: 1, SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: x.input.Question}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			status := conversationData[dto.AIQAStatus](t, conversationRequest(t, x.cancelHTTPEnv, "POST", path+"/questions", "test_qa_http_question_01", token, string(body), 200))
			if status.Seq != 1 || status.State != "answered" || status.Question != input.Question || models.count != 1 || x.provider.Count() != 3 {
				t.Fatal("HTTP问题未走真实持久闭环", status)
			}
			for _, url := range []string{path + "/question-receipt", path + "/questions/1"} {
				raw := conversationRequest(t, x.cancelHTTPEnv, "GET", url, "test_qa_http_question_01", token, "", 200)
				if conversationData[dto.AIQAStatus](t, raw).State != "answered" {
					t.Fatal("状态读取不符")
				}
				for _, hidden := range []string{"snapshotId", "SourceApproval", "PolicyDigest", "tokens", "qa-sql-test", "excerpt"} {
					if strings.Contains(string(raw), hidden) {
						t.Fatal("状态混入答案或内部字段")
					}
				}
			}
			// 原键重放只返回状态，不再加载模型客户端或再执行。
			conversationRequest(t, x.cancelHTTPEnv, "POST", path+"/questions", "test_qa_http_question_01", token, string(body), 200)
			if models.count != 1 || x.provider.Count() != 3 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 1 {
				t.Fatal("POST重放重复执行")
			}
			changed := input
			changed.Question = "另一个测试问题"
			raw, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			conversationRequest(t, x.cancelHTTPEnv, "POST", path+"/questions", "test_qa_http_question_01", token, string(raw), 409)
			conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/questions/1/answer", "", other, "", 404)
			// 创建关闭并移除模型工厂；仅无密钥策略及存储仍能读原答案。
			reader := qaHTTPService(t, x, false, nil)
			qaHTTPRoutes(t, x, reader, false)
			answer := conversationData[dto.AIQAAnswer](t, conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/questions/1/answer", "", token, "", 200))
			if !answer.AIGenerated || answer.ModelName != "qa-sql-test" || len(answer.Quotes) != 1 || answer.Quotes[0].Excerpt != x.scope.Snapshots[0].Sentences[0].Text || x.provider.Count() != 3 {
				t.Fatal("只读答案存证复建失败", answer)
			}
			// 此独立宿主没有GET collection路由，因此关闭POST后标准mux返回404。
			request, err := http.NewRequestWithContext(x.ctx, "POST", x.server.URL+"/api/v1/ai"+path+"/questions", strings.NewReader(string(body)))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer "+token)
			response, err := x.server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 404 {
				t.Fatal("关闭问题提交仍注册了路由")
			}
			x.mustExec(`UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, x.source.ID)
			conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/questions/1/answer", "", token, "", 503)
			conversationRequest(t, x.cancelHTTPEnv, "GET", path+"/question-receipt", "test_qa_http_question_01", token, "", 200)
			if models.count != 1 || x.provider.Count() != 3 {
				t.Fatal("读取或撤回后补模型")
			}
			x.uncharged(t)
		})
	}
}

func TestIntegrationAIConversationsQASchema(t *testing.T) {
	x := newQASQL(t, sql.LevelReadCommitted, "success")
	for _, trigger := range []string{"tm_ai_qa_scope_insert", "tm_ai_qa_scope_update", "tm_ai_qa_scope_no_delete", "tm_ai_qa_limits_insert", "tm_ai_qa_limits_update", "tm_ai_qa_limits_no_delete", "tm_ai_qa_quota_insert", "tm_ai_qa_quota_update", "tm_ai_qa_quota_no_delete", "tm_ai_qa_request_insert", "tm_ai_qa_request_update", "tm_ai_qa_request_no_delete", "tm_ai_qa_call_insert", "tm_ai_qa_call_update", "tm_ai_qa_call_no_delete"} {
		t.Run(trigger, func(t *testing.T) {
			if err := repository.NewAIQASchemaVerifier(x.db).Verify(x.ctx); err != nil {
				t.Fatal("健康探针失败", err)
			}
			x.mustExec("DROP TRIGGER `" + trigger + "`")
			if err := repository.NewAIQASchemaVerifier(x.db).Verify(x.ctx); err == nil {
				t.Fatal("缺触发器未拒绝")
			}
			reportDDL(t, x.pointsTestEnv, "20261021_create_tm_ai_qa.sql")
		})
	}
	for _, c := range []struct{ table, name string }{{"tm_ai_qa_scopes", "chk_tm_ai_qa_scope"}, {"tm_ai_qa_limits", "chk_tm_ai_qa_limits"}, {"tm_ai_qa_quotas", "chk_tm_ai_qa_quota"}, {"tm_ai_qa_requests", "chk_tm_ai_qa_request"}, {"tm_ai_qa_calls", "chk_tm_ai_qa_call"}} {
		t.Run(c.name, func(t *testing.T) {
			y := newQASQL(t, sql.LevelReadCommitted, "success")
			y.mustExec("ALTER TABLE `" + c.table + "` DROP CHECK `" + c.name + "`")
			y.mustExec("ALTER TABLE `" + c.table + "` ADD CONSTRAINT `" + c.name + "` CHECK(1=1)")
			if err := repository.NewAIQASchemaVerifier(y.db).Verify(y.ctx); err == nil {
				t.Fatal("同名宽松CHECK未拒绝")
			}
		})
	}
}
