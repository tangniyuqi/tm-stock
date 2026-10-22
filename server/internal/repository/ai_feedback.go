package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
)

type AIFeedbackRepo struct {
	db        *sql.DB
	isolation sql.IsolationLevel
}

func NewAIFeedbackRepo(db *sql.DB) *AIFeedbackRepo {
	return NewAIFeedbackRepoWithIsolation(db, sql.LevelReadCommitted)
}
func NewAIFeedbackRepoWithIsolation(db *sql.DB, level sql.IsolationLevel) *AIFeedbackRepo {
	return &AIFeedbackRepo{db, level}
}
func (r *AIFeedbackRepo) Within(ctx context.Context, fn func(feedback.Tx) error) error {
	if r == nil || r.db == nil || fn == nil || r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead {
		return feedback.ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return fmt.Errorf("开启反馈事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 已提交时的ErrTxDone不改变结果，其他出口全部回滚。
	if err = fn(&aiFeedbackTx{tx}); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("反馈提交结果未知，须查询原报告类别: %w", err)
	}
	return nil
}

type aiFeedbackTx struct{ tx *sql.Tx }

func (t *aiFeedbackTx) Now(ctx context.Context) (time.Time, error) { return (&pointsTx{t.tx}).Now(ctx) }
func (t *aiFeedbackTx) OwnedReport(ctx context.Context, uid, id int64) (string, error) {
	// 与报告路径同序：先任务，再报告；同报告受理串行，避免RR下空类别间隙锁升级。
	var status int
	if err := t.tx.QueryRowContext(ctx, `SELECT status FROM tm_ai_tasks WHERE id=? AND user_id=? FOR UPDATE`, id, uid).Scan(&status); err != nil {
		return "", feedbackDBError(err)
	}
	if status != 3 {
		return "", feedback.ErrNoReport
	}
	var digest string
	err := t.tx.QueryRowContext(ctx, `SELECT content_digest FROM tm_ai_report_commits WHERE task_id=? AND user_id=? AND status=3 FOR SHARE`, id, uid).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return "", feedback.ErrNoReport
	}
	return digest, feedbackDBError(err)
}
func feedbackDBError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return feedback.ErrNotFound
	}
	return err
}

const feedbackColumns = `id,task_id,user_id,category,report_digest,status,resolution,version,CAST(created_at AS CHAR),CAST(updated_at AS CHAR)`

func scanFeedback(s pointsScanner) (feedback.Record, error) {
	var r feedback.Record
	var created, updated string
	if err := s.Scan(&r.ID, &r.TaskID, &r.UserID, &r.Category, &r.ReportDigest, &r.Status, &r.Resolution, &r.Version, &created, &updated); err != nil {
		return r, feedbackDBError(err)
	}
	var err error
	r.CreatedAt, err = parsePointsUTC(created)
	if err != nil {
		return r, err
	}
	r.UpdatedAt, err = parsePointsUTC(updated)
	if err != nil {
		return r, err
	}
	if !r.Valid() {
		return feedback.Record{}, feedback.ErrInvariant
	}
	return r, nil
}
func (t *aiFeedbackTx) ByCategory(ctx context.Context, uid, id int64, category string) (feedback.Record, error) {
	return scanFeedback(t.tx.QueryRowContext(ctx, `SELECT `+feedbackColumns+` FROM tm_ai_report_feedbacks WHERE user_id=? AND task_id=? AND category=? FOR UPDATE`, uid, id, category))
}
func (t *aiFeedbackTx) ByID(ctx context.Context, id int64) (feedback.Record, error) {
	return scanFeedback(t.tx.QueryRowContext(ctx, `SELECT `+feedbackColumns+` FROM tm_ai_report_feedbacks WHERE id=? FOR UPDATE`, id))
}
func (t *aiFeedbackTx) Insert(ctx context.Context, r feedback.Record) (feedback.Record, error) {
	x, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_report_feedbacks(task_id,user_id,category,report_digest,status,resolution,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, r.TaskID, r.UserID, r.Category, r.ReportDigest, r.Status, r.Resolution, r.Version, pointsUTC(r.CreatedAt), pointsUTC(r.UpdatedAt))
	if err != nil {
		return r, err
	}
	r.ID, err = x.LastInsertId()
	return r, err
}
func (t *aiFeedbackTx) Update(ctx context.Context, old, next feedback.Record) error {
	r, err := t.tx.ExecContext(ctx, `UPDATE tm_ai_report_feedbacks SET status=?,resolution=?,version=?,updated_at=? WHERE id=? AND version=?`, next.Status, next.Resolution, next.Version, pointsUTC(next.UpdatedAt), old.ID, old.Version)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err == nil && n != 1 {
		return feedback.ErrConflict
	}
	return err
}
func (t *aiFeedbackTx) Append(ctx context.Context, e feedback.Event) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO tm_ai_report_feedback_events(feedback_id,seq,actor_id,status,resolution,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, e.FeedbackID, e.Version, e.ActorID, e.Status, e.Resolution, pointsUTC(e.At), pointsUTC(e.At))
	return err
}
func (t *aiFeedbackTx) History(ctx context.Context, id int64) ([]feedback.Event, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT feedback_id,seq,actor_id,status,resolution,CAST(created_at AS CHAR) FROM tm_ai_report_feedback_events WHERE feedback_id=? ORDER BY seq LIMIT 4 FOR SHARE`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []feedback.Event{}
	for rows.Next() {
		var e feedback.Event
		var raw string
		if err = rows.Scan(&e.FeedbackID, &e.Version, &e.ActorID, &e.Status, &e.Resolution, &raw); err != nil {
			return nil, err
		}
		e.At, err = parsePointsUTC(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func feedbackRows(rows *sql.Rows, err error) ([]feedback.Record, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []feedback.Record{}
	for rows.Next() {
		r, err := scanFeedback(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (t *aiFeedbackTx) ForReport(ctx context.Context, uid, id int64) ([]feedback.Record, error) {
	return feedbackRows(t.tx.QueryContext(ctx, `SELECT `+feedbackColumns+` FROM tm_ai_report_feedbacks WHERE user_id=? AND task_id=? ORDER BY id LIMIT 5 FOR UPDATE`, uid, id))
}
func (t *aiFeedbackTx) List(ctx context.Context, after int64, limit int) ([]feedback.Record, error) {
	return feedbackRows(t.tx.QueryContext(ctx, `SELECT `+feedbackColumns+` FROM tm_ai_report_feedbacks WHERE id>? ORDER BY id LIMIT ? FOR SHARE`, after, limit))
}
