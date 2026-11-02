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

type cardComparisonPacket struct {
	Protocol    string            `json:"protocol"`
	InputDigest string            `json:"inputDigest"`
	Response    provider.Response `json:"response"`
}
type CardComparisonAxis struct {
	ID                                   int
	Field, Left, Right, Relation, Reason string
}
type CardComparisonReview struct {
	inputDigest, draftDigest, payloadDigest, origins, outcome, wording string
	axes                                                               []CardComparisonAxis
	resolved                                                           int
}

func (CardComparisonAxis) String() string     { return "两卡口径项（内部定位已隐藏）" }
func (r CardComparisonAxis) GoString() string { return r.String() }
func (CardComparisonReview) String() string {
	return "两卡口径回执（身份与原文已隐藏）"
}
func (r CardComparisonReview) GoString() string { return r.String() }
func (r *CardComparisonReview) InputDigest() string {
	if r == nil {
		return ""
	}
	return r.inputDigest
}
func (r *CardComparisonReview) DraftDigest() string {
	if r == nil {
		return ""
	}
	return r.draftDigest
}
func (r *CardComparisonReview) PayloadDigest() string {
	if r == nil {
		return ""
	}
	return r.payloadDigest
}
func (r *CardComparisonReview) Outcome() string {
	if r == nil {
		return ""
	}
	return r.outcome
}
func (r *CardComparisonReview) Wording() string {
	if r == nil {
		return ""
	}
	return r.wording
}

// 只描述两张卡原句的出处关系，不代表全部字段来源链或独立性批准。
func (r *CardComparisonReview) StatementOriginRelation() string {
	if r == nil {
		return ""
	}
	return r.origins
}
func (r *CardComparisonReview) Axes() []CardComparisonAxis {
	if r == nil {
		return nil
	}
	return slices.Clone(r.axes)
}
func (r *CardComparisonReview) ResolvedCount() int {
	if r == nil {
		return 0
	}
	return r.resolved
}

func (p *CardComparisonPlan) CapturePayload(r provider.Response) ([]byte, error) {
	if !p.Valid() || len(r.Text) > 64<<10 || len(r.Model) > 128 || len(r.RequestID) > 256 || !utf8.ValidString(r.Text) || !utf8.ValidString(r.Model) || !utf8.ValidString(r.RequestID) {
		return nil, ErrCardComparison
	}
	r.Raw = nil
	raw, err := json.Marshal(cardComparisonPacket{CardComparisonVersion, p.digest, r})
	if err != nil || task.CheckStageJSON(raw) != nil {
		return nil, ErrCardComparison
	}
	return raw, nil
}

func comparisonSupport(value string, known bool) bool {
	if !known {
		return value == "unknown"
	}
	return value == "supported" || value == "partial" || value == "unsupported" || value == "uncertain"
}
func comparisonAxisReason(a comparisonAxisInput, left, right, relation string) (string, bool) {
	if !comparisonSupport(left, a.LeftCheckID > 0) || !comparisonSupport(right, a.RightCheckID > 0) {
		return "", false
	}
	missing := a.LeftCheckID == 0 || a.RightCheckID == 0
	if relation == "not_applicable" {
		return "not_required", !a.Required && a.LeftCheckID == 0 && a.RightCheckID == 0
	}
	if relation == "uncertain" {
		if missing {
			return "missing_field", true
		}
		if left != "supported" || right != "supported" {
			return "field_meaning_unverified", true
		}
		return "insufficient_context", true
	}
	if missing || left != "supported" || right != "supported" {
		return "", false
	}
	if relation == "aligned" {
		return "meaning_aligned", true
	}
	if relation == "different" {
		return a.Field + "_differs", true
	}
	return "", false
}

func comparisonOutcome(axes []CardComparisonAxis, wording string) (string, int, bool) {
	if wording != "same_meaning" && wording != "different_wording" && wording != "insufficient_context" {
		return "", 0, false
	}
	different, uncertain, resolved := false, false, 0
	for _, a := range axes {
		switch a.Relation {
		case "different":
			different = true
			resolved++
		case "aligned":
			resolved++
		case "uncertain":
			uncertain = true
		}
	}
	if different {
		if wording == "same_meaning" {
			return "", 0, false
		}
		return "not_comparable", resolved, true
	}
	if uncertain || wording == "insufficient_context" {
		return "insufficient_context", resolved, true
	}
	if wording == "different_wording" {
		return "wording_differs", resolved, true
	}
	return "scope_aligned", resolved, true
}

// 只回放已捕获包，不访问模型；未知字段或不适用不能由模型变成已核事实。
func (p *CardComparisonPlan) Review(ctx context.Context, raw []byte) (*CardComparisonReview, error) {
	if ctx == nil {
		return nil, ErrCardComparison
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !p.Valid() || task.CheckStageJSON(raw) != nil || !supportKeys(raw, "protocol", "inputDigest", "response") {
		return nil, ErrCardComparison
	}
	var packet cardComparisonPacket
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&packet) != nil || packet.Protocol != CardComparisonVersion || packet.InputDigest != p.digest || !supportResponseValid(packet.Response, p.cfg.Model) {
		return nil, ErrCardComparison
	}
	canonical, err := p.CapturePayload(packet.Response)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, ErrCardComparison
	}
	text := []byte(packet.Response.Text)
	if task.CheckStageJSON(text) != nil || !supportKeys(text, "axes", "wording") {
		return nil, ErrCardComparison
	}
	var doc struct {
		Axes    []json.RawMessage `json:"axes"`
		Wording string            `json:"wording"`
	}
	if json.Unmarshal(text, &doc) != nil || len(doc.Axes) != len(p.axes) {
		return nil, ErrCardComparison
	}
	rows := make([]CardComparisonAxis, len(p.axes))
	seen := make([]bool, len(p.axes))
	for _, item := range doc.Axes {
		var row struct {
			ID       int    `json:"id"`
			Left     string `json:"left"`
			Right    string `json:"right"`
			Relation string `json:"relation"`
		}
		if !supportKeys(item, "id", "left", "right", "relation") || json.Unmarshal(item, &row) != nil || row.ID < 1 || row.ID > len(p.axes) || seen[row.ID-1] {
			return nil, ErrCardComparison
		}
		a := p.axes[row.ID-1]
		reason, ok := comparisonAxisReason(a, row.Left, row.Right, row.Relation)
		if !ok {
			return nil, ErrCardComparison
		}
		rows[row.ID-1] = CardComparisonAxis{row.ID, a.Field, row.Left, row.Right, row.Relation, reason}
		seen[row.ID-1] = true
	}
	outcome, resolved, ok := comparisonOutcome(rows, doc.Wording)
	if !ok {
		return nil, ErrCardComparison
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &CardComparisonReview{p.digest, p.draftDigest, task.StageDigest(raw), p.origins, outcome, doc.Wording, rows, resolved}, nil
}

// 固定内部呈现仅供未来原语义终检装配；不授予公开读取或发布权限。
func (p *CardComparisonPlan) VisibleReview(r *CardComparisonReview) (string, error) {
	if !p.Valid() || r == nil || r.inputDigest != p.digest || r.draftDigest != p.draftDigest || !supportDigest(r.payloadDigest) || r.origins != p.origins || len(r.axes) != len(p.axes) {
		return "", ErrCardComparison
	}
	for i, a := range r.axes {
		reason, ok := comparisonAxisReason(p.axes[i], a.Left, a.Right, a.Relation)
		if !ok || a.ID != i+1 || a.Field != p.axes[i].Field || a.Reason != reason {
			return "", ErrCardComparison
		}
	}
	outcome, resolved, ok := comparisonOutcome(r.axes, r.wording)
	if !ok || outcome != r.outcome || resolved != r.resolved {
		return "", ErrCardComparison
	}
	raw, err := json.Marshal(struct {
		Protocol, Focus, Outcome, Wording, StatementOrigins string
		Axes                                                []CardComparisonAxis
	}{CardComparisonVersion, p.cfg.Focus, r.outcome, r.wording, r.origins, r.axes})
	if err != nil {
		return "", ErrCardComparison
	}
	return string(raw), nil
}

func (p *CardComparisonPlan) StagePolicy() task.StagePolicy {
	if !p.Valid() {
		return task.StagePolicy{}
	}
	return task.StagePolicy{Digest: task.StageDigest([]byte(CardComparisonVersion + ":" + p.policy + ":" + p.digest)), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if ctx == nil {
			return ErrCardComparison
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
