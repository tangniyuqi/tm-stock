package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type ExtractionRecoverySpec struct {
	SnapshotID   int64  `json:"snapshot_id"`
	OperationKey string `json:"operation_key"`
}
type recoveryThemes interface {
	ThemeForRecovery(context.Context, int64) (string, error)
}

type AIExtractionRecoveryPlanner struct {
	snapshots extractionSnapshotReader
	themes    recoveryThemes
	specs     []ExtractionRecoverySpec
}

func NewAIExtractionRecoveryPlanner(snapshots extractionSnapshotReader, themes recoveryThemes, specs []ExtractionRecoverySpec) (*AIExtractionRecoveryPlanner, error) {
	if snapshots == nil || themes == nil || len(specs) < 1 || len(specs) > task.MaxSteps {
		return nil, points.ErrInvalid
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		if spec.SnapshotID <= 0 || !points.ValidKey(spec.OperationKey) || seen[spec.OperationKey] {
			return nil, points.ErrInvalid
		}
		seen[spec.OperationKey] = true
	}
	return &AIExtractionRecoveryPlanner{snapshots: snapshots, themes: themes, specs: append([]ExtractionRecoverySpec(nil), specs...)}, nil
}

func (p *AIExtractionRecoveryPlanner) resolve(ctx context.Context, themeID, snapshotID int64) (pipeline.ExtractionStage, error) {
	theme, err := p.themes.ThemeForRecovery(ctx, themeID)
	if err != nil {
		return pipeline.ExtractionStage{}, err
	}
	snapshot, err := p.snapshots.SnapshotForExtraction(ctx, snapshotID)
	if err != nil {
		return pipeline.ExtractionStage{}, err
	}
	return pipeline.NewExtractionStage(snapshot, guard.Vocabulary{
		Predicates: map[string]bool{"DISCLOSES_BUSINESS": true, "LISTED_IN_CATALOG": true},
		Themes:     map[string][]string{theme: nil},
	})
}

func (p *AIExtractionRecoveryPlanner) Plan(ctx context.Context, v task.Task) (task.RecoveryPlan, error) {
	if p == nil || v.SubjectType != task.Theme || v.SubjectID <= 0 {
		return task.RecoveryPlan{}, points.ErrInvalid
	}
	plan := task.RecoveryPlan{UserID: v.UserID, TaskID: v.ID, TaskNo: v.No}
	for _, spec := range p.specs {
		stage, err := p.resolve(ctx, v.SubjectID, spec.SnapshotID)
		if err != nil {
			return task.RecoveryPlan{}, err
		}
		plan.Steps = append(plan.Steps, task.RecoveryStep{Stage: "extract", OperationKey: spec.OperationKey, InputDigest: stage.InputDigest(), Policy: stage.Policy(), Recheck: func(ctx context.Context) error {
			current, err := p.resolve(ctx, v.SubjectID, spec.SnapshotID)
			if err != nil {
				return err
			}
			if current.InputDigest() != stage.InputDigest() || current.Policy().Digest != stage.Policy().Digest {
				return task.ErrStagePolicy
			}
			return nil
		}})
	}
	if err := plan.Validate(v); err != nil {
		return task.RecoveryPlan{}, err
	}
	return plan, nil
}
