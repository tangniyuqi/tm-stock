package service

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

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
