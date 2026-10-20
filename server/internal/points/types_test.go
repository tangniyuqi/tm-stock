package points

import (
	"math"
	"testing"
	"time"
)

func TestRequestValidationAndIntegerBounds(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	good := GrantRequest{UserID: 1, Key: "test_grant_0000001", BizID: "grant.1", Amount: 10, Source: Gift, EffectiveAt: now}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*GrantRequest){func(r *GrantRequest) { r.UserID = 0 }, func(r *GrantRequest) { r.Key = "short" }, func(r *GrantRequest) { r.BizID = "含空格 " }, func(r *GrantRequest) { r.Amount = -1 }, func(r *GrantRequest) { r.Source = 5 }, func(r *GrantRequest) { r.EffectiveAt = now.Add(time.Nanosecond) }, func(r *GrantRequest) { r.ExpiresAt = &now }, func(r *GrantRequest) { r.Source = Purchase; n := now.Add(time.Hour); r.ExpiresAt = &n }} {
		r := good
		change(&r)
		if r.Validate() == nil {
			t.Fatal("非法入账请求通过")
		}
	}
	freeze := FreezeRequest{UserID: 1, Key: "test_freeze_000001", BizType: "ai_task", BizID: "task-1", PriceVersion: "v1", ExpiresAt: now}
	if err := freeze.Validate(); err != nil {
		t.Fatal(err)
	}
	freeze.BizType = "membership"
	if freeze.Validate() == nil {
		t.Fatal("会员兑换不应放行")
	}
	if (FinalizeRequest{UserID: 1, Key: "test_final_0000001", HoldID: 1, State: Holding}).Validate() == nil {
		t.Fatal("非终态不应通过")
	}
	for _, state := range []int{Settled, Released, Expired} {
		if err := (FinalizeRequest{UserID: 1, Key: "test_final_0000001", HoldID: 1, State: state}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, price := range []Price{{Points: 1}, {Version: "v1", Points: 0}, {Version: "含空格 ", Points: 1}} {
		if price.Validate() == nil {
			t.Fatal("无效报价未拒绝")
		}
	}
	if err := (Price{Version: "v1", Points: math.MaxInt64}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]int64{{math.MaxInt64, 1}, {-1, 1}, {1, -1}} {
		if _, err := AddNonnegative(pair[0], pair[1]); err == nil {
			t.Fatal("溢出/负值未拒绝")
		}
	}
	if n, err := AddNonnegative(math.MaxInt64-1, 1); err != nil || n != math.MaxInt64 {
		t.Fatal("合法边界未通过")
	}
	a, err := RequestHash("grant", good)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := RequestHash("freeze", good)
	if a == b || len(a) != 64 {
		t.Fatal("摘要未隔离操作域")
	}
	if _, err := RequestHash("grant", make(chan int)); err == nil {
		t.Fatal("摘要编码错误未传播")
	}
	reverse := ReverseRequest{UserID: 1, GrantTxnID: 1, Key: "test_reverse_0001", BizID: "refund:1"}
	if err := reverse.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ReverseRequest){func(r *ReverseRequest) { r.UserID = 0 }, func(r *ReverseRequest) { r.GrantTxnID = 0 }, func(r *ReverseRequest) { r.Key = "short" }, func(r *ReverseRequest) { r.BizID = "有空格 " }} {
		r := reverse
		change(&r)
		if r.Validate() == nil {
			t.Fatal("非法冲正请求放行")
		}
	}
}
