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

type AIDossierCorrectionSchemaVerifier struct{ db *sql.DB }

func NewAIDossierCorrectionSchemaVerifier(db *sql.DB) *AIDossierCorrectionSchemaVerifier {
	return &AIDossierCorrectionSchemaVerifier{db}
}
func (v *AIDossierCorrectionSchemaVerifier) Verify(parent context.Context) error {
	if v == nil || v.db == nil || parent == nil {
		return dossier.ErrInvalidManagement
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables t JOIN information_schema.table_constraints c ON c.constraint_schema=t.table_schema AND c.table_name=t.table_name WHERE t.table_schema=DATABASE() AND t.table_name='tm_ai_dossier_correction_events' AND t.engine='InnoDB' AND c.constraint_name='chk_tm_ai_dossier_correction_event' AND c.enforced='YES' AND c.constraint_type='CHECK'`).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return dossier.ErrManagementEvidence
	}
	for _, column := range []string{"request_id|varchar(64)|ascii_bin", "actor_id|bigint unsigned|", "theme_id|bigint unsigned|", "before_version|bigint unsigned|", "after_version|bigint unsigned|", "before_digest|char(64)|ascii_bin", "after_digest|char(64)|ascii_bin", "revision|bigint unsigned|", "canonical|mediumtext|utf8mb4_bin", "content_hash|char(64)|ascii_bin", "created_at|datetime(3)|", "updated_at|datetime(3)|"} {
		f := strings.Split(column, "|")
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='tm_ai_dossier_correction_events' AND column_name=? AND column_type=? AND COALESCE(collation_name,'')=? AND is_nullable='NO'`, f[0], f[1], f[2]).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return dossier.ErrManagementEvidence
		}
	}
	for _, index := range []struct{ name, columns string }{{"PRIMARY", "request_id"}, {"uq_tm_ai_dossier_correction_revision", "theme_id,before_version,after_version,revision"}} {
		var columns string
		if err = tx.QueryRowContext(ctx, `SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='tm_ai_dossier_correction_events' AND index_name=? AND non_unique=0 GROUP BY index_name`, index.name).Scan(&columns); err != nil {
			return err
		}
		if columns != index.columns {
			return dossier.ErrManagementEvidence
		}
	}
	for _, side := range []string{"before", "after"} {
		var columns, refs string
		if err = tx.QueryRowContext(ctx, `SELECT GROUP_CONCAT(column_name ORDER BY ordinal_position SEPARATOR ','),GROUP_CONCAT(referenced_column_name ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema=DATABASE() AND table_name='tm_ai_dossier_correction_events' AND constraint_name=? AND referenced_table_name='tm_ai_dossier_versions' GROUP BY constraint_name`, "fk_tm_ai_dossier_correction_"+side).Scan(&columns, &refs); err != nil {
			return err
		}
		if columns != "theme_id,"+side+"_version,"+side+"_digest" || refs != "theme_id,version_no,digest" {
			return dossier.ErrManagementEvidence
		}
	}
	for _, trigger := range []struct{ event, message string }{{"UPDATE", "更正声明审计不可修改"}, {"DELETE", "更正声明审计不可删除"}} {
		var table, event, timing, body string
		if err = tx.QueryRowContext(ctx, `SELECT event_object_table,event_manipulation,action_timing,action_statement FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name=?`, "tm_ai_dossier_correction_no_"+strings.ToLower(trigger.event)).Scan(&table, &event, &timing, &body); err != nil {
			return err
		}
		expected := "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='" + trigger.message + "'"
		if table != "tm_ai_dossier_correction_events" || event != trigger.event || timing != "BEFORE" || qaCompactSQL(body) != qaCompactSQL(expected) {
			return dossier.ErrManagementEvidence
		}
	}
	if err = probeDossierCorrection(ctx, tx); err != nil {
		return err
	}
	return tx.Rollback()
}
func probeDossierCorrection(ctx context.Context, tx *sql.Tx) error {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return err
	}
	theme := int64(3000000000000 + binary.BigEndian.Uint64(seed[:8])%1000000000000)
	a := &aiDossierCorrectionTx{&aiDossierManagementTx{&aiDossierTx{tx: tx, write: true}}, true}
	now, err := a.Now(ctx)
	if err != nil {
		return err
	}
	digests := []string{}
	for version := int64(1); version <= 2; version++ {
		raw := []byte(fmt.Sprintf(`{"schemaProbe":"explicit-correction","version":%d}`, version))
		digest := task.StageDigest(raw)
		digests = append(digests, digest)
		if err = a.Insert(ctx, dossier.StoredVersion{Identity: dossier.DraftIdentity{ThemeID: theme, Version: version, CreatedAt: now.UnixMilli() - 3 + version}, Digest: digest, Canonical: raw}); err != nil {
			return err
		}
	}
	p := dossier.CorrectionPair{ThemeID: theme, BeforeVersion: 1, AfterVersion: 2}
	if _, err = a.ManagementHead(ctx, dossier.ManagementObject{Kind: "publication", ThemeID: theme, Version: 2}, true); err != nil {
		return err
	}
	r := dossier.CorrectionRequest{RequestID: hex.EncodeToString(seed[:]), Object: p, BeforeDigest: digests[0], AfterDigest: digests[1], Enabled: true, ValidUntil: now.Add(time.Minute).UnixMilli(), Declarations: []dossier.CorrectionDeclaration{{Before: dossier.CorrectionLocator{Key: "test_old"}, After: dossier.CorrectionLocator{Key: "test_new"}, StatementKey: "test_declaration"}}, ReviewReference: "test-fixture:correction-schema"}
	e := dossier.CorrectionEvent{Protocol: dossier.CorrectionProtocol, ActorID: 1, Request: r, Revision: 1, CreatedAt: now.UnixMilli()}
	raw, _, err := e.Canonical()
	if err != nil {
		return err
	}
	if err = qaProbeReject(tx, ctx, 3819, `INSERT INTO tm_ai_dossier_correction_events(request_id,actor_id,theme_id,before_version,after_version,before_digest,after_digest,revision,canonical,content_hash,created_at,updated_at) VALUES(?,1,?,1,2,?,?,1,?,?,?,?)`, r.RequestID, theme, r.BeforeDigest, r.AfterDigest, raw, strings.Repeat("0", 64), pointsUTC(now), pointsUTC(now)); err != nil {
		return err
	}
	if err = a.AppendCorrection(ctx, e); err != nil {
		return err
	}
	if locked, e1 := a.CorrectionReadLock(ctx, p); e1 != nil || !locked {
		return dossier.ErrManagementEvidence
	}
	if _, _, err = a.LatestCorrection(ctx, p); err != nil {
		return err
	}
	if _, err = a.CorrectionEvent(ctx, r.RequestID); err != nil {
		return err
	}
	for _, q := range []string{`UPDATE tm_ai_dossier_correction_events SET actor_id=actor_id WHERE request_id=?`, `DELETE FROM tm_ai_dossier_correction_events WHERE request_id=?`} {
		if err = qaProbeReject(tx, ctx, 1644, q, r.RequestID); err != nil {
			return err
		}
	}
	return nil
}
