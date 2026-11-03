package qa

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

type EvidenceSentence struct {
	ID, StartByte, EndByte int
	Text                   string
}
type EvidenceSource struct {
	SnapshotID                    int64
	Digest, SourceType, SourceURL string
	CollectedAt                   int64
	Sentences                     []EvidenceSentence
}
type EvidenceCall struct {
	Step                                             int
	Stage, ModelName, Status, Verdict, RequestDigest string
	CreatedAt, UpdatedAt                             int64
	InputTokens, OutputTokens                        *int
}
type ExecutionEvidence struct {
	State, PromptVersion, CandidateDigest string
	CreatedAt, UpdatedAt                  time.Time
	Sources                               []EvidenceSource
	Calls                                 []EvidenceCall
}

func (ExecutionEvidence) String() string {
	return "问答资料核验记录（正文及定位已隐藏）"
}
func (v ExecutionEvidence) GoString() string { return v.String() }

// validateEvidenceCalls只读复算原预约请求，失败前缀不冒充完整成功回执。
func (p *Plan) validateEvidenceCalls(r Record, rows []CallRecord) ([]EvidenceCall, error) {
	if !matchesReservations(r, rows) {
		return nil, ErrProtocol
	}
	if r.State != "failed" {
		receipt, err := p.replay(rows)
		if err != nil || !sameJSON(receipt, r.Receipt()) {
			return nil, ErrProtocol
		}
	}
	out := []EvidenceCall{}
	var picks []Citation
	stopped := false
	previous := r.CreatedAt
	for n, row := range rows {
		if stopped || !row.Valid() || row.Call.Step != n+1 || row.Call.InputDigest != p.digest || row.CreatedAt.Before(previous) {
			return nil, ErrProtocol
		}
		request, model, err := p.request(row.Call.Stage, picks)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(request)
		if err != nil {
			return nil, ErrInvalid
		}
		if row.Call.Model != model.Name || row.Call.ProfileDigest != model.ProfileDigest || row.Call.RequestDigest != task.StageDigest(raw) || row.Call.MaxTokens != p.cfg.Limits.MaxInputTokens+p.cfg.Limits.MaxOutputTokens {
			return nil, ErrProtocol
		}
		v := EvidenceCall{Step: row.Call.Step, Stage: row.Call.Stage, ModelName: model.Name, Status: "unknown", Verdict: "unavailable", RequestDigest: row.Call.RequestDigest, CreatedAt: row.CreatedAt.UnixMilli(), UpdatedAt: row.UpdatedAt.UnixMilli()}
		if row.State == "reserved" {
			stopped = true
		} else if row.Result.Failure != "" {
			v.Status = row.Result.Failure
			stopped = true
		} else {
			v.Status = "confirmed"
			d := row.Result.Decision
			if n == 0 {
				v.Verdict = d.Action
				if d.Action == "answer" {
					picks = clonePicks(d.Citations)
					if _, err := p.quotes(picks); err != nil {
						return nil, err
					}
				} else {
					stopped = true
				}
			} else {
				v.Verdict = d.Verdict
				stopped = d.Verdict != "pass"
			}
		}
		// invalid_protocol已通过原适配器元数据核验；其它失败不把零值当实际用量。
		if v.Status == "confirmed" || v.Status == "invalid_protocol" {
			u := row.Result.Usage
			if row.Result.Estimated || row.Result.Model != model.Name || !meta(row.Result.RequestID) || !digest(row.Result.ResponseDigest) || u.Prompt <= 0 || u.Completion <= 0 || u.Total != u.Prompt+u.Completion || u.Prompt > p.cfg.Limits.MaxInputTokens || u.Completion > p.cfg.Limits.MaxOutputTokens || u.Cached > u.Prompt || u.Reasoning > u.Completion {
				return nil, ErrProtocol
			}
			in, output := u.Prompt, u.Completion
			v.InputTokens, v.OutputTokens = &in, &output
		}
		out = append(out, v)
		previous = row.UpdatedAt
	}
	return out, nil
}

// ReadEvidence不需要模型客户端。终态失败也须重新核当前资料/报告权限。
func (c *DurableControl) ReadEvidence(parent context.Context) (ExecutionEvidence, error) {
	var out ExecutionEvidence
	var original Record
	var calls []CallRecord
	err := c.within(parent, false, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, false, false)
		if err != nil {
			return err
		}
		if !slices.Contains([]string{"answered", "refused", "failed"}, r.State) {
			return ErrUnavailable
		}
		if err := c.authorize(ctx, tx, c.plan.Proof()); err != nil {
			return err
		}
		rows, err := tx.Calls(ctx, c.plan.digest)
		if err != nil {
			return err
		}
		verified, err := c.plan.validateEvidenceCalls(r, rows)
		if err != nil {
			return err
		}
		original, calls = r, rows
		out = ExecutionEvidence{State: r.State, PromptVersion: Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Sources: []EvidenceSource{}, Calls: verified}
		candidates := []promptSource{}
		for _, snapshot := range c.plan.scope.Snapshots {
			source := EvidenceSource{SnapshotID: snapshot.ID, Digest: snapshot.Hash, SourceType: snapshot.SourceCategory, SourceURL: snapshot.URL, CollectedAt: snapshot.FetchedAt.UnixMilli(), Sentences: []EvidenceSentence{}}
			candidate := promptSource{ID: snapshot.ID, Hash: snapshot.Hash, Sentences: []promptSentence{}}
			for _, sentence := range snapshot.Sentences {
				source.Sentences = append(source.Sentences, EvidenceSentence{ID: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte, Text: sentence.Text})
				candidate.Sentences = append(candidate.Sentences, promptSentence{ID: sentence.Number, Text: sentence.Text})
			}
			out.Sources = append(out.Sources, source)
			candidates = append(candidates, candidate)
		}
		raw, err := json.Marshal(candidates)
		if err != nil {
			return ErrInvalid
		}
		out.CandidateDigest = task.StageDigest(raw)
		return nil
	})
	if err != nil {
		return ExecutionEvidence{}, err
	}
	// 另一个短事务复核授权、原记录及调用；晚到结果也不能混入前一份导出。
	err = c.within(parent, false, func(ctx context.Context, tx Tx) error {
		r, err := c.load(ctx, tx, false, false)
		if err != nil {
			return err
		}
		if err := c.authorize(ctx, tx, c.plan.Proof()); err != nil {
			return err
		}
		rows, err := tx.Calls(ctx, c.plan.digest)
		if err != nil {
			return err
		}
		if !sameJSON(r, original) || !sameJSON(rows, calls) {
			return ErrConflict
		}
		return nil
	})
	if err != nil {
		return ExecutionEvidence{}, err
	}
	return out, nil
}
