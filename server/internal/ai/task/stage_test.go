package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestStageJSONIsolationAndBounds(t *testing.T) {
	for _, s := range []string{`{}`, `{"a":[1,true,null,{"b":"值"}]}`, `{"n":9007199254740993}`, "{} \n", `{"x":"` + strings.Repeat("a", MaxStageResultBytes-8) + `"}`} {
		if err := CheckStageJSON([]byte(s)); err != nil {
			t.Fatal(len(s), err)
		}
	}
	for _, s := range []string{"", `[]`, `null`, `{} {}`, `{"a":1,"a":2}`, `{"a":1,"A":2}`, `{"a":1,"\u0061":2}`, `{"a":`, `{"a":[1,}`, `{"a":{}`, `{"a":[1}`, "{\"a\":\"" + string([]byte{255}) + "\"}", strings.Repeat(`{"a":`, 33) + `0` + strings.Repeat(`}`, 33), `{"a":` + strings.Repeat(`[`, 33) + `0` + strings.Repeat(`]`, 33) + `}`, `{"x":"` + strings.Repeat("a", MaxStageResultBytes) + `"}`} {
		if err := CheckStageJSON([]byte(s)); err == nil {
			t.Fatal("不合格隔离 JSON 放行", len(s))
		}
	}
	if StageDigest([]byte("a")) == StageDigest([]byte("b")) {
		t.Fatal("摘要冲突")
	}
}

func TestStageRequestsPoliciesAndRedaction(t *testing.T) {
	r := StageRequest{UserID: 1, TaskID: 2, Fence: 1, Owner: "worker_stage_00001", OperationKey: "stage_operation_00001", Stage: "extract", InputDigest: strings.Repeat("a", 64), StepNo: 1}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*StageRequest){func(r *StageRequest) { r.UserID = 0 }, func(r *StageRequest) { r.TaskID = 0 }, func(r *StageRequest) { r.Fence = 0 }, func(r *StageRequest) { r.Owner = "bad" }, func(r *StageRequest) { r.OperationKey = "bad" }, func(r *StageRequest) { r.Stage = "unknown" }, func(r *StageRequest) { r.InputDigest = "bad" }, func(r *StageRequest) { r.StepNo = 0 }, func(r *StageRequest) { r.StepNo = 65 }} {
		c := r
		change(&c)
		if err := c.Validate(); err == nil {
			t.Fatal(c)
		}
	}
	for _, stage := range []string{"collect", "extract", "judge", "agent_call"} {
		if !ValidStage(stage) {
			t.Fatal(stage)
		}
	}
	if StageUnitsMatch("collect", Units{Calls: 1, Tokens: 1}) || !StageUnitsMatch("collect", Units{Pages: 1}) || StageUnitsMatch("extract", Units{Pages: 1}) || !StageUnitsMatch("judge", Units{Calls: 1, Tokens: 1}) {
		t.Fatal("阶段预算类型错误")
	}
	p := StagePolicy{Digest: strings.Repeat("a", 64), Check: func(context.Context, StageValidationInput) error { return nil }}
	if !p.Valid() || (StagePolicy{}).Valid() {
		t.Fatal("策略验证错误")
	}
	res := Reservation{Seq: 1, Fence: 1, Owner: r.Owner}
	body := []byte(`{"private":"隔离正文"}`)
	v := StageResult{TaskID: 2, UserID: 1, Fence: 1, ReservationSeq: 1, StepNo: 1, Stage: r.Stage, Owner: r.Owner, InputDigest: r.InputDigest, OutputDigest: StageDigest(body), Payload: body, CreatedAt: time.Now()}
	if err := CheckStageResult(r, res, v); err != nil {
		t.Fatal(err)
	}
	v.OutputDigest = strings.Repeat("b", 64)
	if err := CheckStageResult(r, res, v); err == nil {
		t.Fatal("摘要损坏放行")
	}
	for _, s := range []string{fmt.Sprint(v), fmt.Sprintf("%#v", v)} {
		if strings.Contains(s, "隔离正文") {
			t.Fatal("正文泄漏")
		}
	}
	b, err := json.Marshal(v)
	if err != nil || strings.Contains(string(b), "隔离正文") || strings.Contains(string(b), "Payload") {
		t.Fatal("JSON 泄漏正文", err)
	}
	if receipt := v.Receipt(true); !receipt.NewlyCaptured || receipt.StepNo != 1 || receipt.TaskID != 2 {
		t.Fatal(receipt)
	}
}
