package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// AIQAPolicies只提供受信任的无密钥配置，不得从请求或不受限远端加载。
type AIQAPolicies interface {
	ForPolicy(context.Context, string) (qa.Config, error)
}
type AIQAProviders interface {
	ForPlan(context.Context, qa.Proof) ([3]*provider.OpenAICompat, error)
}
type AIQAAuthenticator interface {
	Authenticate(context.Context, string) (int64, error)
}
type AIQAService struct {
	store          qa.AccessStore
	policies       AIQAPolicies
	models         AIQAProviders
	auth           AIQAAuthenticator
	submit         bool
	reportEvidence AIReportQAEvidenceSource
	reportConfig   AIReportQAConfigProvider
	reportAccess   *AIReportQAAccess
}

func NewAIQAService(store qa.AccessStore, policies AIQAPolicies, models AIQAProviders, auth AIQAAuthenticator, submit bool) (*AIQAService, error) {
	return newAIQAService(store, policies, models, auth, submit, nil, nil)
}

// NewAIQAServiceWithReports 额外装配报告追问；两项依赖必须同时提供，否则报告路径失败关闭。
func NewAIQAServiceWithReports(store qa.AccessStore, policies AIQAPolicies, models AIQAProviders, auth AIQAAuthenticator, submit bool, evidence AIReportQAEvidenceSource, config AIReportQAConfigProvider) (*AIQAService, error) {
	return newAIQAService(store, policies, models, auth, submit, evidence, config)
}

func newAIQAService(store qa.AccessStore, policies AIQAPolicies, models AIQAProviders, auth AIQAAuthenticator, submit bool, evidence AIReportQAEvidenceSource, config AIReportQAConfigProvider) (*AIQAService, error) {
	if store == nil || auth == nil || submit && (policies == nil || models == nil) || (evidence == nil) != (config == nil) {
		return nil, qa.ErrInvalid
	}
	svc := &AIQAService{store: store, policies: policies, models: models, auth: auth, submit: submit}
	if evidence != nil {
		access, err := NewAIReportQAAccess(evidence)
		if err != nil {
			return nil, err
		}
		svc.reportEvidence, svc.reportConfig, svc.reportAccess = evidence, config, access
	}
	return svc, nil
}

func (s *AIQAService) reportReady() bool {
	return s != nil && s.reportEvidence != nil && s.reportConfig != nil && s.reportAccess != nil
}
func (s *AIQAService) check(ctx context.Context, token string, uid int64) error {
	if s == nil || s.store == nil || s.auth == nil || ctx == nil || !conversation.ID(uid) || token == "" {
		return qa.ErrInvalid
	}
	actual, err := s.auth.Authenticate(ctx, token)
	if err != nil {
		return err
	}
	if actual != uid {
		return memberauth.ErrUnauthorized
	}
	return nil
}
func (s *AIQAService) within(parent context.Context, fn func(context.Context, qa.AccessTx) error) error {
	if parent == nil || fn == nil || s == nil || s.store == nil {
		return qa.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	return s.store.WithinAccess(ctx, func(tx qa.AccessTx) error { return fn(ctx, tx) })
}
func qaOwned(ctx context.Context, tx qa.AccessTx, uid, cid, seq int64, key string) (qa.Record, conversation.Message, error) {
	conv, err := ownedConversation(ctx, tx, uid, cid, false)
	if err != nil {
		return qa.Record{}, conversation.Message{}, err
	}
	var r qa.Record
	if key != "" {
		r, err = tx.QuestionByKey(ctx, uid, cid, key)
	} else {
		r, err = tx.Question(ctx, qa.Identity{UserID: uid, ConversationID: cid, MessageSeq: seq}, false)
	}
	if err != nil {
		return qa.Record{}, conversation.Message{}, err
	}
	id := r.Proof.Identity
	if !r.Valid() || id.UserID != uid || id.ConversationID != cid || id.MessageSeq > conv.LastSeq || seq > 0 && id.MessageSeq != seq {
		return qa.Record{}, conversation.Message{}, qa.ErrProtocol
	}
	ms, err := tx.Messages(ctx, uid, cid, id.MessageSeq-1, id.MessageSeq, 1)
	if err != nil {
		return qa.Record{}, conversation.Message{}, err
	}
	if len(ms) != 1 || !ms[0].Valid() || ms[0].UserID != uid || ms[0].ConversationID != cid || ms[0].Seq != id.MessageSeq || ms[0].Input.Kind != "question" || ms[0].RequestHash != r.MessageHash || key != "" && ms[0].Key != key || ms[0].Input.SubjectType != r.Proof.SubjectType || ms[0].Input.SubjectID != r.Proof.SubjectID {
		return qa.Record{}, conversation.Message{}, qa.ErrProtocol
	}
	skill := ms[0].Input.Skill
	if skill != "material_qa" && skill != "report_qa" || skill == "material_qa" && ms[0].Input.TaskID != 0 || skill == "report_qa" && !conversation.ID(ms[0].Input.TaskID) {
		return qa.Record{}, conversation.Message{}, qa.ErrProtocol
	}
	if skill == "report_qa" {
		taskID, _, ok := qa.ParseReportApproval(r.Proof.ScopeApproval)
		if !ok || taskID != ms[0].Input.TaskID {
			return qa.Record{}, conversation.Message{}, qa.ErrProtocol
		}
	}
	return r, ms[0], nil
}
func qaStatus(r qa.Record, m conversation.Message) dto.AIQAStatus {
	return dto.AIQAStatus{Seq: m.Seq, State: r.State, SubjectType: m.Input.SubjectType, SubjectID: m.Input.SubjectID, Skill: m.Input.Skill, Question: m.Input.Question, CreatedAt: r.CreatedAt.UnixMilli(), UpdatedAt: r.UpdatedAt.UnixMilli()}
}
func (s *AIQAService) status(ctx context.Context, token string, uid, cid, seq int64, key string) (dto.AIQAStatus, error) {
	if !conversation.ID(cid) || key == "" && !conversation.ID(seq) || key != "" && !points.ValidKey(key) {
		return dto.AIQAStatus{}, qa.ErrInvalid
	}
	if err := s.check(ctx, token, uid); err != nil {
		return dto.AIQAStatus{}, err
	}
	var out dto.AIQAStatus
	err := s.within(ctx, func(ctx context.Context, tx qa.AccessTx) error {
		r, m, err := qaOwned(ctx, tx, uid, cid, seq, key)
		if err != nil {
			return err
		}
		out = qaStatus(r, m)
		return nil
	})
	if err != nil {
		return dto.AIQAStatus{}, err
	}
	if err := s.check(ctx, token, uid); err != nil {
		return dto.AIQAStatus{}, err
	}
	return out, nil
}
func (s *AIQAService) Receipt(ctx context.Context, token string, uid, cid int64, key string) (dto.AIQAStatus, error) {
	return s.status(ctx, token, uid, cid, 0, key)
}
func (s *AIQAService) Status(ctx context.Context, token string, uid, cid, seq int64) (dto.AIQAStatus, error) {
	return s.status(ctx, token, uid, cid, seq, "")
}
func (s *AIQAService) restore(ctx context.Context, tx qa.AccessTx, r qa.Record, m conversation.Message) (*qa.Plan, error) {
	if s.policies == nil {
		return nil, qa.ErrUnavailable
	}
	cfg, err := s.policies.ForPolicy(ctx, r.Proof.PolicyDigest)
	if err != nil {
		return nil, err
	}
	scope, err := tx.ScopeForProof(ctx, r.Proof)
	if err != nil {
		return nil, err
	}
	now, err := tx.Now(ctx)
	if err != nil {
		return nil, err
	}
	p, err := qa.NewPlan(r.Proof.Identity, m.Input, scope, cfg, now)
	if err != nil {
		return nil, err
	}
	if p.Proof().InputDigest != r.Proof.InputDigest || p.Proof().PolicyDigest != r.Proof.PolicyDigest {
		return nil, qa.ErrProtocol
	}
	return p, nil
}
func (s *AIQAService) Answer(parent context.Context, token string, uid, cid, seq int64) (dto.AIQAAnswer, error) {
	if !conversation.ID(cid) || !conversation.ID(seq) {
		return dto.AIQAAnswer{}, qa.ErrInvalid
	}
	if err := s.check(parent, token, uid); err != nil {
		return dto.AIQAAnswer{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	var p *qa.Plan
	var original qa.Record
	var report *qa.Record
	var reportMessage conversation.Message
	err := s.within(ctx, func(ctx context.Context, tx qa.AccessTx) error {
		r, m, err := qaOwned(ctx, tx, uid, cid, seq, "")
		if err != nil {
			return err
		}
		if r.State != "answered" && r.State != "refused" {
			return qa.ErrUnavailable
		}
		original = r
		if m.Input.Skill == "report_qa" {
			copied := r
			report, reportMessage = &copied, m
			return nil
		}
		p, err = s.restore(ctx, tx, r, m)
		return err
	})
	if err != nil {
		return dto.AIQAAnswer{}, err
	}
	if report != nil {
		return s.answerReport(ctx, token, uid, cid, seq, *report, reportMessage)
	}
	ctl, err := qa.NewDurableControl(s.store, p)
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
	out := dto.AIQAAnswer{Seq: seq, State: view.State, Template: view.Template, SubjectName: view.SubjectName, Quotes: []dto.AIQAQuote{}, AIGenerated: original.Calls > 0}
	if out.AIGenerated {
		out.ModelName = p.Proof().Models[0].Name
		out.FilingNo = p.Proof().Models[0].FilingNo
	}
	for _, q := range view.Quotes {
		out.Quotes = append(out.Quotes, dto.AIQAQuote{SnapshotID: q.SnapshotID, StartByte: q.Start, EndByte: q.End, Excerpt: q.Text, SourceType: q.SourceType, SourceURL: q.SourceURL, CollectedAt: q.CollectedAt})
	}
	return out, nil
}
func (s *AIQAService) Submit(parent context.Context, token string, uid, cid int64, key string, in dto.AIQAInput) (dto.AIQAStatus, error) {
	if in.Skill == "report_qa" {
		return s.submitReport(parent, token, uid, cid, key, in)
	}
	m := conversation.MessageInput{Kind: "question", SubjectType: in.SubjectType, SubjectID: in.SubjectID, Skill: in.Skill, TaskID: in.TaskID, Question: in.Question}
	if !conversation.ID(cid) || !conversation.ID(in.Version) || !points.ValidKey(key) || !in.Valid() || !m.Valid() || m.Skill != "material_qa" || m.SubjectType != "theme" {
		return dto.AIQAStatus{}, qa.ErrInvalid
	}
	if err := s.check(parent, token, uid); err != nil {
		return dto.AIQAStatus{}, err
	}
	if !s.submit || s.policies == nil || s.models == nil {
		return dto.AIQAStatus{}, qa.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	var p *qa.Plan
	var replay *dto.AIQAStatus
	err := s.within(ctx, func(ctx context.Context, tx qa.AccessTx) error {
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
			v := qaStatus(r, existing)
			replay = &v
			return nil
		}
		if !errors.Is(err, qa.ErrNotFound) {
			return err
		}
		if conv.Archived || conv.Version != in.Version || conv.LastSeq >= conversation.MaxID-1 || conv.Version >= conversation.MaxID {
			return qa.ErrConflict
		}
		scope, policy, err := tx.Scope(ctx, m.SubjectType, m.SubjectID)
		if err != nil {
			return err
		}
		cfg, err := s.policies.ForPolicy(ctx, policy)
		if err != nil {
			return err
		}
		now, err := tx.Now(ctx)
		if err != nil {
			return err
		}
		p, err = qa.NewPlan(qa.Identity{UserID: uid, ConversationID: cid, MessageSeq: conv.LastSeq + 1}, m, scope, cfg, now)
		if err != nil {
			return err
		}
		if p.Proof().PolicyDigest != policy {
			return qa.ErrUnavailable
		}
		return nil
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
	models, err := s.models.ForPlan(ctx, p.Proof())
	if err != nil {
		return dto.AIQAStatus{}, err
	}
	for n, model := range p.Proof().Models {
		if !models[n].MatchesDirect(model.ProfileDigest, model.Name) {
			return dto.AIQAStatus{}, qa.ErrUnavailable
		}
	}
	ctl, err := qa.NewDurableControl(s.store, p)
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
