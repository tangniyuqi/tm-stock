package service

import (
	"context"
	"fmt"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type extractionSnapshotReader interface {
	SnapshotForExtraction(context.Context, int64) (sources.Snapshot, error)
}

type extractionRunner interface {
	Run(context.Context, sources.Snapshot) (pipeline.ExtractionResult, error)
}

type candidateWriter interface {
	SavePendingCandidates(context.Context, []candidate.Candidate) ([]candidate.Candidate, error)
}

// AIExtractionService 串联授权快照读取、封闭抽取与内部待审持久化。
// 它不提供 HTTP 接口，也不直接写 addon_quant_theme_stock。
type AIExtractionService struct {
	snapshots  extractionSnapshotReader
	runner     extractionRunner
	candidates candidateWriter
}

func NewAIExtractionService(snapshots extractionSnapshotReader, runner extractionRunner, candidates candidateWriter) (*AIExtractionService, error) {
	if snapshots == nil || runner == nil || candidates == nil {
		return nil, fmt.Errorf("AI 抽取服务依赖不能为空")
	}
	return &AIExtractionService{snapshots: snapshots, runner: runner, candidates: candidates}, nil
}

// ExtractSnapshot 抽取一个快照，并原子写入待审队列；待审为空时不执行数据库写入。
func (s *AIExtractionService) ExtractSnapshot(ctx context.Context, snapshotID int64) (pipeline.ExtractionResult, error) {
	if s == nil || s.snapshots == nil || s.runner == nil || s.candidates == nil || snapshotID <= 0 {
		return pipeline.ExtractionResult{}, fmt.Errorf("AI 抽取服务或快照编号无效")
	}
	snapshot, err := s.snapshots.SnapshotForExtraction(ctx, snapshotID)
	if err != nil {
		return pipeline.ExtractionResult{}, fmt.Errorf("读取可抽取快照失败: %w", err)
	}
	result, err := s.runner.Run(ctx, snapshot)
	if err != nil {
		return pipeline.ExtractionResult{}, err
	}
	if len(result.Candidates) == 0 {
		return result, nil
	}
	stored, err := s.candidates.SavePendingCandidates(ctx, result.Candidates)
	if err != nil {
		return pipeline.ExtractionResult{}, fmt.Errorf("保存抽取待审候选失败: %w", err)
	}
	if len(stored) != len(result.Candidates) {
		return pipeline.ExtractionResult{}, fmt.Errorf("候选待审队列写入数量不一致")
	}
	for _, item := range stored {
		if item.ID <= 0 {
			return pipeline.ExtractionResult{}, fmt.Errorf("待审候选未返回有效编号")
		}
	}
	result.Candidates = stored
	return result, nil
}
