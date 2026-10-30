package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/provider/providertest"
)

func TestLocalReportModelSessionAssembly(t *testing.T) {
	admission, _, _, _, _ := localAdmissionFixture(t)
	base, _, _ := reportFixture(t)
	session := strings.Repeat("a1", 32)
	server := providertest.NewServer(t, providertest.RequireBearer(session, providertest.Success("仅测试:回答", provider.Usage{Prompt: 1, Completion: 1, Total: 2})))
	cfg := provider.Config{Name: "local-eval", BaseURL: server.URL(), Model: "仅测试:模型", TimeoutMS: 1000}
	audit := &reportCheckControl{}
	_, _, plain, err := NewAILocalReportModelControls(admission, base.core.ledger, l1TestPayloadReader{}, cfg, audit)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", strings.Repeat("A", 64), "$TM_FORBIDDEN_VENDOR_KEY"} {
		if s, c, m, err := NewAILocalReportModelControlsWithSession(admission, base.core.ledger, l1TestPayloadReader{}, cfg, audit, token); err == nil || s != nil || c != nil || m != nil {
			t.Fatal("非法会话被接受")
		}
	}
	_, _, model, err := NewAILocalReportModelControlsWithSession(admission, base.core.ledger, l1TestPayloadReader{}, cfg, audit, session)
	if err != nil || model.DirectProfileDigest() != plain.DirectProfileDigest() {
		t.Fatal("会话改变了执行档案身份")
	}
	response, err := model.Generate(t.Context(), provider.Request{Prompt: "仅测试:检查临时父桥认证", MaxTokens: 16})
	if err != nil || response.Text != "仅测试:回答" {
		t.Fatal("临时认证请求未通过", err)
	}
	if requests := server.Requests(); len(requests) != 1 || requests[0].AuthDigest != providertest.AuthDigest("Bearer "+session) {
		t.Fatal("父桥会话未按原认证头发送")
	}
	cfg.APIKey = session
	if _, _, _, err := NewAILocalReportModelControlsWithSession(admission, base.core.ledger, l1TestPayloadReader{}, cfg, audit, session); err == nil {
		t.Fatal("借会话入口将Key放进配置")
	}
}

func TestLocalReportModelAssemblyIsLoopbackOnly(t *testing.T) {
	for _, mode := range []string{"正常", "远端", "HTTPS", "域名", "无端口", "路径", "查询", "密钥", "格式回退", "推理选项", "缺准入", "缺账本", "缺审计", "缺隔离读取"} {
		t.Run(mode, func(t *testing.T) {
			admission, _, _, _, _ := localAdmissionFixture(t)
			base, _, _ := reportFixture(t)
			ledger := base.core.ledger
			var audit AIReportCallAudit = &reportCheckControl{}
			var payloads = l1TestPayloadReader{}
			cfg := provider.Config{Name: "local-eval", BaseURL: "http://127.0.0.1:8123", Model: "仅测试:模型", TimeoutMS: 1000}
			switch mode {
			case "远端":
				cfg.BaseURL = "http://192.0.2.1:8123"
			case "HTTPS":
				cfg.BaseURL = "https://127.0.0.1:8123"
			case "域名":
				cfg.BaseURL = "http://localhost:8123"
			case "无端口":
				cfg.BaseURL = "http://127.0.0.1"
			case "路径":
				cfg.BaseURL += "/unknown"
			case "查询":
				cfg.BaseURL += "?other=1"
			case "密钥":
				cfg.APIKey = "$TM_FORBIDDEN_IN_LOCAL_LOOPBACK"
			case "格式回退":
				cfg.JSONSchemaFallback = true
			case "推理选项":
				cfg.ReasoningEffort = "low"
			case "缺准入":
				admission = nil
			case "缺账本":
				ledger = nil
			case "缺审计":
				audit = nil
			}
			var selection *AILocalReportSelectionControl
			var checks *AILocalReportCheckControl
			var model *provider.OpenAICompat
			var err error
			if mode == "缺隔离读取" {
				selection, checks, model, err = NewAILocalReportModelControls(admission, ledger, nil, cfg, audit)
			} else {
				selection, checks, model, err = NewAILocalReportModelControls(admission, ledger, payloads, cfg, audit)
			}
			if (err == nil) != (mode == "正常") || err != nil && (selection != nil || checks != nil || model != nil) {
				t.Fatal("回环装配边界不符", err)
			}
		})
	}
}

func TestLocalReportModelAuditFinishSurvivesRevocation(t *testing.T) {
	admission, st, _, path, _ := localAdmissionFixture(t)
	base, _, _ := reportFixture(t)
	audit := &reportCheckControl{finishErr: errPointsInjected}
	selection, checks, _, err := NewAILocalReportModelControls(admission, base.core.ledger, l1TestPayloadReader{}, provider.Config{Name: "local-eval", BaseURL: "http://127.0.0.1:8123", Model: "仅测试:模型", TimeoutMS: 1000}, audit)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	r := l1Stage(st.v, st.v.LeaseOwner, "extract", 2, task.StageDigest([]byte("operation")), task.StageDigest([]byte("input")))
	if err := selection.Finish(context.Background(), r, provider.Response{}, errPointsInjected); !errors.Is(err, errPointsInjected) || audit.finishes != 1 {
		t.Fatal("撤回后遗漏调用结果审计或吞掉持久化错误", err)
	}
	r.Stage, r.StepNo = "judge", 3
	if err := checks.Finish(context.Background(), r, provider.Response{}, nil); !errors.Is(err, errPointsInjected) || audit.finishes != 2 {
		t.Fatal("判定结果未交审计", err)
	}
	if err := selection.Start(context.Background(), r, provider.Request{}); !errors.Is(err, points.ErrInvalid) || audit.starts != 0 {
		t.Fatal("错阶段触发调用前审计", err)
	}
}
