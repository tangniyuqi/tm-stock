package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

// AIDossierSchemaVerifier供迁移/启动准备账号运行，业务只读账号不获写入或TRIGGER权限。
// Verify只回滚；通过不代替运行账号VerifyReadAccess或逐请求授权。
type AIDossierSchemaVerifier struct{ db *sql.DB }

type AIDossierSchemaError struct {
	Stage int
	cause error
}

func (e *AIDossierSchemaError) Error() string {
	return fmt.Sprintf("专题结构保护检查阶段%d失败（详情已隐藏）", e.Stage)
}
func (e *AIDossierSchemaError) Unwrap() error { return e.cause }

func NewAIDossierSchemaVerifier(db *sql.DB) *AIDossierSchemaVerifier {
	return &AIDossierSchemaVerifier{db: db}
}

func (v *AIDossierSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return dossier.ErrInvalidDraft
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return &AIDossierSchemaError{Stage: 1, cause: err}
	}
	defer func() { _ = tx.Rollback() }() // 探针不提交，不接触会员、会话或积分。
	for i, check := range []func(context.Context, *sql.Tx) error{verifyPointsSession, verifyDossierMetadata, probeDossier} {
		if err := check(ctx, tx); err != nil {
			return &AIDossierSchemaError{Stage: i + 2, cause: err}
		}
	}
	return tx.Rollback()
}

func verifyDossierMetadata(ctx context.Context, tx *sql.Tx) error {
	for _, c := range []struct {
		table, check string
		columns      []string
	}{
		{"tm_ai_dossier_versions", "chk_tm_ai_dossier_version", []string{"theme_id|bigint unsigned|", "version_no|bigint unsigned|", "digest|char(64)|ascii_bin", "canonical|mediumtext|utf8mb4_bin", "compiled_at|datetime(3)|"}},
		{"tm_ai_dossier_evidence", "chk_tm_ai_dossier_evidence", []string{"theme_id|bigint unsigned|", "version_no|bigint unsigned|", "snapshot_id|bigint unsigned|", "source_id|bigint unsigned|", "content_hash|char(64)|ascii_bin"}},
		{"tm_ai_dossier_publications", "chk_tm_ai_dossier_publication", []string{"theme_id|bigint unsigned|", "version_no|bigint unsigned|", "digest|char(64)|ascii_bin", "approval_ref|varchar(128)|utf8mb4_bin", "enabled|tinyint|", "valid_until|datetime(3)|"}},
		{"tm_ai_dossier_grants", "chk_tm_ai_dossier_grant", []string{"user_id|bigint unsigned|", "theme_id|bigint unsigned|", "version_no|bigint unsigned|", "digest|char(64)|ascii_bin", "approval_ref|varchar(128)|utf8mb4_bin", "enabled|tinyint|", "valid_until|datetime(3)|"}},
	} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, c.table, c.check).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return dossier.ErrInvalidDraft
		}
		for _, column := range append(c.columns, "created_at|datetime(3)|", "updated_at|datetime(3)|") {
			f := strings.Split(column, "|")
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND is_nullable='NO' AND COALESCE(collation_name,'')=?`, c.table, f[0], f[1], f[2]).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return dossier.ErrInvalidDraft
			}
		}
	}
	for _, index := range []struct{ table, name, columns string }{
		{"tm_ai_dossier_versions", "PRIMARY", "theme_id,version_no"},
		{"tm_ai_dossier_versions", "uq_tm_ai_dossier_digest", "theme_id,version_no,digest"},
		{"tm_ai_dossier_evidence", "PRIMARY", "theme_id,version_no,snapshot_id"},
		{"tm_ai_dossier_publications", "PRIMARY", "theme_id,version_no"},
		{"tm_ai_dossier_grants", "PRIMARY", "user_id,theme_id,version_no"},
	} {
		var got string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(COALESCE(column_name,'#expression'),IF(sub_part IS NULL,'',':prefix'),IF(collation='A','',':order')) ORDER BY seq_in_index SEPARATOR ','),'') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=0 AND is_visible='YES'`, index.table, index.name).Scan(&got); err != nil {
			return err
		}
		if got != index.columns {
			return dossier.ErrInvalidDraft
		}
	}
	for _, fk := range []struct{ table, name, parent, columns string }{
		{"tm_ai_dossier_evidence", "fk_tm_ai_dossier_evidence_version", "tm_ai_dossier_versions", "theme_id:theme_id,version_no:version_no"},
		{"tm_ai_dossier_evidence", "fk_tm_ai_dossier_evidence_snapshot", "tm_ai_source_snapshots", "snapshot_id:id"},
		{"tm_ai_dossier_evidence", "fk_tm_ai_dossier_evidence_source", "tm_ai_sources", "source_id:id"},
		{"tm_ai_dossier_publications", "fk_tm_ai_dossier_publication", "tm_ai_dossier_versions", "theme_id:theme_id,version_no:version_no,digest:digest"},
		{"tm_ai_dossier_grants", "fk_tm_ai_dossier_grant", "tm_ai_dossier_versions", "theme_id:theme_id,version_no:version_no,digest:digest"},
	} {
		var got string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(GROUP_CONCAT(CONCAT(k.column_name,':',k.referenced_column_name) ORDER BY k.ordinal_position SEPARATOR ','),'') FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name WHERE k.constraint_schema=DATABASE() AND k.table_name=? AND k.constraint_name=? AND k.referenced_table_schema=DATABASE() AND k.referenced_table_name=? AND r.update_rule IN ('RESTRICT','NO ACTION') AND r.delete_rule IN ('RESTRICT','NO ACTION')`, fk.table, fk.name, fk.parent).Scan(&got); err != nil {
			return err
		}
		if got != fk.columns {
			return dossier.ErrInvalidDraft
		}
	}
	for _, trigger := range []struct{ name, table, event, message string }{
		{"tm_ai_dossier_versions_no_update", "tm_ai_dossier_versions", "UPDATE", "专题版本不可修改"},
		{"tm_ai_dossier_versions_no_delete", "tm_ai_dossier_versions", "DELETE", "专题版本不可删除"},
		{"tm_ai_dossier_evidence_no_update", "tm_ai_dossier_evidence", "UPDATE", "专题依据不可修改"},
		{"tm_ai_dossier_evidence_no_delete", "tm_ai_dossier_evidence", "DELETE", "专题依据不可删除"},
	} {
		var table, event, timing, body string
		if err := tx.QueryRowContext(ctx, `SELECT event_object_table,event_manipulation,action_timing,action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name=?`, trigger.name).Scan(&table, &event, &timing, &body); err != nil {
			return err
		}
		want := "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='" + trigger.message + "'"
		if table != trigger.table || event != trigger.event || timing != "BEFORE" || qaCompactSQL(body) != qaCompactSQL(want) {
			return dossier.ErrInvalidDraft
		}
	}
	for _, table := range []string{"tm_ai_dossier_publications", "tm_ai_dossier_grants"} {
		var value string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(column_default,'#null') FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='enabled'`, table).Scan(&value); err != nil {
			return err
		}
		if value != "0" {
			return dossier.ErrInvalidDraft
		}
	}
	return nil
}

func probeDossier(ctx context.Context, tx *sql.Tx) error {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return err
	}
	id := int64(binary.BigEndian.Uint64(seed[:8])&((1<<50)-1)) + 1
	domain := "probe-" + hex.EncodeToString(seed[:]) + ".invalid"
	// 随机隔离主键与来源，不读取或修改已有业务行；唯一键冲突即拒绝本次启动检查。
	result, err := tx.ExecContext(ctx, `INSERT INTO tm_ai_sources(domain,category,license_tag,license_ref,min_interval_ms,enabled) VALUES(?,'official_catalog','仅测试','test-fixture:dossier-probe',1000,0)`, domain)
	if err != nil {
		return err
	}
	source, err := result.LastInsertId()
	if err != nil {
		return err
	}
	// 正文哈希也随机，防止碰撞既有测试或用户资料。
	hash := hex.EncodeToString(seed[:]) + hex.EncodeToString(seed[:])
	if _, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_source_bodies(content_hash,body_text,sentences_json) VALUES(?,'仅测试：结构保护探针。','[]')`, hash); err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_source_snapshots(source_id,category_snapshot,url,url_hash,content_hash,body_ref,license_tag_snapshot,license_ref_snapshot,fetched_at) VALUES(?,'official_catalog',?,UNHEX(SHA2(?,256)),?,?,'仅测试','test-fixture:dossier-probe',UTC_TIMESTAMP(3))`, source, "https://"+domain, "https://"+domain, hash, hash)
	if err != nil {
		return err
	}
	snapshot, err := result.LastInsertId()
	if err != nil {
		return err
	}
	const version = `INSERT INTO tm_ai_dossier_versions(theme_id,version_no,digest,canonical,compiled_at,created_at,updated_at) VALUES(?,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`
	if _, err = tx.ExecContext(ctx, version, id, 1, hash, "{}"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_evidence VALUES(?,1,?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, snapshot, source, hash); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_publications(theme_id,version_no,digest,approval_ref,valid_until,created_at,updated_at) VALUES(?,1,?,'test-fixture:probe',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, hash); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO tm_ai_dossier_grants(user_id,theme_id,version_no,digest,approval_ref,valid_until,created_at,updated_at) VALUES(?,?,1,?,'test-fixture:probe',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, id, id, hash); err != nil {
		return err
	}
	var enabled int
	if err = tx.QueryRowContext(ctx, `SELECT p.enabled+g.enabled FROM tm_ai_dossier_publications p JOIN tm_ai_dossier_grants g ON g.theme_id=p.theme_id AND g.version_no=p.version_no WHERE p.theme_id=? AND g.user_id=?`, id, id).Scan(&enabled); err != nil {
		return err
	}
	if enabled != 0 {
		return dossier.ErrInvalidDraft
	}
	for _, args := range [][]any{{0, 1, hash, "{}"}, {int64(9007199254740992), 1, hash, "{}"}, {id, 0, hash, "{}"}, {id, int64(9007199254740992), hash, "{}"}, {id, 2, "bad", "{}"}, {id, 2, hash, "[]"}, {id, 2, hash, `{"p":"` + strings.Repeat("x", 262145) + `"}`}} {
		if err := qaProbeReject(tx, ctx, 3819, version, args...); err != nil {
			return err
		}
	}
	if err := qaProbeReject(tx, ctx, 1062, version, id, 1, hash, "{}"); err != nil {
		return err
	}
	for _, query := range []string{
		`INSERT INTO tm_ai_dossier_versions SELECT theme_id,2,digest,canonical,'1970-01-01',created_at,updated_at FROM tm_ai_dossier_versions WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_versions SELECT theme_id,2,digest,canonical,created_at+INTERVAL 1 SECOND,created_at,updated_at FROM tm_ai_dossier_versions WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_versions SELECT theme_id,2,digest,canonical,compiled_at,created_at,updated_at+INTERVAL 1 SECOND FROM tm_ai_dossier_versions WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_evidence SELECT theme_id,version_no,snapshot_id,source_id,'bad',created_at,updated_at FROM tm_ai_dossier_evidence WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_evidence SELECT theme_id,version_no,0,source_id,content_hash,created_at,updated_at FROM tm_ai_dossier_evidence WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_evidence SELECT theme_id,version_no,snapshot_id,0,content_hash,created_at,updated_at FROM tm_ai_dossier_evidence WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_evidence SELECT theme_id,version_no,snapshot_id,source_id,content_hash,'1970-01-01','1970-01-01' FROM tm_ai_dossier_evidence WHERE theme_id=?`,
		`INSERT INTO tm_ai_dossier_evidence SELECT theme_id,version_no,snapshot_id,source_id,content_hash,created_at,updated_at+INTERVAL 1 SECOND FROM tm_ai_dossier_evidence WHERE theme_id=?`,
	} {
		if err := qaProbeReject(tx, ctx, 3819, query, id); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`UPDATE tm_ai_dossier_versions SET canonical=canonical WHERE theme_id=?`,
		`DELETE FROM tm_ai_dossier_versions WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_evidence SET content_hash=content_hash WHERE theme_id=?`,
		`DELETE FROM tm_ai_dossier_evidence WHERE theme_id=?`,
	} {
		if err := qaProbeReject(tx, ctx, 1644, q, id); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`UPDATE tm_ai_dossier_publications SET digest=REPEAT('f',64) WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET digest=REPEAT('f',64) WHERE theme_id=?`,
	} {
		if err := qaProbeReject(tx, ctx, 1452, q, id); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`UPDATE tm_ai_dossier_publications SET enabled=2 WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_publications SET approval_ref=' ' WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_publications SET valid_until=created_at WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET enabled=2 WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET user_id=0 WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET user_id=9007199254740992 WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET approval_ref=' ' WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET valid_until=created_at WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_publications SET created_at='1970-01-01' WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_publications SET updated_at=created_at-INTERVAL 1 SECOND WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET created_at='1970-01-01' WHERE theme_id=?`,
		`UPDATE tm_ai_dossier_grants SET updated_at=created_at-INTERVAL 1 SECOND WHERE theme_id=?`,
	} {
		if err := qaProbeReject(tx, ctx, 3819, q, id); err != nil {
			return err
		}
	}
	return nil
}
