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
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// 摘要对应20261103迁移的完整触发器体，使用原qaCompactSQL规范化；测试必须核迁移同源。
var dossierManagementTriggers = []struct{ name, table, event, digest string }{
	{"tm_ai_dossier_management_event_no_update", "tm_ai_dossier_management_events", "UPDATE", "a296446ed25c3ac55b33eccbc0a4f3ad9f9bc1a8fd7789fc53e2eb3017f9e1f8"},
	{"tm_ai_dossier_management_event_no_delete", "tm_ai_dossier_management_events", "DELETE", "ae388ac6b70cb1e0a3deff0834e6dc14b827e3d4442dcd06a87eb27b290ed6ae"},
	{"tm_ai_dossier_management_head_no_delete", "tm_ai_dossier_management_heads", "DELETE", "4abff19df2d1e7d2f6118066bcfa9d3fba313529181e0fe3fef2d04d3947f36a"},
	{"tm_ai_dossier_management_head_insert", "tm_ai_dossier_management_heads", "INSERT", "ab97b86f9ddb98ec0b0cd2934f6bdced289451584ecd73c660b4bca544dfe1bb"},
	{"tm_ai_dossier_management_head_update", "tm_ai_dossier_management_heads", "UPDATE", "430608f5226f3edd0d7f37f97089ea0058451716650afe86a431fc52fec3f37d"},
	{"tm_ai_dossier_managed_publication_insert", "tm_ai_dossier_publications", "INSERT", "b69551f1b6c81df013d6413498c58fbedfb559b870419ef56521dec9240a864c"},
	{"tm_ai_dossier_managed_publication_update", "tm_ai_dossier_publications", "UPDATE", "836ce3a4ed6e23ac7224c4c979cc21d0c11e945dcb16bcc3bc19f18d3122203e"},
	{"tm_ai_dossier_managed_publication_delete", "tm_ai_dossier_publications", "DELETE", "f40262d72397e0b28ab2f2c61a1963919a07f84ba75b6746e54ca1867ed2b5b0"},
	{"tm_ai_dossier_managed_grant_insert", "tm_ai_dossier_grants", "INSERT", "a1f6b6d71271748fbf56adf669216d47f05b2156c7d5b435b4103e8b3b2f94a6"},
	{"tm_ai_dossier_managed_grant_update", "tm_ai_dossier_grants", "UPDATE", "152f5d9a40c5a20083f89563a6d13bf1f70261de5ad216b3f41f7ae7c3fe1718"},
	{"tm_ai_dossier_managed_grant_delete", "tm_ai_dossier_grants", "DELETE", "6dafd3925243c8c2a10436e277601388acc9622b2a2334a4b9c03d038cd15468"},
}

type AIDossierManagementSchemaVerifier struct{ db *sql.DB }

func NewAIDossierManagementSchemaVerifier(db *sql.DB) *AIDossierManagementSchemaVerifier {
	return &AIDossierManagementSchemaVerifier{db}
}
func (v *AIDossierManagementSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if err := NewAIDossierSchemaVerifier(v.db).Verify(ctx); err != nil {
		return fmt.Errorf("专题管理基础结构未通过: %w", err)
	}
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }() // 准备身份仅做受控合成探针，所有出口回滚。
	if err = verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	for _, table := range []struct{ name, check string }{
		{"tm_ai_dossier_management_heads", "chk_tm_ai_dossier_management_head"}, {"tm_ai_dossier_management_events", "chk_tm_ai_dossier_management_event"},
	} {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name=? AND t.engine='InnoDB' AND c.constraint_name=? AND c.constraint_type='CHECK' AND c.enforced='YES'`, table.name, table.check).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return dossier.ErrInvalidManagement
		}
	}
	for _, table := range []struct {
		name    string
		columns []string
	}{
		{"tm_ai_dossier_management_heads", []string{"kind|varchar(16)|ascii_bin|NO", "user_id|bigint unsigned||NO", "theme_id|bigint unsigned||NO", "version_no|bigint unsigned||NO", "revision|bigint unsigned||NO", "request_id|varchar(64)|ascii_bin|YES"}},
		{"tm_ai_dossier_management_events", []string{"request_id|varchar(64)|ascii_bin|NO", "actor_id|bigint unsigned||NO", "kind|varchar(16)|ascii_bin|NO", "user_id|bigint unsigned||NO", "theme_id|bigint unsigned||NO", "version_no|bigint unsigned||NO", "version_digest|char(64)|ascii_bin|NO", "revision|bigint unsigned||NO", "canonical|mediumtext|utf8mb4_bin|NO", "content_hash|char(64)|ascii_bin|NO"}},
	} {
		for _, column := range append(table.columns, "created_at|datetime(3)||NO", "updated_at|datetime(3)||NO") {
			fields := strings.Split(column, "|")
			var n int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=? AND column_type=? AND COALESCE(collation_name,'')=? AND is_nullable=?`, table.name, fields[0], fields[1], fields[2], fields[3]).Scan(&n); err != nil {
				return err
			}
			if n != 1 {
				return dossier.ErrInvalidManagement
			}
		}
	}
	for _, index := range []struct{ table, name, columns string }{
		{"tm_ai_dossier_management_heads", "PRIMARY", "kind,user_id,theme_id,version_no"},
		{"tm_ai_dossier_management_events", "PRIMARY", "request_id"},
		{"tm_ai_dossier_management_events", "uq_tm_ai_dossier_management_revision", "kind,user_id,theme_id,version_no,revision"},
	} {
		var columns string
		if err = tx.QueryRowContext(ctx, `SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND non_unique=0 GROUP BY table_name,index_name`, index.table, index.name).Scan(&columns); err != nil {
			return err
		}
		if columns != index.columns {
			return dossier.ErrInvalidManagement
		}
	}
	for _, fk := range []struct{ table, name, columns, referenced string }{
		{"tm_ai_dossier_management_heads", "fk_tm_ai_dossier_management_head", "theme_id,version_no", "theme_id,version_no"},
		{"tm_ai_dossier_management_events", "fk_tm_ai_dossier_management_event", "theme_id,version_no,version_digest", "theme_id,version_no,digest"},
	} {
		var columns, referenced string
		if err = tx.QueryRowContext(ctx, `SELECT GROUP_CONCAT(column_name ORDER BY ordinal_position SEPARATOR ','),GROUP_CONCAT(referenced_column_name ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema=DATABASE() AND table_name=? AND constraint_name=? AND referenced_table_name='tm_ai_dossier_versions' GROUP BY table_name,constraint_name`, fk.table, fk.name).Scan(&columns, &referenced); err != nil {
			return err
		}
		if columns != fk.columns || referenced != fk.referenced {
			return dossier.ErrInvalidManagement
		}
	}
	for _, required := range dossierManagementTriggers {
		var table, event, timing, body string
		if err = tx.QueryRowContext(ctx, `SELECT event_object_table,event_manipulation,action_timing,action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name=?`, required.name).Scan(&table, &event, &timing, &body); err != nil {
			return err
		}
		if table != required.table || event != required.event || timing != "BEFORE" || task.StageDigest([]byte(qaCompactSQL(body))) != required.digest {
			return dossier.ErrInvalidManagement
		}
	}
	if err = probeDossierManagement(ctx, tx); err != nil {
		return err
	}
	return tx.Rollback()
}
func (r *AIDossierManagementRepo) VerifyAccess(parent context.Context) error {
	if r == nil || r.db == nil || parent == nil {
		return dossier.ErrInvalidManagement
	}
	if err := NewAIDossierRepo(r.db, sql.LevelReadCommitted).VerifyReadAccess(parent); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return dossierDBError(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	// 零行语句只测权限，不调用行触发器、不产生许可/审计或业务数据。
	for _, query := range []string{
		`SELECT kind,user_id,theme_id,version_no,revision,request_id,created_at,updated_at FROM tm_ai_dossier_management_heads WHERE 1=0`,
		`SELECT request_id,actor_id,kind,user_id,theme_id,version_no,version_digest,revision,canonical,content_hash,created_at,updated_at FROM tm_ai_dossier_management_events WHERE 1=0`,
	} {
		rows, e := tx.QueryContext(ctx, query)
		if e != nil {
			return e
		}
		if e = rows.Close(); e != nil {
			return e
		}
	}
	for _, table := range []struct {
		name, columns string
		update        bool
	}{
		{"tm_ai_dossier_management_heads", "kind,user_id,theme_id,version_no,revision,request_id,created_at,updated_at", true},
		{"tm_ai_dossier_management_events", "request_id,actor_id,kind,user_id,theme_id,version_no,version_digest,revision,canonical,content_hash,created_at,updated_at", false},
		{"tm_ai_dossier_publications", "theme_id,version_no,digest,approval_ref,enabled,valid_until,created_at,updated_at", true},
		{"tm_ai_dossier_grants", "user_id,theme_id,version_no,digest,approval_ref,enabled,valid_until,created_at,updated_at", true},
	} {
		if _, err = tx.ExecContext(ctx, "INSERT INTO "+table.name+"("+table.columns+") SELECT "+table.columns+" FROM "+table.name+" WHERE 1=0"); err != nil {
			return err
		}
		if table.update {
			column := "enabled"
			if table.name == "tm_ai_dossier_management_heads" {
				column = "revision"
			}
			if _, err = tx.ExecContext(ctx, "UPDATE "+table.name+" SET "+column+"="+column+" WHERE 1=0"); err != nil {
				return err
			}
		}
	}
	return tx.Rollback()
}
func probeDossierManagement(ctx context.Context, tx *sql.Tx) error {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return err
	}
	theme := int64(1000000000000 + binary.BigEndian.Uint64(seed[:8])%1000000000000)
	adapter := &aiDossierManagementTx{&aiDossierTx{tx: tx, write: true}}
	now, err := adapter.Now(ctx)
	if err != nil {
		return err
	}
	canonical := []byte("{\"schemaProbe\":\"dossier-management\"}")
	digest := task.StageDigest(canonical)
	if err = adapter.Insert(ctx, dossier.StoredVersion{Identity: dossier.DraftIdentity{ThemeID: theme, Version: 1, CreatedAt: now.UnixMilli()}, Digest: digest, Canonical: canonical}); err != nil {
		return err
	}
	for i, kind := range []string{"publication", "member_read"} {
		user := int64(0)
		if kind == "member_read" {
			user = 1
		}
		key := dossier.ManagementObject{Kind: kind, UserID: user, ThemeID: theme, Version: 1}
		h, e := adapter.ManagementHead(ctx, key, true)
		if e != nil {
			return e
		}
		at, e := adapter.Now(ctx)
		if e != nil {
			return e
		}
		request := dossier.ManagementRequest{RequestID: hex.EncodeToString(seed[:]) + fmt.Sprint(i), Object: key, VersionDigest: digest, Enabled: true, ValidUntil: at.Add(time.Minute).UnixMilli(), ReviewReference: "test-fixture:management-schema"}
		event := dossier.ManagementEvent{Protocol: dossier.ManagementProtocol, ActorID: 1, Request: request, After: dossier.ManagementState{Digest: digest, Reference: dossier.ManagedApprovalPrefix + request.RequestID, Enabled: true, ValidUntil: request.ValidUntil}, Revision: 1, CreatedAt: at.UnixMilli()}
		if e = adapter.AppendManagementEvent(ctx, event); e != nil {
			return e
		}
		if e = adapter.AdvanceManagementHead(ctx, key, h, event); e != nil {
			return e
		}
		if e = adapter.WritePermission(ctx, key, nil, event.After, event.CreatedAt); e != nil {
			return e
		}
		for _, probe := range []struct {
			query string
			args  []any
		}{
			{`UPDATE tm_ai_dossier_management_events SET actor_id=actor_id WHERE request_id=?`, []any{request.RequestID}},
			{`DELETE FROM tm_ai_dossier_management_events WHERE request_id=?`, []any{request.RequestID}},
			{`UPDATE tm_ai_dossier_management_heads SET revision=0,request_id=NULL WHERE kind=? AND user_id=? AND theme_id=? AND version_no=1`, []any{kind, user, theme}},
			{`DELETE FROM tm_ai_dossier_management_heads WHERE kind=? AND user_id=? AND theme_id=? AND version_no=1`, []any{kind, user, theme}},
		} {
			if e = qaProbeReject(tx, ctx, 1644, probe.query, probe.args...); e != nil {
				return e
			}
		}
		if kind == "publication" {
			for _, query := range []string{`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE theme_id=? AND version_no=1`, `UPDATE tm_ai_dossier_publications SET approval_ref='legacy-strip' WHERE theme_id=? AND version_no=1`, `DELETE FROM tm_ai_dossier_publications WHERE theme_id=? AND version_no=1`} {
				if e = qaProbeReject(tx, ctx, 1644, query, theme); e != nil {
					return e
				}
			}
		} else {
			for _, query := range []string{`UPDATE tm_ai_dossier_grants SET enabled=0 WHERE user_id=1 AND theme_id=? AND version_no=1`, `UPDATE tm_ai_dossier_grants SET approval_ref='legacy-strip' WHERE user_id=1 AND theme_id=? AND version_no=1`, `DELETE FROM tm_ai_dossier_grants WHERE user_id=1 AND theme_id=? AND version_no=1`} {
				if e = qaProbeReject(tx, ctx, 1644, query, theme); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
