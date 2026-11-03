package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// Evidence复用原归属/授权和审计，不需要模型工厂，不执行提交或积分操作。
func (s *AIQAService) Evidence(parent context.Context, token string, uid, cid, seq int64) (dto.AIQAEvidence, error) {
	if !conversation.ID(cid) || !conversation.ID(seq) {
		return dto.AIQAEvidence{}, qa.ErrInvalid
	}
	if err := s.check(parent, token, uid); err != nil {
		return dto.AIQAEvidence{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	var plan *qa.Plan
	var original qa.Record
	var message conversation.Message
	err := s.within(ctx, func(ctx context.Context, tx qa.AccessTx) error {
		r, m, err := qaOwned(ctx, tx, uid, cid, seq, "")
		if err != nil {
			return err
		}
		if r.State != "answered" && r.State != "refused" && r.State != "failed" {
			return qa.ErrUnavailable
		}
		original, message = r, m
		if m.Input.Skill == "report_qa" {
			return nil
		}
		plan, err = s.restore(ctx, tx, r, m)
		return err
	})
	if err != nil {
		return dto.AIQAEvidence{}, err
	}
	var access qa.ReportAccess
	if message.Input.Skill == "report_qa" {
		plan, err = s.restoreReportRead(ctx, uid, cid, seq, original, message)
		if err != nil {
			return dto.AIQAEvidence{}, err
		}
		access = s.reportAccess
	}
	control, err := qa.NewDurableControlWithReports(s.store, plan, access)
	if err != nil {
		return dto.AIQAEvidence{}, err
	}
	evidence, err := control.ReadEvidence(ctx)
	if err != nil {
		return dto.AIQAEvidence{}, err
	}
	if err := s.check(ctx, token, uid); err != nil {
		return dto.AIQAEvidence{}, err
	}
	out := dto.AIQAEvidence{Seq: seq, State: evidence.State, Skill: message.Input.Skill, SubjectType: message.Input.SubjectType, SubjectID: message.Input.SubjectID, SubjectName: plan.Proof().SubjectName, TaskID: message.Input.TaskID, PromptVersion: evidence.PromptVersion, CandidateDigest: evidence.CandidateDigest, CreatedAt: evidence.CreatedAt.UnixMilli(), UpdatedAt: evidence.UpdatedAt.UnixMilli(), Sources: []dto.AIQAEvidenceSource{}, Calls: []dto.AIQAEvidenceCall{}}
	for _, source := range evidence.Sources {
		v := dto.AIQAEvidenceSource{SnapshotID: source.SnapshotID, Digest: source.Digest, SourceType: source.SourceType, SourceURL: source.SourceURL, CollectedAt: source.CollectedAt, Sentences: []dto.AIQAEvidenceSentence{}}
		for _, sentence := range source.Sentences {
			v.Sentences = append(v.Sentences, dto.AIQAEvidenceSentence{ID: sentence.ID, StartByte: sentence.StartByte, EndByte: sentence.EndByte, Text: sentence.Text})
		}
		out.Sources = append(out.Sources, v)
	}
	for _, call := range evidence.Calls {
		out.Calls = append(out.Calls, dto.AIQAEvidenceCall{Step: call.Step, Stage: call.Stage, ModelName: call.ModelName, Status: call.Status, Verdict: call.Verdict, RequestDigest: call.RequestDigest, CreatedAt: call.CreatedAt, UpdatedAt: call.UpdatedAt, InputTokens: call.InputTokens, OutputTokens: call.OutputTokens})
	}
	return out, nil
}
