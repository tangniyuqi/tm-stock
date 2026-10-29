package finalreport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func auditRecordFixture() CallAudit {
	return CallAudit{Request: task.StageRequest{UserID: 1, TaskID: 2, StepNo: 2, Stage: "extract", Owner: "test_audit_owner_0001", Fence: 1, OperationKey: strings.Repeat("a", 64), InputDigest: strings.Repeat("b", 64)}, Phase: "start", TaskNo: strings.Repeat("c", 32), AdmissionDigest: strings.Repeat("d", 64), ProfileDigest: strings.Repeat("e", 64), RequestDigest: strings.Repeat("f", 64), Outcome: "started", CreatedAt: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}
}
func TestCallAuditRecordClosedStates(t *testing.T) {
	for _, mode := range []string{"正常", "判定", "错误返回", "阶段", "步号", "时点", "任务号", "准入", "请求", "结果", "结果状态", "原键"} {
		t.Run(mode, func(t *testing.T) {
			a := auditRecordFixture()
			switch mode {
			case "判定":
				a.Request.Stage = "judge"
				a.Request.StepNo = 4
			case "错误返回":
				a.Phase = "finish"
				a.Outcome = "errored"
				a.ResultDigest = strings.Repeat("0", 64)
			case "阶段":
				a.Request.Stage = "other"
			case "步号":
				a.Request.StepNo = 5
			case "时点":
				a.CreatedAt = a.CreatedAt.Add(time.Nanosecond)
			case "任务号":
				a.TaskNo = "other"
			case "准入":
				a.AdmissionDigest = ""
			case "请求":
				a.RequestDigest = ""
			case "结果":
				a.ResultDigest = strings.Repeat("0", 64)
			case "结果状态":
				a.Phase = "finish"
				a.Outcome = "pass"
				a.ResultDigest = strings.Repeat("0", 64)
			case "原键":
				a.Request.OperationKey = "bad"
			}
			valid := mode == "正常" || mode == "判定" || mode == "错误返回"
			if (a.Validate() == nil) != valid {
				t.Fatal("审计状态不封闭")
			}
		})
	}
}
func TestCallAuditDigestsDoNotPersistContent(t *testing.T) {
	a := auditRecordFixture()
	zero := 0.0
	req := provider.Request{TaskID: a.TaskNo, Step: 2, Attempt: 1, Model: "仅测试:模型", System: "仅测试:敏感提示", Prompt: "仅测试:原始资料", MaxTokens: 10, Format: provider.FormatJSONSchema, Strict: true, Temperature: &zero, Schema: []byte(`{"type":"object"}`)}
	digest, err := ReportRequestDigest(a.Request, a.TaskNo, req.Model, req)
	if err != nil || !ValidDigest(digest) {
		t.Fatal(err)
	}
	req.Prompt += "其它"
	changed, err := ReportRequestDigest(a.Request, a.TaskNo, req.Model, req)
	if err != nil || changed == digest {
		t.Fatal("请求变更未改变摘要", err)
	}
	req.Attempt = 2
	if _, err := ReportRequestDigest(a.Request, a.TaskNo, req.Model, req); err == nil {
		t.Fatal("允许隐藏重试")
	}
	result := provider.Response{Text: "仅测试:原始结果", Raw: []byte("仅测试:原始响应")}
	d1, err := ReportResultDigest(result, false)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := ReportResultDigest(result, true)
	if err != nil || d1 == d2 {
		t.Fatal("结果状态未绑定", err)
	}
	a.Phase, a.Outcome, a.RequestDigest, a.ResultDigest = "finish", "returned", digest, d1
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"敏感提示", "原始资料", "原始结果", "原始响应"} {
		if strings.Contains(string(b), private) {
			t.Fatal("审计泄漏正文")
		}
	}
	if strings.Contains(a.String(), a.RequestDigest) || a.String() != a.GoString() {
		t.Fatal("格式化泄漏身份")
	}
}
