package service

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type AIReportInputBinding struct {
	Task    task.Task
	Agent   agenttask.Plan
	Receipt agenttask.Receipt
}

func (AIReportInputBinding) String() string     { return "报告输入读取绑定（身份已隐藏）" }
func (b AIReportInputBinding) GoString() string { return b.String() }

type AIReportInputSource interface {
	ForInputs(context.Context, int64, int64) (AIReportInputBinding, error)
}

// 启动时装配；不提供模型客户端、不发起外部读取，不授予新报告发布权。
func (s *AIReportReadService) SetInputEvidence(source AIReportInputSource, payloads agents.PayloadReader) error {
	if s == nil || source == nil || payloads == nil {
		return points.ErrInvalid
	}
	s.inputSource, s.inputPayloads = source, payloads
	return nil
}

func (s *AIReportReadService) ReadInputs(parent context.Context, uid, id int64) (dto.AIReportInputs, error) {
	if s == nil || parent == nil || uid <= 0 || id <= 0 {
		return dto.AIReportInputs{}, points.ErrInvalid
	}
	if !s.enabled || s.inputSource == nil || s.inputPayloads == nil {
		return dto.AIReportInputs{}, finalreport.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	before, err := s.Read(ctx, uid, id)
	if err != nil {
		return dto.AIReportInputs{}, err
	}
	binding, err := s.inputSource.ForInputs(ctx, uid, id)
	if err != nil {
		return dto.AIReportInputs{}, err
	}
	v, p := binding.Task, binding.Agent
	if v.UserID != uid || v.ID != id || v.Status != task.Succeeded || v.SubjectID != before.SubjectID || p.ValidateScope(v) != nil {
		return dto.AIReportInputs{}, points.ErrInvariant
	}
	r, result, err := s.inputPayloads.Quarantined(ctx, p.ActorID, p.RequestID())
	if err != nil {
		return dto.AIReportInputs{}, err
	}
	if binding.Receipt != (agenttask.Receipt{RequestID: r.Request.RequestID, RemoteID: r.RemoteID, ProfileDigest: r.ProfileDigest, ResultDigest: r.ResultDigest}) {
		return dto.AIReportInputs{}, points.ErrInvariant
	}
	// NewHint同时验证原任务/操作人/请求/模型配置/隔离结果摘要；完整输入不会混入选择提示。
	if _, err := agenttask.NewHint(v, p, r, result); err != nil {
		return dto.AIReportInputs{}, err
	}
	_, audit, err := agents.InputAuditPayload(result)
	if err != nil {
		return dto.AIReportInputs{}, err
	}
	if audit == nil {
		return dto.AIReportInputs{}, finalreport.ErrUnavailable
	}
	fixture, err := audit.Fixture()
	if err != nil {
		return dto.AIReportInputs{}, err
	}
	out := dto.AIReportInputs{TaskID: id, SubjectID: before.SubjectID, Schema: audit.Schema, Scope: audit.Scope, UpstreamCommit: audit.UpstreamCommit, Provider: audit.Provider, FixtureDigest: audit.FixtureDigest, FixtureSubject: fixture.Subject, FixtureNotice: fixture.Notice, Facts: []dto.AIReportInputFact{}, Calls: []dto.AIReportInputCall{}}
	for _, f := range fixture.Facts {
		out.Facts = append(out.Facts, dto.AIReportInputFact{ID: f.ID, Text: f.Text})
	}
	for _, c := range audit.Calls {
		out.Calls = append(out.Calls, dto.AIReportInputCall{Seq: c.Seq, Node: c.Node, Model: c.Model, State: c.State, RequestDigest: c.RequestDigest, PromptDigest: c.PromptDigest, InputTokens: c.InputTokens, OutputTokens: c.OutputTokens})
	}
	after, err := s.Read(ctx, uid, id)
	if err != nil {
		return dto.AIReportInputs{}, err
	}
	b, err := json.Marshal(before)
	if err != nil {
		return dto.AIReportInputs{}, points.ErrInvariant
	}
	a, err := json.Marshal(after)
	if err != nil || !bytes.Equal(a, b) {
		return dto.AIReportInputs{}, finalreport.ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return dto.AIReportInputs{}, err
	}
	return out, nil
}
