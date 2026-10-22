package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// submitReport 复用同一会话/问答存储，只把证据范围换成当前可读报告；不创建任务或扣积分。
func (s *AIQAService) submitReport(parent context.Context, token string, uid, cid int64, key string, in dto.AIQAInput) (dto.AIQAStatus, error) {
	if !conversation.ID(cid) || !points.ValidKey(key) || !in.Valid() || in.Skill != "report_qa" {
		return dto.AIQAStatus{}, qa.ErrInvalid
	}
	if err := s.check(parent, token, uid); err != nil {
		return dto.AIQAStatus{}, err
	}
	if !s.submit || !s.reportReady() {
		return dto.AIQAStatus{}, qa.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	evidence, err := s.reportEvidence.ForQA(ctx, uid, in.TaskID)
	if err != nil {
		return dto.AIQAStatus{}, reportQAError(err)
	}
	if evidence.TaskID != in.TaskID || evidence.SubjectID != in.SubjectID || evidence.SubjectType != in.SubjectType {
		return dto.AIQAStatus{}, qa.ErrUnavailable
	}
	cfg, err := s.reportConfig.ForReport(ctx, evidence)
	if err != nil {
		return dto.AIQAStatus{}, reportQAError(err)
	}
	m := conversation.MessageInput{Kind: "question", SubjectType: in.SubjectType, SubjectID: in.SubjectID, Skill: "report_qa", TaskID: in.TaskID, Question: in.Question}
	var plan *qa.Plan
	var replay *dto.AIQAStatus
	err = s.within(ctx, func(ctx context.Context, tx qa.AccessTx) error {
		conv, err := ownedConversation(ctx, tx, uid, cid, false)
		if err != nil {
			return err
		}
		r, existing, err := qaOwned(ctx, tx, uid, cid, 0, key)
		if err == nil {
			hash, err := m.Digest(uid, cid)
			if err != nil {
				return err
			}
			if hash != r.MessageHash {
				return qa.ErrConflict
			}
			value := qaStatus(r, existing)
			replay = &value
			return nil
		}
		if !errors.Is(err, qa.ErrNotFound) {
			return err
		}
		if conv.Archived || conv.Version != in.Version || conv.LastSeq >= conversation.MaxID-1 || conv.Version >= conversation.MaxID {
			return qa.ErrConflict
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		scope, err := qa.BuildReportScope(evidence, now)
		if err != nil {
			return err
		}
		plan, err = qa.NewReportPlanScoped(qa.Identity{UserID: uid, ConversationID: cid, MessageSeq: conv.LastSeq + 1}, m, scope, cfg, now)
		return err
	})
	if err != nil {
		return dto.AIQAStatus{}, err
	}
	if replay != nil {
		if err := s.check(ctx, token, uid); err != nil {
			return dto.AIQAStatus{}, err
		}
		return *replay, nil
	}
	models, err := s.models.ForPlan(ctx, plan.Proof())
	if err != nil {
		return dto.AIQAStatus{}, err
	}
	for n, model := range plan.Proof().Models {
		if !models[n].MatchesDirect(model.ProfileDigest, model.Name) {
			return dto.AIQAStatus{}, qa.ErrUnavailable
		}
	}
	ctl, err := qa.NewDurableControlWithReports(s.store, plan, s.reportAccess)
	if err != nil {
		return dto.AIQAStatus{}, err
	}
	engine, err := qa.NewEngine(models[0], models[1], models[2], ctl)
	if err != nil {
		return dto.AIQAStatus{}, err
	}
	if _, err := ctl.Enqueue(ctx, key, in.Version); err != nil {
		return dto.AIQAStatus{}, err
	}
	_, err = ctl.RunChecked(ctx, engine, func(ctx context.Context) error { return s.check(ctx, token, uid) })
	if check := s.check(ctx, token, uid); check != nil {
		return dto.AIQAStatus{}, check
	}
	if errors.Is(err, qa.ErrQuota) {
		if audit := ctl.RejectQueued(ctx); audit != nil {
			return dto.AIQAStatus{}, qa.ErrUnavailable
		}
		return dto.AIQAStatus{}, qa.ErrQuota
	}
	if err != nil {
		return dto.AIQAStatus{}, qa.ErrUnavailable
	}
	return s.Receipt(ctx, token, uid, cid, key)
}

// answerReport 每次重读当前报告并回放原存证；报告撤回/不可读时不返回旧引文。
func (s *AIQAService) answerReport(parent context.Context, token string, uid, cid, seq int64, r qa.Record, m conversation.Message) (dto.AIQAAnswer, error) {
	if !s.reportReady() {
		return dto.AIQAAnswer{}, qa.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	scope, err := s.reportAccess.Scope(ctx, r.Proof)
	if err != nil {
		return dto.AIQAAnswer{}, reportQAError(err)
	}
	evidence, err := s.reportEvidence.ForQA(ctx, uid, m.Input.TaskID)
	if err != nil {
		return dto.AIQAAnswer{}, reportQAError(err)
	}
	cfg, err := s.reportConfig.ForReport(ctx, evidence)
	if err != nil {
		return dto.AIQAAnswer{}, reportQAError(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	plan, err := qa.NewReportPlanScoped(r.Proof.Identity, m.Input, scope, cfg, now)
	if err != nil {
		return dto.AIQAAnswer{}, qa.ErrUnavailable
	}
	if plan.Proof().InputDigest != r.Proof.InputDigest || plan.Proof().PolicyDigest != r.Proof.PolicyDigest {
		return dto.AIQAAnswer{}, qa.ErrProtocol
	}
	ctl, err := qa.NewDurableControlWithReports(s.store, plan, s.reportAccess)
	if err != nil {
		return dto.AIQAAnswer{}, err
	}
	view, err := ctl.Read(ctx)
	if err != nil {
		return dto.AIQAAnswer{}, err
	}
	if err := s.check(ctx, token, uid); err != nil {
		return dto.AIQAAnswer{}, err
	}
	out := dto.AIQAAnswer{Seq: seq, State: view.State, Template: view.Template, SubjectName: view.SubjectName, Quotes: []dto.AIQAQuote{}, AIGenerated: r.Calls > 0}
	if out.AIGenerated {
		out.ModelName = plan.Proof().Models[0].Name
		out.FilingNo = plan.Proof().Models[0].FilingNo
	}
	for _, q := range view.Quotes {
		out.Quotes = append(out.Quotes, dto.AIQAQuote{SnapshotID: q.SnapshotID, StartByte: q.Start, EndByte: q.End, Excerpt: q.Text, SourceType: q.SourceType, SourceURL: q.SourceURL, CollectedAt: q.CollectedAt})
	}
	return out, nil
}

func reportQAError(err error) error {
	if errors.Is(err, qa.ErrInvalid) || errors.Is(err, qa.ErrUnavailable) || errors.Is(err, qa.ErrProtocol) || errors.Is(err, qa.ErrNotFound) {
		return err
	}
	if errors.Is(err, points.ErrNotFound) || errors.Is(err, conversation.ErrNotFound) {
		return qa.ErrNotFound
	}
	return qa.ErrUnavailable
}
