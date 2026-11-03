package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type reportInputsFixture struct {
	binding AIReportInputBinding
	record  agents.Record
	result  agents.Result
	calls   int
	hook    func()
}

func (f *reportInputsFixture) ForInputs(_ context.Context, uid, id int64) (AIReportInputBinding, error) {
	if uid != f.binding.Task.UserID || id != f.binding.Task.ID {
		return AIReportInputBinding{}, points.ErrNotFound
	}
	return f.binding, nil
}
func (f *reportInputsFixture) Quarantined(_ context.Context, actor int64, key string) (agents.Record, agents.Result, error) {
	f.calls++
	if f.hook != nil {
		f.hook()
	}
	if actor != f.record.ActorID || key != f.record.Request.RequestID {
		return agents.Record{}, agents.Result{}, points.ErrNotFound
	}
	return f.record, f.result, nil
}

func TestAIReportInputsCurrentAuthorizationAndProjection(t *testing.T) {
	for _, mode := range []string{"valid", "disabled", "unbound", "wrong_user", "wrong_task", "binding", "actor", "receipt", "raw_digest", "legacy", "revoked_after_read"} {
		t.Run(mode, func(t *testing.T) {
			s, st, ctl := reportReadFixture(t)
			p := agenttask.Plan{UserID: st.v.UserID, TaskID: st.v.ID, TaskNo: st.v.No, SubjectType: st.v.SubjectType, SubjectID: st.v.SubjectID, ActorID: 7, Symbol: "600001", ProfileDigest: strings.Repeat("b", 64), Units: task.Units{Calls: 1, Tokens: 100}}
			// 仅协议替身，沿原任务键生成原请求，单位使用合法的独立调用上限。
			p.Units.Calls = 1
			p.Units.Tokens = 100
			p.Units.Pages = 0
			p.OperationKey = p.RequestID()
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "eval", "agents", "inputs-synthetic-protocol.json"))
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]any{"analysis_id": "test_remote_id", "reports": map[string]string{"news_report": "仅测试线索"}, "evaluation_scope": "synthetic_fixture_only", "input_audit": json.RawMessage(raw)})
			if err != nil {
				t.Fatal(err)
			}
			result := agents.Result{RemoteID: "test_remote_id", Payload: payload}
			r := agents.Record{ID: 9, ActorID: 7, Request: agents.Request{RequestID: p.RequestID(), Symbol: p.Symbol}, ProfileDigest: p.ProfileDigest, RemoteID: result.RemoteID, State: agents.Quarantined, ResultDigest: result.Digest()}
			f := &reportInputsFixture{binding: AIReportInputBinding{Task: st.v, Agent: p, Receipt: agenttask.Receipt{RequestID: r.Request.RequestID, RemoteID: r.RemoteID, ProfileDigest: r.ProfileDigest, ResultDigest: r.ResultDigest}}, record: r, result: result}
			if err := s.SetInputEvidence(f, f); err != nil {
				t.Fatal(err)
			}
			uid, id := st.v.UserID, st.v.ID
			switch mode {
			case "disabled":
				s.enabled = false
			case "unbound":
				s.inputSource = nil
			case "wrong_user":
				uid++
			case "wrong_task":
				id++
			case "binding":
				f.binding.Task.UserID++
			case "actor":
				f.record.ActorID++
			case "receipt":
				f.binding.Receipt.ResultDigest = strings.Repeat("c", 64)
			case "raw_digest":
				f.result.Payload = append(f.result.Payload, ' ')
			case "legacy":
				f.result.Payload = []byte(`{"analysis_id":"test_remote_id","reports":{"news_report":"仅测试线索"}}`)
				f.record.ResultDigest = f.result.Digest()
				f.binding.Receipt.ResultDigest = f.record.ResultDigest
			case "revoked_after_read":
				f.hook = func() { ctl.err = finalreport.ErrUnavailable }
			}
			out, err := s.ReadInputs(context.Background(), uid, id)
			if (err == nil) != (mode == "valid") {
				t.Fatal("原任务当前授权/隔离输入边界不符", mode, err)
			}
			if err == nil {
				b, e := json.Marshal(out)
				if e != nil || len(out.Calls) != 14 || len(out.Facts) != 1 || out.TaskID != id || strings.Contains(string(b), "隔离提示私有标记") || strings.Contains(string(b), "request_json") || strings.Contains(string(b), "call_id") || out.Calls[0].CostMicros != nil {
					t.Fatal("会员投影泄漏内部输入或补造费用")
				}
			}
			if (mode == "disabled" || mode == "unbound" || mode == "wrong_user" || mode == "wrong_task") && f.calls != 0 {
				t.Fatal("授权前读取隔离正文")
			}
		})
	}
	var missing *AIReportReadService
	if err := missing.SetInputEvidence(nil, nil); err == nil {
		t.Fatal("空依赖被装配")
	}
	if _, err := missing.ReadInputs(context.Background(), 1, 1); err == nil {
		t.Fatal("空服务放行")
	}
}
