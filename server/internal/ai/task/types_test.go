package task

import (
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestTaskRequestBudgetAndWhitelist(t *testing.T) {
	good := CreateRequest{UserID: 1, Key: "test_task_request_001", AnalysisType: 1, SubjectType: Theme, SubjectID: 2, PriceVersion: "v1"}
	for _, change := range []func(*CreateRequest){
		func(r *CreateRequest) { r.UserID = 0 }, func(r *CreateRequest) { r.Key = "short" }, func(r *CreateRequest) { r.AnalysisType = 0 }, func(r *CreateRequest) { r.AnalysisType = 3 }, func(r *CreateRequest) { r.SubjectID = 0 }, func(r *CreateRequest) { r.SubjectType = "stock" }, func(r *CreateRequest) { r.PriceVersion = "" }, func(r *CreateRequest) { r.PriceVersion = "中文" }, func(r *CreateRequest) { r.PriceVersion = strings.Repeat("v", 33) },
	} {
		bad := good
		change(&bad)
		if err := bad.Validate(); !errors.Is(err, points.ErrInvalid) {
			t.Fatal("无效任务请求放行", bad, err)
		}
	}
	for _, analysis := range []int{1, 2} {
		r := good
		r.AnalysisType = analysis
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	good.SubjectType = Topic
	if err := good.Validate(); err != nil {
		t.Fatal("专题应被允许", err)
	}
	b := Budget{MaxAttempts: 2, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 1000}
	if err := b.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Budget){func(b *Budget) { b.MaxAttempts = 0 }, func(b *Budget) { b.MaxAttempts = 101 }, func(b *Budget) { b.MaxCalls = 0 }, func(b *Budget) { b.MaxTokens = 0 }, func(b *Budget) { b.MaxPages = 0 }, func(b *Budget) { b.MaxDurationMS = 999 }, func(b *Budget) { b.MaxDurationMS = 86400001 }} {
		bad := b
		change(&bad)
		if err := bad.Validate(); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	for _, owner := range []string{"", "short", strings.Repeat("a", 65), "worker_中文_000001"} {
		if ValidOwner(owner) {
			t.Fatal("非法租约身份放行", owner)
		}
	}
	if !ValidOwner(strings.Repeat("a", 64)) {
		t.Fatal("上界身份拒绝")
	}
	for _, code := range []string{"provider_unavailable", "invalid_output", "guard_rejected", "budget_exhausted", "attempts_exhausted", "internal_failure"} {
		if !ValidFailure(code) {
			t.Fatal(code)
		}
	}
	for _, code := range []string{"", "cancelled", "expired", "未知错误原文", "success"} {
		if ValidFailure(code) {
			t.Fatal("非失败白名单放行", code)
		}
	}
}

func TestTaskStableReceiptAndTerminalStates(t *testing.T) {
	v := Task{ID: 1, UserID: 2, HoldID: 3, PricePoints: 7, No: strings.Repeat("a", 32), PriceVersion: "v1"}
	before := v.Created()
	for state := 0; state < 8; state++ {
		v.Status = state
		v.Fence++
		v.EventSeq++
		if v.Terminal() != (state >= 3 && state <= 6) {
			t.Fatal(state)
		}
		if v.Created() != before {
			t.Fatal("状态改变稳定创建收据")
		}
	}
}
