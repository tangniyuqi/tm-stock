package feedback

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFeedbackClosedStateAndHistory(t *testing.T) {
	now := time.UnixMilli(1792480000000).UTC()
	r := Record{ID: 1, TaskID: 9, UserID: 7, Category: "fact", ReportDigest: strings.Repeat("a", 64), Status: "received", Version: 1, CreatedAt: now, UpdatedAt: now}
	es := []Event{{FeedbackID: 1, Version: 1, Status: "received", At: now}}
	if ValidateHistory(r, es) != nil {
		t.Fatal("初始历史拒绝")
	}
	for _, status := range []string{"processing", "resolved"} {
		r.Status = status
		r.Version++
		if status == "resolved" {
			r.Resolution = "reviewed"
		}
		es = append(es, Event{FeedbackID: 1, Version: r.Version, Status: status, Resolution: r.Resolution, ActorID: 8, At: now})
		if ValidateHistory(r, es) != nil {
			t.Fatal("处理历史拒绝")
		}
	}
	for _, change := range []func(*Record){func(r *Record) { r.ID = MaxID + 1 }, func(r *Record) { r.Category = "unknown" }, func(r *Record) { r.ReportDigest = "bad" }, func(r *Record) { r.UpdatedAt = now.Add(-time.Millisecond) }, func(r *Record) { r.Status = "received" }, func(r *Record) { r.Resolution = "unknown" }} {
		bad := r
		change(&bad)
		if ValidateHistory(bad, es) == nil {
			t.Fatal("非法记录放行")
		}
	}
	for _, change := range []func([]Event){func(es []Event) { es[0].ActorID = 1 }, func(es []Event) { es[1].Version = 3 }, func(es []Event) { es[2].At = now.Add(-time.Millisecond) }, func(es []Event) { es[2].FeedbackID = 2 }, func(es []Event) { es[2].Resolution = "referred" }} {
		bad := append([]Event(nil), es...)
		change(bad)
		if ValidateHistory(r, bad) == nil {
			t.Fatal("非法历史放行")
		}
	}
	if ValidateHistory(r, es[:2]) == nil || Transition("resolved", "processing") || Transition("received", "resolved") {
		t.Fatal("历史缺失或越级放行")
	}
	for _, d := range []Decision{{1, "processing", ""}, {2, "resolved", "reviewed"}, {2, "resolved", "referred"}, {2, "dismissed", "unconfirmed"}, {2, "dismissed", "not_applicable"}} {
		if !d.Valid() {
			t.Fatal("合法决定拒绝")
		}
	}
	for _, d := range []Decision{{0, "processing", ""}, {1, "resolved", "reviewed"}, {3, "resolved", "reviewed"}, {2, "dismissed", "reviewed"}} {
		if d.Valid() {
			t.Fatal("非法决定放行")
		}
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", r, r), r.ReportDigest) {
		t.Fatal("内部追溯信息进入格式化日志")
	}
}
