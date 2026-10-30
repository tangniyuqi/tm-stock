package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

// VerifyReadAccess 检查读取依赖的列权限和共享锁语法，不读取正文、不写入。
// 此检查不替代结构约束/触发器验证，也不证明账号没有额外权限。
// 装配方必须在完整结构保护验证通过后调用；当前尚未开放公开路由。
func (r *AIDossierRepo) VerifyReadAccess(parent context.Context) error {
	if r == nil || r.db == nil || parent == nil || (r.isolation != sql.LevelReadCommitted && r.isolation != sql.LevelRepeatableRead) {
		return dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: r.isolation})
	if err != nil {
		return fmt.Errorf("专题读取权限检查事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 所有出口只回滚，无业务写入。
	if err := verifyPointsSession(ctx, tx); err != nil {
		return fmt.Errorf("专题读取会话配置失败: %w", err)
	}
	for i, query := range []string{
		`SELECT id,status,deleted_at FROM addon_member WHERE 1=0 FOR SHARE`,
		`SELECT id,status FROM addon_quant_theme WHERE 1=0 FOR SHARE`,
		`SELECT theme_id,version_no,digest,canonical,compiled_at FROM tm_ai_dossier_versions WHERE 1=0 FOR SHARE`,
		`SELECT theme_id,version_no,snapshot_id,source_id,content_hash FROM tm_ai_dossier_evidence WHERE 1=0 FOR SHARE`,
		`SELECT theme_id,version_no,digest,approval_ref,valid_until,enabled FROM tm_ai_dossier_publications WHERE 1=0 FOR SHARE`,
		`SELECT user_id,theme_id,version_no,digest,approval_ref,valid_until,enabled FROM tm_ai_dossier_grants WHERE 1=0 FOR SHARE`,
		`SELECT id,domain,category,license_tag,license_ref,license_expires_at,min_interval_ms,enabled FROM tm_ai_sources WHERE 1=0 FOR SHARE`,
		`SELECT s.id,s.source_id,s.category_snapshot,s.url,s.content_hash,b.body_text,b.sentences_json,s.license_tag_snapshot,s.license_ref_snapshot,s.license_expires_snapshot,s.fetched_at FROM tm_ai_source_snapshots s JOIN tm_ai_source_bodies b ON b.content_hash=s.body_ref WHERE 1=0`,
	} {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("专题读取权限检查项%d失败: %w", i, err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("专题读取权限检查项%d关闭失败: %w", i, err)
		}
	}
	return tx.Rollback()
}
