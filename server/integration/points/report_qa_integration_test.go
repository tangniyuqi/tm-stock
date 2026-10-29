//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type reportQAAuth map[string]int64

func (a reportQAAuth) Authenticate(_ context.Context, token string) (int64, error) {
	if uid, ok := a[token]; ok {
		return uid, nil
	}
	return 0, memberauth.ErrUnauthorized
}

type reportQAPolicies struct{}

func (reportQAPolicies) ForPolicy(context.Context, string) (qa.Config, error) {
	return qa.Config{}, qa.ErrUnavailable
}

type reportQAModels struct{ model *provider.OpenAICompat }

func (p reportQAModels) ForPlan(context.Context, qa.Proof) ([3]*provider.OpenAICompat, error) {
	return [3]*provider.OpenAICompat{p.model, p.model, p.model}, nil
}

type reportQAConfig struct{ cfg qa.Config }

func (c reportQAConfig) ForReport(context.Context, qa.ReportEvidence) (qa.Config, error) {
	return c.cfg, nil
}

func qaID(id int64) string { return strconv.FormatInt(id, 10) }

func TestIntegrationAIReportQA(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newWorkflowEnv(t, level, "pass")
			w := e.worker(t, true)
			if _, err := w.Run(e.ctx, e.v.UserID, e.v.ID); err != nil {
				t.Fatal("报告任务未完成", err)
			}
			if e.queryInt("SELECT status FROM tm_ai_tasks WHERE id="+qaID(e.v.ID)) != int(task.Succeeded) {
				t.Fatal("报告任务未成功")
			}
			web := reportHTTPFixtureWithModel(t, e.reportEnv, "", true, "测试模型", "test-only-filing")
			if _, err := web.reader.Read(e.ctx, 1, e.v.ID); err != nil {
				t.Fatal("原报告当前不可读", err)
			}
			reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_conversations.sql")
			reportDDL(t, e.pointsTestEnv, "20261021_create_tm_ai_qa.sql")
			e.mustExec("INSERT INTO tm_ai_qa_limits(id,approval_ref,enabled,user_requests,instance_requests,user_tokens,instance_tokens,version,created_at,updated_at) VALUES(1,'test-only-budget',0,10,20,1000000,2000000,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))")
			e.mustExec("UPDATE tm_ai_qa_limits SET enabled=1,version=2,updated_at=UTC_TIMESTAMP(3)")

			convRepo := repository.NewAIConversationRepo(e.db)
			convSvc, err := service.NewAIConversationService(convRepo)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := convSvc.Create(e.ctx, 1, "test_report_qa_conversation_01", "报告追问测试")
			if err != nil {
				t.Fatal(err)
			}
			var conv conversation.Record
			if err := convRepo.Within(e.ctx, false, func(tx conversation.Tx) error {
				var err error
				conv, err = tx.ByID(e.ctx, 1, receipt.ID, false)
				return err
			}); err != nil {
				t.Fatal(err)
			}

			usage := provider.Usage{Prompt: 20, Completion: 30, Total: 50}
			steps := []providertest.Step{}
			for n := 0; n < 2; n++ {
				steps = append(steps,
					providertest.Success(`{"action":"answer","reason":"none","citations":[{"snapshot_id":1,"sentence_ids":[1]}]}`, usage),
					providertest.Success(`{"verdict":"pass","reason":"verified"}`, usage),
					providertest.Success(`{"verdict":"pass","reason":"verified"}`, usage),
				)
			}
			upstream := providertest.NewServer(t, steps...)
			direct, err := provider.NewDirect(provider.Config{Enabled: true, Name: "报告问答测试", BaseURL: upstream.URL(), Model: "qa-report-test", APIKey: "test-only", TimeoutMS: 20000}, true)
			if err != nil {
				t.Fatal(err)
			}
			model := qa.Model{Name: "qa-report-test", FilingNo: "test-only-filing", ApprovalRef: "test-only-approval", QualityRef: "test-only-quality", BudgetRef: "test-only-budget", ProfileDigest: direct.DirectProfileDigest()}
			cfg := qa.Config{Selector: model, Entailment: model, Semantic: model, Limits: qa.Limits{MaxSources: 16, MaxSentences: 64, MaxEvidenceBytes: 65536, MaxPromptBytes: 32768, MaxInputTokens: 12000, MaxOutputTokens: 1024, MaxTotalTokens: 39072, Timeout: 20 * time.Second}}
			realEvidence, err := service.NewAIReportQAEvidence(web.reader)
			if err != nil {
				t.Fatal(err)
			}
			reportQuotes, err := realEvidence.ForQA(e.ctx, 1, e.v.ID)
			if err != nil {
				t.Fatal("读取报告证据失败", err)
			}
			if len(reportQuotes.Quotes) != 3 {
				t.Fatal("报告追问证据应只含三节AI引文", len(reportQuotes.Quotes))
			}
			for _, q := range reportQuotes.Quotes {
				if strings.Contains(q.Text, "公司信息见本段") {
					t.Fatal("公司事实进入了报告追问证据")
				}
			}
			evidence := realEvidence
			svc, err := service.NewAIQAServiceWithReports(repository.NewAIQARepo(e.db), reportQAPolicies{}, reportQAModels{direct}, reportQAAuth{"one": 1, "two": 2}, true, evidence, reportQAConfig{cfg})
			if err != nil {
				t.Fatal(err)
			}
			key := "question_report_qa_0001"
			input := dto.AIQAInput{Version: conv.Version, SubjectType: "theme", SubjectID: e.v.SubjectID, Skill: "report_qa", TaskID: e.v.ID, Question: "报告的题材定义引用了什么分类？"}
			txnsBefore := e.queryInt("SELECT COUNT(*) FROM tm_points_txns")
			status, err := svc.Submit(e.ctx, "one", 1, conv.ID, key, input)
			if err != nil {
				t.Logf("after submit fail messages=%d requests=%d quotas=%d", e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages"), e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests"), e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_quotas"))
				t.Fatal("报告追问提交失败", err)
			}
			if status.State != "answered" || status.Skill != "report_qa" || status.Question != input.Question {
				t.Fatal("报告追问状态不符", status)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE kind='question' AND skill='report_qa' AND task_id="+qaID(e.v.ID)) != 1 {
				t.Fatal("报告追问未写入唯一请求/消息")
			}
			if upstream.Count() != 3 {
				t.Fatal("报告追问模型调用次数不符", upstream.Count())
			}
			answer, err := svc.Answer(e.ctx, "one", 1, conv.ID, status.Seq)
			if err != nil {
				t.Fatal("报告追问答案读取失败", err)
			}
			if answer.State != "answered" || len(answer.Quotes) != 1 || answer.Quotes[0].Excerpt == "" || answer.Quotes[0].SourceURL == "" {
				t.Fatal("报告追问答案不符", answer)
			}
			// 模拟重启为只读：不装配模型客户端，也不重写原问题或证明。
			readOnly, err := service.NewAIQAServiceWithReportAccess(repository.NewAIQARepo(e.db), nil, nil, reportQAAuth{"one": 1}, false, false, evidence, reportQAConfig{cfg})
			if err != nil {
				t.Fatal(err)
			}
			readBack, err := readOnly.Answer(e.ctx, "one", 1, conv.ID, status.Seq)
			if err != nil || len(readBack.Quotes) != 1 || readBack.Quotes[0] != answer.Quotes[0] {
				t.Fatal("关闭提交及模型后历史答案不可读", err)
			}
			if _, err := readOnly.Submit(e.ctx, "one", 1, conv.ID, "question_read_only_denied", input); !errors.Is(err, qa.ErrUnavailable) {
				t.Fatal("只读报告装配放行了新问题", err)
			}
			if upstream.Count() != 3 || e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != 1 {
				t.Fatal("只读回放或拒绝新提交增加了调用或请求")
			}
			if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 0 || e.queryInt("SELECT COUNT(*) FROM tm_points_txns") != txnsBefore {
				t.Fatal("报告追问改动了积分")
			}
			// 同一原键并发提交只能执行一次；失败方只允许冲突/不可用，不得产生第二次模型调用。
			var latest conversation.Record
			if err := convRepo.Within(e.ctx, false, func(tx conversation.Tx) error {
				var err error
				latest, err = tx.ByID(e.ctx, 1, conv.ID, false)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			key2 := "question_report_qa_0002"
			second := input
			second.Version, second.Question = latest.Version, "报告的事件时间线记载了什么？"
			var wg sync.WaitGroup
			outcomes := make([]error, 8)
			for n := 0; n < len(outcomes); n++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					_, outcomes[index] = svc.Submit(e.ctx, "one", 1, conv.ID, key2, second)
				}(n)
			}
			wg.Wait()
			accepted := 0
			for _, err := range outcomes {
				if err == nil {
					accepted++
					continue
				}
				if !errors.Is(err, qa.ErrConflict) && !errors.Is(err, qa.ErrUnavailable) {
					t.Fatal("并发报告追问返回未预期错误", err)
				}
			}
			if accepted < 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != 2 || e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages") != 2 || upstream.Count() != 6 {
				t.Fatal("并发报告追问未恰好执行一次", accepted, e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests"), upstream.Count())
			}
			status2, err := svc.Receipt(e.ctx, "one", 1, conv.ID, key2)
			if err != nil || status2.State != "answered" {
				t.Fatal("并发报告追问终态不符", status2, err)
			}
			// 重复读取答案不得再次调用模型。
			for n := 0; n < 2; n++ {
				if _, err := svc.Answer(e.ctx, "one", 1, conv.ID, status2.Seq); err != nil {
					t.Fatal("重复读取报告追问答案失败", err)
				}
			}
			if upstream.Count() != 6 {
				t.Fatal("重复读取答案触发了模型调用", upstream.Count())
			}
			if _, err := svc.Submit(e.ctx, "bad", 1, conv.ID, "question_report_qa_0009", second); !errors.Is(err, memberauth.ErrUnauthorized) {
				t.Fatal("无效令牌未被拒绝", err)
			}

			if _, err := svc.Receipt(e.ctx, "two", 2, conv.ID, key); !errors.Is(err, qa.ErrNotFound) && !errors.Is(err, conversation.ErrNotFound) {
				t.Fatal("他人报告追问未被拒绝", err)
			}
			if _, err := svc.Answer(e.ctx, "two", 2, conv.ID, status.Seq); !errors.Is(err, qa.ErrNotFound) && !errors.Is(err, conversation.ErrNotFound) {
				t.Fatal("他人读取报告追问答案未被拒绝", err)
			}
			// 他人不能用同一个任务号提交报告追问，也不得留下消息或请求。
			beforeMessages, beforeRequests := e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages"), e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests")
			if _, err := svc.Submit(e.ctx, "two", 2, conv.ID, "question_report_qa_foreign", input); !errors.Is(err, qa.ErrNotFound) && !errors.Is(err, conversation.ErrNotFound) {
				t.Fatal("他人任务号报告追问未被拒绝", err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages") != beforeMessages || e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != beforeRequests {
				t.Fatal("他人任务号报告追问留下了消息或请求")
			}
			// 不能借其他任务号把别的报告塞进当前会话。
			foreign := input
			foreign.TaskID = e.v.ID + 1
			if _, err := svc.Submit(e.ctx, "one", 1, conv.ID, "question_report_qa_0002", foreign); !errors.Is(err, qa.ErrNotFound) && !errors.Is(err, qa.ErrUnavailable) {
				t.Fatal("不存在的报告任务未被拒绝", err)
			}
			if e.queryInt("SELECT COUNT(*) FROM tm_ai_conversation_messages") != 2 || e.queryInt("SELECT COUNT(*) FROM tm_ai_qa_requests") != 2 {
				t.Fatal("越权/不存在报告留下了请求或消息")
			}
			if err := repository.NewAISourceRepo(e.db).SetSourceEnabled(e.ctx, e.source.ID, false); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Answer(e.ctx, "one", 1, conv.ID, status.Seq); !errors.Is(err, qa.ErrUnavailable) {
				t.Fatal("来源撤回后仍读取报告追问", err)
			}
			if _, err := readOnly.Answer(e.ctx, "one", 1, conv.ID, status.Seq); !errors.Is(err, qa.ErrUnavailable) {
				t.Fatal("只读装配在来源撤回后仍返回历史答案", err)
			}
			if _, err := svc.Submit(e.ctx, "one", 1, conv.ID, "question_report_qa_0003", input); !errors.Is(err, qa.ErrUnavailable) {
				t.Fatal("来源撤回后仍可提交报告追问", err)
			}
		})
	}
}
