package agenttask

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func fixture() (Plan, task.Task, agents.Record) {
	v := task.Task{ID: 9, UserID: 2, No: strings.Repeat("a", 32), SubjectType: task.Theme, SubjectID: 3, AnalysisType: 1, Budget: task.Budget{MaxAttempts: 3, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 30000}}
	p := Plan{UserID: 2, TaskID: 9, TaskNo: v.No, SubjectType: task.Theme, SubjectID: 3, ActorID: 7, OperationKey: "test_agent_operation", Symbol: "600001", ProfileDigest: strings.Repeat("b", 64), Units: task.Units{Calls: 2, Tokens: 100}}
	p.OperationKey = p.RequestID()
	r := agents.Record{ID: 4, ActorID: 7, Request: agents.Request{RequestID: p.RequestID(), Symbol: p.Symbol}, RemoteID: "test_remote_id", ProfileDigest: p.ProfileDigest, State: agents.Quarantined, ResultDigest: strings.Repeat("c", 64)}
	return p, v, r
}

func TestPlanBindingAndClosedReceipt(t *testing.T) {
	p, v, r := fixture()
	if p.Validate(v) != nil || !agents.ValidRequestID(p.RequestID()) || !agents.ValidDigest(p.InputDigest(v)) {
		t.Fatal("计划无效")
	}
	for _, alter := range []func(*Plan){func(p *Plan) { p.Symbol = "600002" }, func(p *Plan) { p.ActorID++ }, func(p *Plan) { p.ProfileDigest = strings.Repeat("d", 64) }, func(p *Plan) { p.Units.Tokens++ }} {
		q := p
		alter(&q)
		if q.RequestID() != p.RequestID() || q.InputDigest(v) == p.InputDigest(v) {
			t.Fatal("计划变更可以规避原键")
		}
	}
	q := p
	q.OperationKey = "test_other_operation"
	if q.RequestID() != p.RequestID() || q.Validate(v) == nil {
		t.Fatal("换操作键绕过单阶段绑定")
	}
	payload, policy, err := ReceiptPolicy(p, p.InputDigest(v), r)
	if err != nil || !policy.Valid() || task.CheckStageJSON(payload) != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if json.Unmarshal(payload, &fields) != nil || len(fields) != 4 {
		t.Fatal("回执字段越界")
	}
	in := task.StageValidationInput{Stage: "agent_call", InputDigest: p.InputDigest(v), Payload: string(payload)}
	if policy.Check(context.Background(), in) != nil {
		t.Fatal("回执未通过")
	}
	for _, field := range []string{"stage", "input", "payload"} {
		x := in
		if field == "stage" {
			x.Stage = "extract"
		}
		if field == "input" {
			x.InputDigest = strings.Repeat("e", 64)
		}
		if field == "payload" {
			x.Payload = `{"test":"未核验正文"}`
		}
		if policy.Check(context.Background(), x) == nil {
			t.Fatal("篡改回执放行")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if policy.Check(ctx, in) == nil {
		t.Fatal("取消仍通过")
	}
	_, again, _ := ReceiptPolicy(p, p.InputDigest(v), r)
	if again.Digest != policy.Digest {
		t.Fatal("策略不稳定")
	}
	r.ResultDigest = strings.Repeat("f", 64)
	_, changed, _ := ReceiptPolicy(p, p.InputDigest(v), r)
	if changed.Digest == policy.Digest {
		t.Fatal("策略未绑定结果")
	}
}

func TestPlanAndReceiptRejectInvalidFields(t *testing.T) {
	for _, alter := range []func(*Plan, *task.Task){
		func(_ *Plan, v *task.Task) { v.ID = 0 }, func(_ *Plan, v *task.Task) { v.SubjectType = "stock" }, func(p *Plan, _ *task.Task) { p.UserID++ }, func(p *Plan, _ *task.Task) { p.TaskID++ }, func(p *Plan, _ *task.Task) { p.TaskNo = "" }, func(p *Plan, _ *task.Task) { p.SubjectID++ }, func(p *Plan, _ *task.Task) { p.SubjectType = task.Topic }, func(p *Plan, _ *task.Task) { p.ActorID = 0 }, func(p *Plan, _ *task.Task) { p.OperationKey = "bad" }, func(p *Plan, _ *task.Task) { p.Symbol = "X" }, func(p *Plan, _ *task.Task) { p.ProfileDigest = "bad" }, func(p *Plan, _ *task.Task) { p.Units = task.Units{Pages: 1} }, func(p *Plan, _ *task.Task) { p.Units.Tokens = 99999 }, func(_ *Plan, v *task.Task) { v.Budget.MaxCalls = 0 }, func(_ *Plan, v *task.Task) { v.CurrentStep = 2 },
	} {
		p, v, _ := fixture()
		alter(&p, &v)
		if p.Validate(v) == nil {
			t.Fatal(p, v)
		}
	}
	for _, alter := range []func(*agents.Record){func(r *agents.Record) { r.ID = 0 }, func(r *agents.Record) { r.State = agents.Completed; r.ResultDigest = "" }, func(r *agents.Record) { r.Request.RequestID = "test_wrong_request" }, func(r *agents.Record) { r.Request.Symbol = "600002" }, func(r *agents.Record) { r.ActorID++ }, func(r *agents.Record) { r.ProfileDigest = strings.Repeat("a", 64) }} {
		p, v, r := fixture()
		alter(&r)
		if _, _, err := ReceiptPolicy(p, p.InputDigest(v), r); err == nil {
			t.Fatal(r)
		}
	}
	p, _, r := fixture()
	if _, _, err := ReceiptPolicy(p, "bad", r); err == nil {
		t.Fatal("非法输入摘要")
	}
	o := Options{Lease: task.RecoveryOptions{Lease: time.Second, Heartbeat: 50 * time.Millisecond, Timeout: time.Second}, PollInterval: 50 * time.Millisecond}
	if o.Validate() != nil {
		t.Fatal(o)
	}
	for _, alter := range []func(*Options){func(o *Options) { o.Lease.Lease = 0 }, func(o *Options) { o.PollInterval = 0 }, func(o *Options) { o.PollInterval = 6 * time.Second }} {
		q := o
		alter(&q)
		if q.Validate() == nil {
			t.Fatal(q)
		}
	}
}

func TestTerminalReceiptPolicyBindingAndFailClosed(t *testing.T) {
	for _, state := range []string{agents.Failed, agents.Cancelled} {
		t.Run(state, func(t *testing.T) {
			p, v, r := fixture()
			r.State, r.ResultDigest = state, ""
			input := p.InputDigest(v)
			payload, policy, err := TerminalReceiptPolicy(p, input, r)
			if err != nil || !policy.Valid() || task.CheckStageJSON(payload) != nil {
				t.Fatal(err)
			}
			var fields map[string]string
			if json.Unmarshal(payload, &fields) != nil || len(fields) != 4 || fields["State"] != state || fields["RequestID"] != p.RequestID() || fields["RemoteID"] != r.RemoteID || fields["ProfileDigest"] != r.ProfileDigest {
				t.Fatal("失败回执字段越界或身份未绑定")
			}
			in := task.StageValidationInput{Stage: "agent_call", InputDigest: input, Payload: string(payload)}
			if policy.Check(context.Background(), in) != nil {
				t.Fatal("有效终态回执被拒绝")
			}
			for _, part := range []string{"stage", "input", "payload", "context"} {
				changed := in
				ctx, cancel := context.WithCancel(context.Background())
				switch part {
				case "stage":
					changed.Stage = "judge"
				case "input":
					changed.InputDigest = strings.Repeat("f", 64)
				case "payload":
					changed.Payload = `{}`
				case "context":
					cancel()
				}
				err := policy.Check(ctx, changed)
				cancel()
				if err == nil {
					t.Fatal("终态回执校验未关闭", part)
				}
			}
			_, again, _ := TerminalReceiptPolicy(p, input, r)
			if again.Digest != policy.Digest {
				t.Fatal("终态回执策略不稳定")
			}
			if _, _, err := ReceiptPolicy(p, input, r); err == nil {
				t.Fatal("失败回执冒充已隔离研究结果")
			}
		})
	}
	for _, field := range []string{"record", "request", "symbol", "actor", "profile", "input", "running", "completed", "quarantined", "unknown"} {
		p, v, r := fixture()
		r.State, r.ResultDigest = agents.Failed, ""
		input := p.InputDigest(v)
		switch field {
		case "record":
			r.ID = 0
		case "request":
			r.Request.RequestID = "test_other_request"
		case "symbol":
			r.Request.Symbol = "600002"
		case "actor":
			r.ActorID++
		case "profile":
			r.ProfileDigest = strings.Repeat("d", 64)
		case "input":
			input = "bad"
		case "running":
			r.State = agents.Running
		case "completed":
			r.State = agents.Completed
		case "quarantined":
			r.State, r.ResultDigest = agents.Quarantined, strings.Repeat("c", 64)
		case "unknown":
			r.State, r.RemoteID, r.ErrorCode = agents.SubmissionUnknown, "", "submission_unknown"
		}
		if payload, policy, err := TerminalReceiptPolicy(p, input, r); err == nil || len(payload) != 0 || policy.Valid() {
			t.Fatal("非确认失败或错绑回执被接受", field, err)
		}
	}
}
