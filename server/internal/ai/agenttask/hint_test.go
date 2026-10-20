package agenttask

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

func TestHintIdentityAndIsolation(t *testing.T) {
	p, v, r := fixture()
	v.CurrentStep = 4
	result := agents.Result{RemoteID: r.RemoteID, Payload: json.RawMessage(`{"analysis_id":"test_remote_id","reports":{"news":"test-only-private-hint"}}`)}
	r.ResultDigest = result.Digest()
	h, err := NewHint(v, p, r, result)
	if err != nil || !h.Matches(v) || p.Validate(v) == nil || p.ValidateScope(v) != nil {
		t.Fatal(h, err)
	}
	result.Payload[0] = 'x'
	body := h.Payload()
	body[0] = 'x'
	if !json.Valid(h.Payload()) || h.Plan() != p || h.Record() != r || strings.Contains(fmt.Sprintf("%+v %#v", h, h), "test-only-private-hint") {
		t.Fatal("副本/隐藏失败")
	}
	v.UserID++
	if h.Matches(v) {
		t.Fatal("跨会员复用")
	}
	var missing *Hint
	if missing.Matches(v) || missing.Plan() != (Plan{}) || missing.Record() != (agents.Record{}) || missing.Payload() != nil {
		t.Fatal("空线索放行")
	}
}
func TestHintRejectsUnboundOrAmbiguousResults(t *testing.T) {
	for _, name := range []string{"actor", "remote", "digest", "state", "duplicate", "size", "task", "malformed"} {
		t.Run(name, func(t *testing.T) {
			p, v, r := fixture()
			result := agents.Result{RemoteID: r.RemoteID, Payload: json.RawMessage(`{"analysis_id":"test_remote_id","reports":{"news":"test-only"}}`)}
			switch name {
			case "actor":
				r.ActorID++
			case "remote":
				result.RemoteID = "different"
			case "state":
				r.State = agents.Completed
			case "duplicate":
				result.Payload = json.RawMessage(`{"analysis_id":"test_remote_id","reports":{"news":"a","news":"b"}}`)
			case "size":
				result.Payload = json.RawMessage(`{"analysis_id":"test_remote_id","reports":{"news":"` + strings.Repeat("x", MaxHintBytes) + `"}}`)
			case "task":
				v.No = strings.Repeat("d", 32)
			case "malformed":
				result.Payload = []byte("bad")
			}
			r.ResultDigest = result.Digest()
			if name == "digest" {
				r.ResultDigest = strings.Repeat("b", 64)
			}
			if h, err := NewHint(v, p, r, result); err == nil || h != nil {
				t.Fatal("未绑定结果进入选择器", err)
			}
		})
	}
}
