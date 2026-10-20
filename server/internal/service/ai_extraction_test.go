package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type fakeExtractionSnapshotReader struct {
	snapshot sources.Snapshot
	err      error
	calls    int
}

func (r *fakeExtractionSnapshotReader) SnapshotForExtraction(_ context.Context, id int64) (sources.Snapshot, error) {
	r.calls++
	if r.err != nil {
		return sources.Snapshot{}, r.err
	}
	if id != r.snapshot.ID {
		return sources.Snapshot{}, errors.New("wrong snapshot")
	}
	return r.snapshot, nil
}

type fakeExtractionRunner struct {
	result pipeline.ExtractionResult
	err    error
	calls  int
}

func (r *fakeExtractionRunner) Run(_ context.Context, _ sources.Snapshot) (pipeline.ExtractionResult, error) {
	r.calls++
	return r.result, r.err
}

type fakeCandidateWriter struct {
	items []candidate.Candidate
	err   error
	calls int
}

func (w *fakeCandidateWriter) SavePendingCandidates(_ context.Context, items []candidate.Candidate) ([]candidate.Candidate, error) {
	w.calls++
	if w.err != nil {
		return nil, w.err
	}
	w.items = append([]candidate.Candidate(nil), items...)
	for i := range w.items {
		w.items[i].ID = int64(i + 100)
	}
	return w.items, nil
}

func TestAIExtractionServicePersistsOnlyCandidates(t *testing.T) {
	reader := &fakeExtractionSnapshotReader{snapshot: sources.Snapshot{ID: 42}}
	runner := &fakeExtractionRunner{result: pipeline.ExtractionResult{
		Attempts: 1, Model: "approved-model-v1",
		Candidates: []candidate.Candidate{{SnapshotID: 42, ClaimIndex: 0}},
	}}
	writer := &fakeCandidateWriter{}
	service, err := NewAIExtractionService(reader, runner, writer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ExtractSnapshot(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || runner.calls != 1 || writer.calls != 1 || len(result.Candidates) != 1 || result.Candidates[0].ID != 100 {
		t.Fatalf("snapshot extraction was not persisted as pending: result=%+v reader=%d runner=%d writer=%d", result, reader.calls, runner.calls, writer.calls)
	}
}

func TestAIExtractionServiceChecksAuthorizationBeforeProvider(t *testing.T) {
	reader := &fakeExtractionSnapshotReader{err: errors.New("source revoked")}
	runner := &fakeExtractionRunner{}
	writer := &fakeCandidateWriter{}
	service, err := NewAIExtractionService(reader, runner, writer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExtractSnapshot(context.Background(), 42); err == nil {
		t.Fatal("revoked source must stop extraction")
	}
	if runner.calls != 0 || writer.calls != 0 {
		t.Fatalf("revoked source reached provider or queue: runner=%d writer=%d", runner.calls, writer.calls)
	}
}

func TestAIExtractionServiceSkipsEmptyQueue(t *testing.T) {
	reader := &fakeExtractionSnapshotReader{snapshot: sources.Snapshot{ID: 42}}
	runner := &fakeExtractionRunner{}
	writer := &fakeCandidateWriter{}
	service, _ := NewAIExtractionService(reader, runner, writer)
	if _, err := service.ExtractSnapshot(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if writer.calls != 0 {
		t.Fatalf("empty extraction should not write a queue batch")
	}
}
