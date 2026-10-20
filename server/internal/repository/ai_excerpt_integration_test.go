//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const excerptMigration = "20261011_add_theme_stock_excerpt_trimmed.sql"

func TestIntegrationExcerptMigrationPreservesDataAndRejectsUnknownShapes(t *testing.T) {
	e := newConvergeEnv(t)
	path := filepath.Join("..", "..", "migrations", excerptMigration)
	if err := e.execFile(path); err == nil {
		t.Fatal("未建归属表也执行了节选迁移")
	}
	e.mustExecFile(filepath.Join("..", "..", "migrations", convergeOriginalMig))
	e.mustExec(`INSERT INTO addon_quant_theme_stock(theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status)
 VALUES(1,1,'688001.SH',2,'测试：公司披露主营算力服务。','https://example.test/report','2026-10-10 00:00:00.123',2,1)`)
	e.mustExecFile(path)
	e.mustExecFile(path)
	if e.queryInt("SELECT excerpt_trimmed FROM addon_quant_theme_stock") != 0 || e.queryString("SELECT source_excerpt FROM addon_quant_theme_stock") != "测试：公司披露主营算力服务。" || e.queryInt("SELECT audit_status FROM addon_quant_theme_stock") != 2 {
		t.Fatal("增量迁移改写了历史依据或状态")
	}
	for _, v := range []any{2, -1, nil} {
		err := e.exec("UPDATE addon_quant_theme_stock SET excerpt_trimmed = ?", v)
		var me *mysqldrv.MySQLError
		if !errors.As(err, &me) || (me.Number != 3819 && me.Number != 1048) {
			t.Fatalf("非法标记未被库约束拒绝: %v", err)
		}
	}
	e.mustExec("UPDATE addon_quant_theme_stock SET excerpt_trimmed=1")
	e.mustExecFile(filepath.Join("..", "..", "migrations", convergeMig))
	e.mustExecFile(path)
	if e.queryInt("SELECT excerpt_trimmed FROM addon_quant_theme_stock") != 1 {
		t.Fatal("重跑共表收敛丢失节选标记")
	}
	e.mustExec("ALTER TABLE addon_quant_theme_stock DROP CHECK chk_theme_stock_excerpt_trimmed")
	e.mustExec("ALTER TABLE addon_quant_theme_stock ADD CONSTRAINT chk_theme_stock_excerpt_trimmed CHECK (excerpt_trimmed >= 0)")
	if err := e.execFile(path); err == nil {
		t.Fatal("错误的同名约束未中止迁移")
	}
	e.mustExec("ALTER TABLE addon_quant_theme_stock DROP CHECK chk_theme_stock_excerpt_trimmed")
	e.mustExec("ALTER TABLE addon_quant_theme_stock MODIFY excerpt_trimmed varchar(8) NOT NULL DEFAULT '0'")
	if err := e.execFile(path); err == nil {
		t.Fatal("未知列类型被静默接受")
	}
}

// 真正入队并审核一条不连续引文，验证布尔列的 INSERT / Scan 和 C 端读路径，不调用模型。
func verifyTrimmedMappingRepository(t *testing.T, e *convergeEnv, db *sql.DB, base candidate.Candidate) {
	t.Helper()
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()
	repo := NewAISourceRepo(db)
	src, err := repo.RegisterSource(ctx, sources.Source{Domain: "excerpt.test", Category: sources.CategoryAnnualReport, LicenseTag: "test", LicenseRef: "test:written-authorization"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetSourceEnabled(ctx, src.ID, true); err != nil {
		t.Fatal(err)
	}
	src, err = repo.SourceByID(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	body := base.EntityKey + "在年报中披露主营算力服务。公司披露其他业务事项。算力服务包括数据中心运营。"
	snap, err := sources.BuildSnapshot(src, "https://excerpt.test/report", "text/plain; charset=utf-8", []byte(body), time.Now().UTC(), 4096)
	if err != nil {
		t.Fatal(err)
	}
	snap, err = repo.SaveSnapshot(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	c := base
	c.ID, c.SnapshotID, c.ThemeID, c.SourceURL, c.CollectedAt = 0, snap.ID, 200001, snap.URL, snap.FetchedAt
	c.SentenceIDs = []int{1, 3}
	raw, err := json.Marshal(struct {
		Claims []guard.Claim `json:"claims"`
	}{[]guard.Claim{{Predicate: c.Predicate, Entity: c.EntityKey, Theme: c.ThemeKey, SentenceIDs: c.SentenceIDs}}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := guard.ValidateExtraction(raw, guard.NewSnapshot(snap.Body), guard.Vocabulary{Predicates: map[string]bool{c.Predicate: true}, Themes: map[string][]string{c.ThemeKey: {c.ThemeKey}}})
	if err != nil || len(report.Accepted) != 1 {
		t.Fatalf("节选装置不合格: %+v %v", report, err)
	}
	a := report.Accepted[0]
	c.SourceExcerpt, c.QuoteSegments, c.ExcerptTrimmed = a.Quote, a.Segments, true
	e.mustExec("INSERT INTO addon_quant_theme(id,name,level,parent_id,status) VALUES(200001,?,1,0,1)", c.ThemeKey)
	saved, err := NewAIExtractionRepo(db).SavePendingCandidates(ctx, []candidate.Candidate{c})
	if err != nil {
		t.Fatal(err)
	}
	c = saved[0]
	digest, err := candidate.EvidenceDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	req := candidate.ReviewRequest{CandidateID: c.ID, EvidenceDigest: digest, Decision: "approve", RequestID: "test_mysql_excerpt_review_0001"}
	var mappingID int64
	err = NewAIReviewRepo(db).WithinReviewTx(ctx, func(tx candidate.ReviewTx) error {
		record, err := tx.LockCandidate(ctx, c.ID)
		if err != nil {
			return err
		}
		snapshot, err := tx.ApprovalSnapshot(ctx, record.Candidate)
		if err != nil {
			return err
		}
		if err := candidate.ValidateAgainstSnapshot(record.Candidate, snapshot); err != nil {
			return err
		}
		if _, err := tx.LockCatalog(ctx, record.Candidate); err != nil {
			return err
		}
		mappingID, err = tx.InsertMapping(ctx, record.Candidate, 7)
		if err != nil {
			return err
		}
		if _, err := tx.AppendAudit(ctx, req, 7, mappingID); err != nil {
			return err
		}
		return tx.RefreshThemeCount(ctx, c.ThemeID)
	})
	if err != nil {
		t.Fatal(err)
	}
	public := NewThemeRepo(db)
	m, err := public.FindVisibleMapping(ctx, c.ThemeID, c.TsCode)
	if err != nil || m == nil || !m.ExcerptTrimmed || m.SourceExcerpt != c.SourceExcerpt {
		t.Fatalf("C 端读回丢失节选: %+v %v", m, err)
	}
	list, err := public.ListVisibleMappings(ctx, []int64{c.ThemeID})
	if err != nil || len(list) != 1 || !list[0].ExcerptTrimmed {
		t.Fatalf("列表扫描丢失节选: %+v %v", list, err)
	}
	checkMapping := func(wantMatch bool) {
		t.Helper()
		if err := NewAIInspectionRepo(db).WithinInspectionTx(ctx, func(tx candidate.InspectionTx) error {
			current, err := tx.LockMapping(ctx, mappingID, c.ThemeID)
			if err != nil {
				return err
			}
			if current.Matches(c) != wantMatch {
				t.Fatal("复核未比较节选标记")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	checkMapping(true)
	e.mustExec("UPDATE addon_quant_theme_stock SET excerpt_trimmed=0 WHERE id=?", mappingID)
	checkMapping(false)
}
