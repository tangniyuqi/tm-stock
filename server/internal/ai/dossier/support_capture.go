package dossier

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type supportPacket struct {
	InputDigest string            `json:"inputDigest"`
	Response    provider.Response `json:"response"`
}

// SupportResult只标记原计划中的字段，不接受模型提供的原文/字段名或自由解释。
type SupportResult struct {
	ID                           int
	Card, Field, Verdict, Reason string
}
type SupportReview struct {
	inputDigest, draftDigest string
	results                  []SupportResult
}

func (SupportReview) String() string     { return "专题字段核验记录（内部定位已隐藏）" }
func (r SupportReview) GoString() string { return r.String() }
func (SupportResult) String() string     { return "专题字段核验项（内部定位已隐藏）" }
func (r SupportResult) GoString() string { return r.String() }
func (r *SupportReview) Results() []SupportResult {
	if r == nil {
		return nil
	}
	return slices.Clone(r.results)
}
func (r *SupportReview) InputDigest() string {
	if r == nil {
		return ""
	}
	return r.inputDigest
}
func (r *SupportReview) DraftDigest() string {
	if r == nil {
		return ""
	}
	return r.draftDigest
}

// CapturePayload只保留受限回执；原始HTTP仍须进入既有调用审计，不在此补造用量。
// 结构捕获不代表响应完整或语义结果有效，Review/阶段策略会另行拒绝无效回执。
func (p *SupportPlan) CapturePayload(response provider.Response) ([]byte, error) {
	if !p.Valid() || len(response.Text) > 64<<10 || len(response.Model) > 128 || len(response.RequestID) > 256 ||
		!utf8.ValidString(response.Text) || !utf8.ValidString(response.Model) || !utf8.ValidString(response.RequestID) {
		return nil, ErrSupportReview
	}
	response.Raw = nil
	raw, err := json.Marshal(supportPacket{p.digest, response})
	if err != nil || task.CheckStageJSON(raw) != nil {
		return nil, ErrSupportReview
	}
	return raw, nil
}

func (p *SupportPlan) packet(raw []byte) (supportPacket, error) {
	var packet supportPacket
	if !p.Valid() || task.CheckStageJSON(raw) != nil {
		return packet, ErrSupportReview
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&packet) != nil || packet.InputDigest != p.digest || len(packet.Response.Raw) != 0 {
		return supportPacket{}, ErrSupportReview
	}
	canonical, err := p.CapturePayload(packet.Response)
	if err != nil || !bytes.Equal(canonical, raw) {
		return supportPacket{}, ErrSupportReview
	}
	return packet, nil
}

func supportResponseValid(r provider.Response, c SupportConfig) bool {
	u := r.Usage
	return r.Model == c.Model && r.FinishReason == "stop" && !r.UsageEstimated && supportMeta(r.RequestID, 256) &&
		u.Prompt > 0 && int64(u.Prompt) <= c.MaxInputTokens && u.Completion > 0 && u.Completion <= c.MaxOutputTokens &&
		int64(u.Total) == int64(u.Prompt)+int64(u.Completion) &&
		u.Cached >= 0 && u.Cached <= u.Prompt && u.Reasoning >= 0 && u.Reasoning <= u.Completion
}

func supportReason(field, verdict, reason string) bool {
	switch verdict {
	case "supported":
		return reason == "direct_support"
	case "partial":
		return reason == "partial_support"
	case "uncertain":
		return reason == "insufficient_context"
	case "unsupported":
		if reason == "not_supported" {
			return true
		}
		switch field {
		case "section":
			return reason == "section_mismatch"
		case "subject":
			return reason == "subject_mismatch"
		case "publishedDate", "effectiveDate", "period":
			return reason == "time_mismatch"
		case "scope":
			return reason == "scope_mismatch"
		case "stage":
			return reason == "stage_mismatch"
		case "basis":
			return reason == "basis_mismatch"
		case "unit":
			return reason == "unit_mismatch"
		}
	}
	return false
}
func supportKeys(raw json.RawMessage, keys ...string) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || len(obj) != len(keys) {
		return false
	}
	for _, key := range keys {
		if value, ok := obj[key]; !ok || string(value) == "null" {
			return false
		}
	}
	return true
}

// Review只回放原计划已捕获的响应，不访问模型或恢复已撤回的资料读取权。
func (p *SupportPlan) Review(ctx context.Context, raw []byte) (*SupportReview, error) {
	if ctx == nil {
		return nil, ErrSupportReview
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	packet, err := p.packet(raw)
	if err != nil || !supportResponseValid(packet.Response, p.cfg) {
		return nil, ErrSupportReview
	}
	text := []byte(packet.Response.Text)
	if task.CheckStageJSON(text) != nil || !supportKeys(text, "results") {
		return nil, ErrSupportReview
	}
	var doc struct {
		Results []json.RawMessage `json:"results"`
	}
	if json.Unmarshal(text, &doc) != nil || len(doc.Results) != len(p.checks) {
		return nil, ErrSupportReview
	}
	rows := make([]SupportResult, len(p.checks))
	seen := make([]bool, len(p.checks))
	for _, item := range doc.Results {
		var row struct {
			ID      int    `json:"id"`
			Verdict string `json:"verdict"`
			Reason  string `json:"reason"`
		}
		if !supportKeys(item, "id", "verdict", "reason") || json.Unmarshal(item, &row) != nil || row.ID < 1 || row.ID > len(p.checks) || seen[row.ID-1] {
			return nil, ErrSupportReview
		}
		check := p.checks[row.ID-1]
		if !supportReason(check.Field, row.Verdict, row.Reason) {
			return nil, ErrSupportReview
		}
		seen[row.ID-1] = true
		rows[row.ID-1] = SupportResult{ID: row.ID, Card: check.Card, Field: check.Field, Verdict: row.Verdict, Reason: row.Reason}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return &SupportReview{inputDigest: p.digest, draftDigest: p.draftDigest, results: rows}, nil
}

// 原阶段服务负责原任务/租约/预算/审计。合法否定结果可存证，但不能作为发布通过。
func (p *SupportPlan) StagePolicy() task.StagePolicy {
	if !p.Valid() {
		return task.StagePolicy{}
	}
	return task.StagePolicy{Digest: task.StageDigest([]byte(SupportVersion + ":" + p.policy + ":" + p.digest)), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if ctx == nil {
			return ErrSupportReview
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if in.Stage != "judge" || in.InputDigest != p.digest {
			return task.ErrStagePolicy
		}
		_, err := p.Review(ctx, []byte(in.Payload))
		return err
	}}
}
