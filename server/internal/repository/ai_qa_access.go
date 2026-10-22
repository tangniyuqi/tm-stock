package repository

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var _ qa.AccessStore = (*AIQARepo)(nil)

func (r *AIQARepo) WithinAccess(ctx context.Context, fn func(qa.AccessTx) error) error {
	if fn == nil {
		return qa.ErrInvalid
	}
	return r.WithinQA(ctx, false, func(tx qa.Tx) error { return fn(tx.(*aiQATx)) })
}
func (t *aiQATx) Scope(ctx context.Context, kind string, id int64) (qa.Scope, string, error) {
	if kind != "theme" || id <= 0 {
		return qa.Scope{}, "", qa.ErrUnavailable
	}
	s := qa.Scope{SubjectType: kind, SubjectID: id}
	var policy, until string
	var refs []byte
	var enabled int
	err := t.tx.QueryRowContext(ctx, `SELECT subject_name,approval_ref,policy_digest,evidence_json,CAST(valid_until AS CHAR),enabled FROM tm_ai_qa_scopes WHERE subject_type=? AND subject_id=?`, kind, id).Scan(&s.SubjectName, &s.ApprovalRef, &policy, &refs, &until, &enabled)
	if err != nil {
		return qa.Scope{}, "", qaDBError(err)
	}
	if enabled != 1 {
		return qa.Scope{}, "", qa.ErrUnavailable
	}
	var evidence []qa.EvidenceProof
	if strictQAJSON(refs, &evidence) != nil || len(evidence) > 8 {
		return qa.Scope{}, "", qa.ErrProtocol
	}
	s.ValidUntil, err = parsePointsUTC(until)
	if err != nil {
		return qa.Scope{}, "", err
	}
	s.Snapshots, err = t.qaSnapshots(ctx, evidence)
	if err != nil {
		return qa.Scope{}, "", err
	}
	return s, policy, nil
}
func (t *aiQATx) ScopeForProof(ctx context.Context, p qa.Proof) (qa.Scope, error) {
	if !p.Valid() {
		return qa.Scope{}, qa.ErrInvalid
	}
	if err := t.Authorize(ctx, p); err != nil {
		return qa.Scope{}, err
	}
	rows, err := t.qaSnapshots(ctx, p.Evidence)
	if err != nil {
		return qa.Scope{}, err
	}
	return qa.Scope{SubjectType: p.SubjectType, SubjectID: p.SubjectID, SubjectName: p.SubjectName, ApprovalRef: p.ScopeApproval, ValidUntil: p.ValidUntil, Snapshots: rows}, nil
}
func (t *aiQATx) qaSnapshots(ctx context.Context, refs []qa.EvidenceProof) ([]sources.Snapshot, error) {
	if len(refs) > 8 {
		return nil, qa.ErrProtocol
	}
	out := []sources.Snapshot{}
	bytes, sentences := 0, 0
	for n, ref := range refs {
		if ref.ID <= 0 || n > 0 && ref.ID <= refs[n-1].ID {
			return nil, qa.ErrProtocol
		}
		s, err := sourceSnapshotByID(ctx, t.tx, ref.ID, false)
		if err != nil {
			return nil, err
		}
		bytes += len(s.Body)
		sentences += len(s.Sentences)
		if s.Hash != ref.Hash || bytes > 64<<10 || sentences > 512 {
			return nil, qa.ErrProtocol
		}
		out = append(out, s)
	}
	return out, nil
}
