package agents

import (
	"strings"
	"testing"
)

func initialRecord() Record {
	return Record{ID: 1, ActorID: 7, Request: Request{RequestID: "test_record_01", Symbol: "600001"}, ProfileDigest: strings.Repeat("a", 64), State: Submitting}
}

func TestRecordAndTransitionContract(t *testing.T) {
	old := initialRecord()
	if !old.Valid() {
		t.Fatal("有效记录拒绝")
	}
	accepted := old
	accepted.State = Accepted
	accepted.RemoteID = "remote-1"
	if !ValidChange(old, accepted, "submitted", nil) {
		t.Fatal("提交转换拒绝")
	}
	unknown := old
	unknown.State = SubmissionUnknown
	unknown.ErrorCode = "submission_unknown"
	if !ValidChange(old, unknown, "submission_unknown", nil) {
		t.Fatal("未知转换拒绝")
	}
	running := accepted
	running.State = Running
	if !ValidChange(accepted, running, "observed", nil) || ValidChange(running, accepted, "observed", nil) {
		t.Fatal("运行状态倒退")
	}
	completed := running
	completed.State = Completed
	if !ValidChange(running, completed, "observed", nil) || ValidChange(completed, running, "observed", nil) {
		t.Fatal("终态倒退")
	}
	cancelling := running
	cancelling.CancelRequested = true
	cancelling.ErrorCode = "cancel_pending"
	if !ValidChange(running, cancelling, "cancel_requested", nil) || ValidChange(cancelling, cancelling, "cancel_requested", nil) {
		t.Fatal("取消重复")
	}
	ack := cancelling
	ack.ErrorCode = ""
	if !ValidChange(cancelling, ack, "cancel_acknowledged", nil) {
		t.Fatal("回执拒绝")
	}
	ack.ErrorCode = "cancel_unknown"
	if !ValidChange(cancelling, ack, "cancel_unknown", nil) {
		t.Fatal("未知回执拒绝")
	}
	result := Result{RemoteID: "remote-1", Payload: []byte(`{"analysis_id":"remote-1","reports":{"news":"仅测试"}}`)}
	quarantined := completed
	quarantined.State = Quarantined
	quarantined.ResultDigest = result.Digest()
	if !result.Valid() || !ValidChange(completed, quarantined, "quarantined", &result) {
		t.Fatal("隔离转换拒绝")
	}
	for _, change := range []func(*Record){func(r *Record) { r.ID = 0 }, func(r *Record) { r.ActorID = 0 }, func(r *Record) { r.Request.Symbol = "bad" }, func(r *Record) { r.ProfileDigest = "bad" }, func(r *Record) { r.State = "success" }, func(r *Record) { r.RemoteID = "../x" }, func(r *Record) { r.ResultDigest = "" }, func(r *Record) { r.ErrorCode = "原文错误" }} {
		r := quarantined
		change(&r)
		if r.Valid() {
			t.Fatal("损坏记录放行", r)
		}
	}
	for _, change := range []func(*Record){func(r *Record) { r.ActorID++ }, func(r *Record) { r.ID++ }, func(r *Record) { r.Request.Symbol = "600002" }, func(r *Record) { r.ProfileDigest = strings.Repeat("b", 64) }, func(r *Record) { r.RemoteID = "other" }, func(r *Record) { r.ErrorCode = "cancel_unknown" }} {
		r := completed
		change(&r)
		if ValidChange(running, r, "observed", nil) {
			t.Fatal("身份或附属字段变更放行")
		}
	}
	if ValidChange(completed, quarantined, "quarantined", nil) || ValidChange(completed, quarantined, "other", &result) || ValidChange(running, running, "observed", &result) {
		t.Fatal("非法事件载荷放行")
	}
	if Terminal(Running) || !Terminal(Quarantined) || ValidRemoteState(Submitting) {
		t.Fatal("状态分类错误")
	}
	for _, payload := range []string{`null`, `[]`, `{}`, `{"analysis_id":"other","reports":{"news":"x"}}`, `{"analysis_id":"remote-1","reports":{}}`, string([]byte{0xff})} {
		r := result
		r.Payload = []byte(payload)
		if r.Valid() {
			t.Fatal("无效隔离正文放行")
		}
	}
	if (Profile{Digest: strings.Repeat("a", 64), MaxSubmissions: 0}).Valid() || (Profile{Digest: "bad", MaxSubmissions: 1}).Valid() {
		t.Fatal("配置错误放行")
	}
}
