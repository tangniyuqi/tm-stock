//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type qaSQLFixture struct {
	*cancelHTTPEnv
	repo     *repository.AIQARepo
	plan     *qa.Plan
	control  *qa.DurableControl
	engine   *qa.Engine
	provider *providertest.Server
	cfg      qa.Config
	scope    qa.Scope
	input    conversation.MessageInput
	conv     conversation.Record
	direct   *provider.OpenAICompat
	source   sources.Source
}

func qaSuccessSteps(id int64) []providertest.Step {
	u := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
	return []providertest.Step{providertest.Success(fmt.Sprintf(`{"action":"answer","reason":"none","citations":[{"snapshot_id":%d,"sentence_ids":[1]}]}`, id), u), providertest.Success(`{"verdict":"pass","reason":"verified"}`, u), providertest.Success(`{"verdict":"pass","reason":"verified"}`, u)}
}
func newQASQL(t *testing.T, level sql.IsolationLevel, mode string) *qaSQLFixture {
	t.Helper()
	e, convRepo, svc := conversationEnvironment(t, level)
	reportDDL(t, e.pointsTestEnv, "20261010_create_ai_source_snapshots.sql")
	reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_qa.sql")
	e.mustExec(`CREATE TABLE addon_quant_theme(id bigint UNSIGNED PRIMARY KEY,name varchar(128),status tinyint,deleted_at datetime(3));INSERT INTO addon_quant_theme VALUES(1,'示例题材',1,NULL)`)
	cr, err := svc.Create(e.ctx, 1, "test_qa_conversation_01", "示例问答会话")
	if err != nil {
		t.Fatal(err)
	}
	var conv conversation.Record
	if err := convRepo.Within(e.ctx, false, func(tx conversation.Tx) error { var err error; conv, err = tx.ByID(e.ctx, 1, cr.ID, false); return err }); err != nil {
		t.Fatal(err)
	}
	var now time.Time
	if err := convRepo.Within(e.ctx, false, func(tx conversation.Tx) error { var err error; now, err = tx.Now(e.ctx); return err }); err != nil {
		t.Fatal(err)
	}
	src := repository.NewAISourceRepo(e.db)
	s, err := src.RegisterSource(e.ctx, sources.Source{Domain: "qa.example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试授权", LicenseRef: "仅测试书面存档"})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.SetSourceEnabled(e.ctx, s.ID, true); err != nil {
		t.Fatal(err)
	}
	s.Enabled = true
	snap, err := sources.BuildSnapshot(s, "https://qa.example.test/source", sources.MediaPlain, []byte("示例题材目录列出测试设备分类。示例资料的第二句记载部件名称。"), now, 65536)
	if err != nil {
		t.Fatal(err)
	}
	snap, err = src.SaveSnapshot(e.ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	steps := qaSuccessSteps(snap.ID)
	if mode == "unknown" {
		steps = []providertest.Step{providertest.Drop()}
	}
	if mode == "hang" {
		steps = []providertest.Step{providertest.Hang()}
	}
	if mode == "protocol" {
		steps = []providertest.Step{providertest.Success(`{"answer":"自由正文不允许"}`, provider.Usage{Prompt: 20, Completion: 30, Total: 50})}
	}
	if mode == "token_expire" {
		// JWT秒级取整，预留初始鉴权窗口；只延迟测试上游，不改产品执行时限。
		steps[0] = providertest.SlowHeader(5100*time.Millisecond, steps[0])
	}
	if mode == "revoke" || mode == "member_off" {
		steps[1] = providertest.Handler(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			query, arg := `UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, s.ID
			if mode == "member_off" {
				query, arg = `UPDATE addon_member SET status=0 WHERE id=?`, 1
			}
			if _, err := e.db.ExecContext(ctx, query, arg); err != nil {
				t.Error("模型调用期间来源撤回未完成", err)
				http.Error(w, "test-only", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"test-revoke","model":"qa-sql-test","choices":[{"finish_reason":"stop","message":{"content":"{\"verdict\":\"pass\",\"reason\":\"verified\"}"}}],"usage":{"prompt_tokens":20,"completion_tokens":30,"total_tokens":50}}`)
		})
	}
	upstream := providertest.NewServer(t, steps...)
	direct, err := provider.NewDirect(provider.Config{Enabled: true, Name: "问答真库测试", BaseURL: upstream.URL(), Model: "qa-sql-test", APIKey: "test-only", TimeoutMS: 20000}, true)
	if err != nil {
		t.Fatal(err)
	}
	m := qa.Model{Name: "qa-sql-test", FilingNo: "仅测试备案", ApprovalRef: "仅测试模型批准", QualityRef: "仅测试质量记录", BudgetRef: "仅测试日预算", ProfileDigest: direct.DirectProfileDigest()}
	cfg := qa.Config{Selector: m, Entailment: m, Semantic: m, AllowedDomains: []string{s.Domain}, Limits: qa.Limits{MaxSources: 4, MaxSentences: 32, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second}}
	scope := qa.Scope{SubjectType: "theme", SubjectID: 1, SubjectName: "示例题材", ApprovalRef: "仅测试范围批准", ValidUntil: now.Add(time.Hour), Snapshots: []sources.Snapshot{snap}}
	in := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "示例题材目录列出什么分类？"}
	p, err := qa.NewPlan(qa.Identity{UserID: 1, ConversationID: cr.ID, MessageSeq: 1}, in, scope, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := json.Marshal(p.Proof().Evidence)
	if err != nil {
		t.Fatal(err)
	}
	e.mustExec(`INSERT INTO tm_ai_qa_scopes(subject_type,subject_id,subject_name,approval_ref,policy_digest,evidence_json,valid_until,enabled,version,created_at,updated_at) VALUES('theme',1,?,?,?,?,?,0,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, scope.SubjectName, scope.ApprovalRef, p.Proof().PolicyDigest, ev, scope.ValidUntil.Format("2006-01-02 15:04:05.000"))
	e.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
	e.mustExec(`INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,?,0,10,20,1000000,2000000,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.BudgetRef)
	e.mustExec(`UPDATE tm_ai_qa_limits SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)`)
	repo := repository.NewAIQARepoWithIsolation(e.db, level)
	ctl, err := qa.NewDurableControl(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := qa.NewEngine(direct, direct, direct, ctl)
	if err != nil {
		t.Fatal(err)
	}
	return &qaSQLFixture{e, repo, p, ctl, engine, upstream, cfg, scope, in, conv, direct, s}
}

type qaSQLUnknown struct {
	qa.Store
	proof  qa.Proof
	target string
	once   atomic.Bool
}

func (s *qaSQLUnknown) WithinQA(ctx context.Context, write bool, fn func(qa.Tx) error) error {
	eligible := false
	err := s.Store.WithinQA(ctx, write, func(tx qa.Tx) error {
		if err := fn(tx); err != nil {
			return err
		}
		if !write {
			return nil
		}
		r, err := tx.Question(ctx, s.proof.Identity, false)
		if err != nil {
			return err
		}
		eligible = r.State == s.target
		if s.target == "call" {
			calls, err := tx.Calls(ctx, s.proof.InputDigest)
			if err != nil {
				return err
			}
			eligible = len(calls) == 1 && calls[0].State == "recorded"
		}
		return nil
	})
	if err == nil && eligible && s.once.CompareAndSwap(false, true) {
		return qa.ErrUnavailable
	}
	return err
}

func TestIntegrationAIConversationsQARecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"queued", "running", "answered", "call", "corrupt", "missing", "revoke", "scope_changed"} {
				t.Run(mode, func(t *testing.T) {
					x := newQASQL(t, level, mode)
					var store qa.Store = x.repo
					if mode == "queued" || mode == "running" || mode == "answered" || mode == "call" {
						store = &qaSQLUnknown{Store: x.repo, proof: x.plan.Proof(), target: mode}
					}
					ctl, err := qa.NewDurableControl(store, x.plan)
					if err != nil {
						t.Fatal(err)
					}
					engine, err := qa.NewEngine(x.direct, x.direct, x.direct, ctl)
					if err != nil {
						t.Fatal(err)
					}
					_, err = ctl.Enqueue(x.ctx, "test_qa_question_001", 1)
					if mode == "queued" {
						if err == nil || x.record(t).State != "queued" {
							t.Fatal("入队未在真实提交后制造未知")
						}
						if _, err := ctl.Enqueue(x.ctx, "test_qa_question_001", 1); err != nil || x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 1 || x.provider.Count() != 0 {
							t.Fatal("原键恢复重复写入", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					_, runErr := ctl.Run(x.ctx, engine)
					if mode == "running" {
						if runErr == nil || x.record(t).State != "running" || x.provider.Count() != 0 || x.queryInt(`SELECT SUM(tokens) FROM tm_ai_qa_quotas`) != 78144 {
							t.Fatal("未知执行权被重跑/释放")
						}
						if _, err := ctl.Run(x.ctx, engine); err == nil || x.record(t).State != "running" || x.provider.Count() != 0 {
							t.Fatal("未知执行权重放破坏状态")
						}
						return
					}
					if mode == "answered" {
						if runErr == nil || x.record(t).State != "answered" {
							t.Fatal("终态提交未知被覆盖")
						}
						if _, err := engine.ReadStored(x.ctx, x.plan); err != nil || x.provider.Count() != 3 {
							t.Fatal("终态只读恢复失败", err)
						}
						return
					}
					if mode == "call" || mode == "revoke" {
						want := 1
						if mode == "revoke" {
							want = 2
						}
						if runErr == nil || x.record(t).State != "failed" || x.provider.Count() != want || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls WHERE state='recorded'`) != want || x.queryInt(`SELECT SUM(tokens) FROM tm_ai_qa_quotas`) != 78144 {
							t.Fatal("调用审计未知/在途撤权处理不符", runErr)
						}
						return
					}
					if runErr != nil {
						t.Fatal(runErr)
					}
					switch mode {
					case "corrupt":
						var calls []qa.CallRecord
						if err := x.repo.WithinQA(x.ctx, false, func(tx qa.Tx) error {
							var err error
							calls, err = tx.Calls(x.ctx, x.plan.Proof().InputDigest)
							return err
						}); err != nil {
							t.Fatal(err)
						}
						calls[1].Result.Decision = qa.Decision{Verdict: "block", Reason: "unsupported"}
						raw, err := json.Marshal(calls[1].Result)
						if err != nil {
							t.Fatal(err)
						}
						x.mustExec(`DROP TRIGGER tm_ai_qa_call_update`)
						x.mustExec(`UPDATE tm_ai_qa_calls SET result_json=? WHERE step_no=2`, raw)
					case "missing":
						x.mustExec(`DROP TRIGGER tm_ai_qa_call_no_delete`)
						x.mustExec(`DELETE FROM tm_ai_qa_calls WHERE step_no=3`)
					case "scope_changed":
						x.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=0,version=version+1,updated_at=UTC_TIMESTAMP(3)`)
					}
					if out, err := engine.ReadStored(x.ctx, x.plan); err == nil || len(out.Quotes) != 0 || x.provider.Count() != 3 {
						t.Fatal("损坏/撤回的原存证仍交付")
					}
					x.uncharged(t)
				})
			}
		})
	}
}
func (x *qaSQLFixture) record(t *testing.T) qa.Record {
	t.Helper()
	var r qa.Record
	if err := x.repo.WithinQA(x.ctx, false, func(tx qa.Tx) error {
		var err error
		r, err = tx.Question(x.ctx, x.plan.Proof().Identity, false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return r
}
func (x *qaSQLFixture) uncharged(t *testing.T) {
	t.Helper()
	if x.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`) != 0 || x.queryInt(`SELECT COUNT(*) FROM tm_points_holds`) != 0 || x.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 0 {
		t.Fatal("问答产生了任务/冻结")
	}
}

func TestIntegrationAIConversationsQALifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			x := newQASQL(t, level, "success")
			var wg sync.WaitGroup
			var good atomic.Int32
			issues := make(chan error, 20)
			for n := 0; n < 20; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if r, err := x.control.Enqueue(x.ctx, "test_qa_question_001", 1); err == nil && r.State == "queued" {
						good.Add(1)
					} else if err != nil {
						issues <- err
					} else {
						issues <- qa.ErrProtocol
					}
				}()
			}
			wg.Wait()
			close(issues)
			for err := range issues {
				t.Error("并发原键入队响应失败", err)
			}
			if good.Load() != 20 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 1 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 1 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_quotas`) != 0 || x.provider.Count() != 0 {
				t.Fatal("原键入队未原子/幂等", good.Load())
			}
			good.Store(0)
			// 实际目录可能包含全角空白；按原报告相同的Normalize口径核验同一对象，而不是另造别名。
			x.mustExec(`UPDATE addon_quant_theme SET name=CONCAT(name,'　') WHERE id=1`)
			for n := 0; n < 16; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ctl, err := qa.NewDurableControl(x.repo, x.plan)
					if err != nil {
						return
					}
					engine, err := qa.NewEngine(x.direct, x.direct, x.direct, ctl)
					if err != nil {
						return
					}
					if _, err = ctl.Run(x.ctx, engine); err == nil {
						good.Add(1)
					}
				}()
			}
			wg.Wait()
			if good.Load() != 1 || x.provider.Count() != 3 {
				t.Fatal("唯一执行/失败所有权不符", good.Load(), x.provider.Count())
			}
			r := x.record(t)
			if r.State != "answered" || r.Calls != 3 || r.ReservedTokens != 39072 || x.queryInt(`SELECT SUM(requests) FROM tm_ai_qa_quotas`) != 2 || x.queryInt(`SELECT SUM(tokens) FROM tm_ai_qa_quotas`) != 78144 {
				t.Fatal("最终状态/双层最坏预留不符", r)
			}
			// 仅重新实例化Repo/控制器/Engine并沿用可信Plan，不包含实际进程终止/启动验收。
			ctl, err := qa.NewDurableControl(repository.NewAIQARepo(x.db), x.plan)
			if err != nil {
				t.Fatal(err)
			}
			engine, err := qa.NewEngine(x.direct, x.direct, x.direct, ctl)
			if err != nil {
				t.Fatal(err)
			}
			view, err := engine.ReadStored(x.ctx, x.plan)
			if err != nil || len(view.Quotes) != 1 || view.Quotes[0].Text != x.scope.Snapshots[0].Sentences[0].Text || x.provider.Count() != 3 {
				t.Fatal("存证重读失败", err)
			}
			if _, err := ctl.Run(x.ctx, engine); err == nil || x.provider.Count() != 3 || x.record(t).State != "answered" {
				t.Fatal("重放重跑或覆盖终态")
			}
			if err := ctl.Fail(x.ctx, "unconfirmed"); err != nil || x.record(t).State != "answered" {
				t.Fatal("后到失败覆盖终态", err)
			}
			x.mustExec(`UPDATE tm_ai_conversations SET archived=1,version=version+1,updated_at=UTC_TIMESTAMP(3) WHERE id=?`, x.conv.ID)
			if _, err := engine.ReadStored(x.ctx, x.plan); err != nil {
				t.Fatal("归档阻止授权历史读取", err)
			}
			x.mustExec(`UPDATE tm_ai_sources SET enabled=0 WHERE id=?`, x.source.ID)
			if v, err := engine.ReadStored(x.ctx, x.plan); err == nil || len(v.Quotes) != 0 || x.provider.Count() != 3 {
				t.Fatal("撤权后仍交付")
			}
			x.uncharged(t)
		})
	}
}

func TestIntegrationAIConversationsQAFailures(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"unknown", "protocol", "hang"} {
				t.Run(mode, func(t *testing.T) {
					x := newQASQL(t, level, mode)
					if _, err := x.control.Enqueue(x.ctx, "test_qa_question_001", 1); err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithCancel(x.ctx)
					defer cancel()
					if mode == "hang" {
						go func() {
							if x.provider.WaitCount(1, time.Second) {
								cancel()
							}
						}()
					}
					if v, err := x.control.Run(ctx, x.engine); err == nil || v != nil {
						t.Fatal("异常仍成功")
					}
					r := x.record(t)
					if r.State != "failed" || r.Calls != 1 || r.ReservedTokens != 13024 || x.provider.Count() != 1 || x.queryInt(`SELECT SUM(tokens) FROM tm_ai_qa_quotas`) != 78144 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls WHERE state='recorded'`) != 1 {
						t.Fatal("失败丢失审计或释放配额", r)
					}
					if _, err := x.engine.ReadStored(x.ctx, x.plan); err == nil {
						t.Fatal("失败交付引文")
					}
					if _, err := x.control.Run(x.ctx, x.engine); err == nil || x.provider.Count() != 1 {
						t.Fatal("失败自动重跑")
					}
					x.uncharged(t)
				})
			}
		})
	}
}

func TestIntegrationAIConversationsQABoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"quota", "scope", "policy", "member", "owner", "version", "atomic", "constraints"} {
				t.Run(mode, func(t *testing.T) {
					x := newQASQL(t, level, "success")
					switch mode {
					case "scope":
						x.mustExec(`UPDATE tm_ai_qa_scopes SET enabled=0,version=version+1,updated_at=UTC_TIMESTAMP(3)`)
					case "policy":
						x.mustExec(`UPDATE tm_ai_qa_limits SET enabled=0,version=version+1,updated_at=UTC_TIMESTAMP(3)`)
					case "member":
						x.mustExec(`UPDATE addon_member SET status=0 WHERE id=1`)
					case "owner":
						p, err := qa.NewPlan(qa.Identity{UserID: 2, ConversationID: x.conv.ID, MessageSeq: 1}, x.input, x.scope, x.cfg, time.Now().UTC().Truncate(time.Millisecond))
						if err != nil {
							t.Fatal(err)
						}
						x.control, err = qa.NewDurableControl(x.repo, p)
						if err != nil {
							t.Fatal(err)
						}
					case "atomic":
						x.mustExec(`CREATE TRIGGER test_qa_insert_failure BEFORE INSERT ON tm_ai_qa_requests FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test-only-rollback'`)
					}
					version := int64(1)
					if mode == "version" {
						version = 2
					}
					r, err := x.control.Enqueue(x.ctx, "test_qa_question_001", version)
					if mode != "quota" && mode != "constraints" {
						if err == nil || x.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages`) != 0 || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests`) != 0 || x.queryInt(`SELECT last_seq FROM tm_ai_conversations`) != 0 || x.queryInt(`SELECT version FROM tm_ai_conversations`) != 1 || x.provider.Count() != 0 {
							t.Fatal("拒绝后残留消息/执行/版本", err)
						}
						return
					}
					if err != nil || r.State != "queued" {
						t.Fatal(err)
					}
					if mode == "quota" {
						x.mustExec(`UPDATE tm_ai_qa_limits SET user_tokens=1,instance_tokens=1000000,version=version+1,updated_at=UTC_TIMESTAMP(3)`)
						if err := x.control.Begin(x.ctx, x.plan.Proof()); !errors.Is(err, qa.ErrQuota) || x.record(t).State != "queued" || x.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_quotas`) != 0 || x.provider.Count() != 0 {
							t.Fatal("配额失败仍占执行权/单边预留", err)
						}
					} else {
						if _, err := x.control.Run(x.ctx, x.engine); err != nil {
							t.Fatal(err)
						}
						for _, query := range []string{`DELETE FROM tm_ai_qa_requests`, `DELETE FROM tm_ai_qa_calls`, `DELETE FROM tm_ai_qa_quotas`, `UPDATE tm_ai_qa_quotas SET tokens=tokens-1`, `UPDATE tm_ai_qa_requests SET state='failed',reason='unconfirmed',version=version+1`, `UPDATE tm_ai_qa_calls SET result_json='{}'`, `UPDATE tm_ai_qa_scopes SET enabled=0`, `UPDATE tm_ai_qa_limits SET enabled=0`} {
							if _, err := x.db.ExecContext(x.ctx, query); err == nil {
								t.Fatal("直接篡改被接受", query)
							}
						}
					}
					x.uncharged(t)
				})
			}
		})
	}
}
