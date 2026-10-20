package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AIReportDraft 仅为内部候选，不能从会员接口序列化；它不是终检通过或积分已结算的收据。
type AIReportDraft struct {
	InputDigest string
	Payload     []byte `json:"-"`
}

func (d AIReportDraft) String() string   { return "内部报告候选(正文已隐藏，尚未终检)" }
func (d AIReportDraft) GoString() string { return d.String() }

// Convert 从已落库且已完成的编号抽取步骤回取证据，不接受调用方提供的模型正文。
// 不预留预算、不调用模型、不写 judge 响应；调用编排须另行持有 judge 预算并经过终检。
func (s *AIReportService) Convert(ctx context.Context, r task.StageRequest, selection *finalreport.SelectionStage) (AIReportDraft, error) {
	return s.convertSelection(ctx, r, selection, selection.InputDigest(), selection.Policy(), finalreport.SelectionVersion, false, selection.Build, nil)
}

// ConvertModel 只读取已捕获的选择模型响应；复用前同步核对 Agent 第一步回执。
// 支持后续判定已完成时回取原候选，但不跳过终检服务的全前缀与当前授权校验。
func (s *AIReportService) ConvertModel(ctx context.Context, r task.StageRequest, p *finalreport.SelectionModelPlan) (AIReportDraft, error) {
	return s.convertSelection(ctx, r, p.Selection(), p.InputDigest(), p.StagePolicy(), finalreport.SelectionModelVersion, true, p.Build, func(ctx context.Context, tx finalreport.Tx, v task.Task) error {
		proof, policy, err := p.AgentBinding(v, r.Owner)
		if err != nil {
			return err
		}
		view, err := readStageView(ctx, tx, proof, policy)
		if err != nil {
			return err
		}
		if !view.complete {
			return task.ErrNotReady
		}
		return nil
	})
}

func (s *AIReportService) convertSelection(ctx context.Context, r task.StageRequest, selection *finalreport.SelectionStage, input string, policy task.StagePolicy, version string, allowLater bool, build func(context.Context, string) (string, error), prefix func(context.Context, finalreport.Tx, task.Task) error) (AIReportDraft, error) {
	if s == nil {
		return AIReportDraft{}, points.ErrInvalid
	}
	if !s.enabled {
		return AIReportDraft{}, finalreport.ErrDisabled
	}
	if r.Validate() != nil || r.Stage != "extract" || !selection.Valid() || r.InputDigest != input || !policy.Valid() {
		return AIReportDraft{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	read := func() (stageView, error) {
		var out stageView
		err := s.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
			v, err := lockOwnedTask(ctx, tx, r.UserID, r.TaskID)
			if err != nil {
				return err
			}
			if !selection.Matches(v, s.policy) {
				return task.ErrStagePolicy
			}
			if err := checkOriginalAdmission(ctx, tx, v); err != nil {
				return err
			}
			if err := checkSelectionAdmissionScope(ctx, tx, v, selection); err != nil {
				return err
			}
			if prefix != nil {
				if err := prefix(ctx, tx, v); err != nil {
					return err
				}
			}
			out, err = readStageView(ctx, tx, r, policy)
			if err != nil {
				return err
			}
			if !out.complete || v.CurrentStep < r.StepNo || !allowLater && v.CurrentStep != r.StepNo {
				return task.ErrNotReady
			}
			snaps, theme, err := s.reportEvidence(ctx, tx, v, selection.SnapshotIDs())
			if err != nil {
				return err
			}
			if !selection.MatchesEvidence(snaps, theme) {
				return finalreport.ErrRejected
			}
			return nil
		})
		return out, err
	}
	before, err := read()
	if err != nil {
		return AIReportDraft{}, err
	}
	raw, err := build(ctx, string(before.result.Payload))
	if err != nil {
		return AIReportDraft{}, err
	}
	after, err := read()
	if err != nil {
		return AIReportDraft{}, err
	}
	if before.result.OutputDigest != after.result.OutputDigest || before.checkpoint != after.checkpoint {
		return AIReportDraft{}, points.ErrInvariant
	}
	// 不绑定租约 owner/fence，允许新围栏复用已完成输入；完整步骤键/策略/输出不能换。
	b, err := json.Marshal([]any{version, r.TaskID, r.UserID, before.v.No, r.StepNo,
		r.OperationKey, r.InputDigest, before.result.OutputDigest, before.checkpoint.PolicyDigest, s.policy.Digest()})
	if err != nil {
		return AIReportDraft{}, points.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return AIReportDraft{}, err
	}
	return AIReportDraft{InputDigest: task.StageDigest(b), Payload: []byte(raw)}, nil
}
