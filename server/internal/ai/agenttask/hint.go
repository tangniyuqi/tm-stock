package agenttask

import (
	"bytes"
	"encoding/json"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// Hint 是不可信研究线索，不是可发布证据。过大结果整份拒绝，不静默截断。
type Hint struct {
	plan    Plan
	record  agents.Record
	payload json.RawMessage
}

const MaxHintBytes = 256 << 10

func NewHint(v task.Task, p Plan, r agents.Record, result agents.Result) (*Hint, error) {
	if p.ValidateScope(v) != nil || len(result.Payload) > MaxHintBytes || task.CheckStageJSON(result.Payload) != nil || !result.Valid() || result.RemoteID != r.RemoteID || result.Digest() != r.ResultDigest {
		return nil, points.ErrInvalid
	}
	if _, _, err := ReceiptPolicy(p, p.InputDigest(v), r); err != nil {
		return nil, err
	}
	return &Hint{p, r, bytes.Clone(result.Payload)}, nil
}
func (h *Hint) Matches(v task.Task) bool {
	return h != nil && h.plan.ValidateScope(v) == nil
}
func (h *Hint) Plan() Plan {
	if h == nil {
		return Plan{}
	}
	return h.plan
}
func (h *Hint) Record() agents.Record {
	if h == nil {
		return agents.Record{}
	}
	return h.record
}
func (h *Hint) Payload() json.RawMessage {
	if h == nil {
		return nil
	}
	return bytes.Clone(h.payload)
}
func (h *Hint) String() string   { return "不可信 Agent 线索(原文已隐藏)" }
func (h *Hint) GoString() string { return h.String() }
