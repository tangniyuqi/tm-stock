package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
)

type AIL0Service struct {
	store     pipeline.RunStore
	snapshots extractionSnapshotReader
	runner    extractionRunner
	policy    pipeline.RunPolicy
	busy      chan struct{}
}

func NewAIL0Service(store pipeline.RunStore, snapshots extractionSnapshotReader, runner extractionRunner, policy pipeline.RunPolicy) (*AIL0Service, error) {
	if store == nil || snapshots == nil || (runner != nil && (policy.Provider == "" || policy.Model == "" || policy.ApprovalRef == "")) {
		return nil, pipeline.ErrInvalidRun
	}
	return &AIL0Service{store: store, snapshots: snapshots, runner: runner, policy: policy, busy: make(chan struct{}, 1)}, nil
}

func (s *AIL0Service) RunByRequest(ctx context.Context, actorID int64, requestID string) (pipeline.RunRecord, error) {
	if actorID <= 0 || !pipeline.ValidRequestID(requestID) {
		return pipeline.RunRecord{}, pipeline.ErrInvalidRun
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	record, err := s.store.RunByRequest(ctx, requestID)
	if err != nil {
		return pipeline.RunRecord{}, err
	}
	if record.ActorID != actorID {
		return pipeline.RunRecord{}, pipeline.ErrRunConflict
	}
	return record, nil
}

func (s *AIL0Service) Extract(ctx context.Context, actorID int64, request pipeline.RunRequest) (pipeline.RunRecord, error) {
	if actorID <= 0 {
		return pipeline.RunRecord{}, pipeline.ErrInvalidRun
	}
	if err := request.Validate(); err != nil {
		return pipeline.RunRecord{}, err
	}
	if s.runner == nil {
		return pipeline.RunRecord{}, pipeline.ErrDisabled
	}
	select {
	case s.busy <- struct{}{}:
		defer func() { <-s.busy }()
	default:
		record, err := s.RunByRequest(ctx, actorID, request.RequestID)
		if err == nil {
			if record.Request != request {
				return pipeline.RunRecord{}, pipeline.ErrRunConflict
			}
			return record, nil
		}
		return pipeline.RunRecord{}, pipeline.ErrRunBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	record, started, err := s.store.BeginRun(ctx, request, actorID, s.policy)
	if err != nil {
		return pipeline.RunRecord{}, err
	}
	if !started {
		return record, nil
	}
	snapshot, err := s.snapshots.SnapshotForExtraction(ctx, request.SnapshotID)
	var result pipeline.ExtractionResult
	if err == nil {
		result, err = s.runner.Run(pipeline.WithRunID(ctx, record.ID), snapshot)
	}
	failure := ""
	if err != nil {
		failure = "extraction_failed"
		if errors.Is(err, pipeline.ErrAuditUnavailable) {
			failure = "audit_unavailable"
		}
	}
	// 请求取消仍落终态；落库失败时保留 running，不会在重放中再次调用模型。
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()
	finished, finishErr := s.store.FinishRun(finishCtx, record.ID, result, failure)
	if finishErr != nil {
		return record, fmt.Errorf("抽取结果待核对，请保留原请求编号: %w", pipeline.ErrRunBusy)
	}
	return finished, nil
}
