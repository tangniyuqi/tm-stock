package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 终态读取也只能加载原预留/检查点下的捕获包，不补造模型证据，不要求旧租约仍有效。
func dossierCapturedStep(ctx context.Context, tx task.StageTx, v task.Task, step int, input, key, policy string) ([]byte, error) {
	state, err := tx.LockBudget(ctx, v.UserID, v.ID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Reservations(ctx, v.UserID, v.ID)
	if err != nil {
		return nil, err
	}
	if task.CheckBudget(v, state, rows) != nil || step < 1 || len(rows) < step || v.CurrentStep < step {
		return nil, points.ErrInvariant
	}
	res := rows[step-1]
	cp, err := tx.StageCheckpoint(ctx, v.UserID, v.ID, step)
	if err != nil {
		return nil, err
	}
	result, err := tx.StageResult(ctx, v.UserID, v.ID, step)
	if err != nil {
		return nil, err
	}
	r := task.StageRequest{TaskID: v.ID, UserID: v.UserID, StepNo: step, Stage: "judge", InputDigest: input}
	if task.CheckStageResult(r, res, result) != nil || res.Key != key || res.InputDigest != input || cp.TaskID != v.ID || cp.UserID != v.UserID || cp.StepNo != step || cp.ReservationSeq != step || cp.Stage != "judge" || cp.InputDigest != input || cp.PolicyDigest != policy || cp.OutputDigest != result.OutputDigest || cp.Fence < result.Fence || cp.Fence > v.Fence || !task.ValidOwner(cp.Owner) || cp.CreatedAt.Before(result.CreatedAt) || cp.EventSeq <= 0 || cp.EventSeq > v.EventSeq {
		return nil, points.ErrInvariant
	}
	event, err := tx.StageEvent(ctx, v.ID, cp.EventSeq)
	if err != nil {
		return nil, err
	}
	if event != (task.Event{TaskID: v.ID, Seq: cp.EventSeq, Type: "step_completed", Status: task.Running, Step: step}) {
		return nil, points.ErrInvariant
	}
	return result.Payload, nil
}
func dossierCapturedReview(ctx context.Context, tx task.StageTx, v task.Task, p *dossier.SupportPlan) (*dossier.SupportReview, error) {
	if !p.Valid() || !p.MatchesTask(v) || p.StepNo() != 4 {
		return nil, points.ErrInvalid
	}
	raw, err := dossierCapturedStep(ctx, tx, v, 4, p.InputDigest(), p.OperationKey(v, 4), p.StagePolicy().Digest)
	if err != nil {
		return nil, err
	}
	return p.Review(ctx, raw)
}

func dossierCapturedComparison(ctx context.Context, tx task.StageTx, v task.Task, p *dossier.CardComparisonPlan) (*dossier.CardComparisonReview, error) {
	if !p.Valid() || !p.MatchesTask(v) || p.StepNo() != 5 {
		return nil, points.ErrInvalid
	}
	raw, err := dossierCapturedStep(ctx, tx, v, 5, p.InputDigest(), p.OperationKey(v, 5), p.StagePolicy().Digest)
	if err != nil {
		return nil, err
	}
	return p.Review(ctx, raw)
}

func loadDossierReportPolicy(ctx context.Context, tx finalreport.Tx, v task.Task, p AIL1ReportPlan, raw []byte) (*finalreport.Policy, error) {
	pending, err := p.pendingPolicy()
	if err != nil {
		return nil, err
	}
	proposal, err := pending.Decode(string(raw))
	if err != nil {
		return nil, err
	}
	var used []sources.Snapshot
	for _, id := range pending.EvidenceIDs(proposal) {
		for _, snap := range p.Snapshots {
			if snap.ID == id {
				used = append(used, snap)
				break
			}
		}
	}
	in, visible, err := pending.Inputs(string(raw), used)
	if err != nil {
		return nil, err
	}
	ep, err := finalreport.NewEntailmentCheck(p.Model, in)
	if err != nil {
		return nil, err
	}
	packet, err := dossierCapturedStep(ctx, tx, v, 3, ep.InputDigest(), ep.OperationKey(v, 3), ep.StagePolicy().Digest)
	if err != nil {
		return nil, err
	}
	ent, err := ep.Evidence(string(packet))
	if err != nil {
		return nil, err
	}
	review, err := dossierCapturedReview(ctx, tx, v, p.Dossier)
	if err != nil {
		return nil, err
	}
	visible, err = finalreport.DossierVisible(visible, p.Dossier, review)
	if err != nil {
		return nil, err
	}
	var compared *dossier.CardComparisonReview
	semanticStep := 5
	if p.ReportProfile == task.DossierComparisonReportProfile {
		compared, err = dossierCapturedComparison(ctx, tx, v, p.Comparison)
		if err != nil {
			return nil, err
		}
		visible, err = finalreport.DossierComparisonVisible(visible, p.Dossier, review, p.Comparison, compared)
		if err != nil {
			return nil, err
		}
		semanticStep = 6
	}
	sp, err := finalreport.NewSemanticCheck(p.Model, v.No, visible)
	if err != nil {
		return nil, err
	}
	packet, err = dossierCapturedStep(ctx, tx, v, semanticStep, sp.InputDigest(), sp.OperationKey(v, semanticStep), sp.StagePolicy().Digest)
	if err != nil {
		return nil, err
	}
	sem, err := sp.Evidence(string(packet))
	if err != nil {
		return nil, err
	}
	if p.ReportProfile == task.DossierComparisonReportProfile {
		return finalreport.NewDossierComparisonCapturedPolicy(p.Policy, p.Dossier, p.Comparison, review, compared, ent, sem, p.Companies)
	}
	return finalreport.NewDossierCapturedPolicy(p.Policy, p.Dossier, review, ent, sem, p.Companies)
}
