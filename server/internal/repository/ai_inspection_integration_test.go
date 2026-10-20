//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 复用可丢弃测试库的真实授权/审核样本，验证 DAO 的锁、约束与原子性；不调用模型。
func verifyInspectionRepository(t *testing.T, e *convergeEnv, db *sql.DB, c candidate.Candidate, original candidate.AuditEvent) {
	t.Helper()
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()
	repo := NewAIInspectionRepo(db)
	create := func(id string) candidate.InspectionBatch {
		t.Helper()
		var b candidate.InspectionBatch
		err := repo.WithinInspectionTx(ctx, func(tx candidate.InspectionTx) error {
			window, err := tx.SamplingWindow(ctx, 0, 2000)
			if err != nil {
				return err
			}
			ids := make([]int64, 0, len(window))
			for _, a := range window {
				ids = append(ids, a.ID)
			}
			raw, err := json.Marshal(window)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			b = candidate.InspectionBatch{Request: candidate.InspectionBatchRequest{RequestID: id, WindowSize: 2000, SampleSize: 1}, ActorID: 8, PopulationCount: len(window), EligibleCount: len(window), PopulationReviewIDs: ids, NextReviewID: ids[len(ids)-1], PopulationDigest: hex.EncodeToString(sum[:])}
			b, err = tx.InsertBatch(ctx, b, []candidate.AuditEvent{original})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	b := create("test_mysql_inspection_batch_0001")
	loaded, found, err := repo.BatchByRequest(ctx, b.Request.RequestID)
	if err != nil || !found || len(loaded.Items) != 1 || len(loaded.PopulationReviewIDs) != loaded.PopulationCount {
		t.Fatalf("批次读取失败: %+v %v", loaded, err)
	}
	item, err := repo.ItemByID(ctx, b.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := candidate.InspectionDigest(item)
	if err != nil {
		t.Fatal(err)
	}
	// 来源已过期仍可观察、记录问题，但不得把它确认为当前有效依据。
	err = repo.WithinInspectionTx(ctx, func(tx candidate.InspectionTx) error {
		item, err := tx.LockItem(ctx, item.Selection.ID)
		if err != nil {
			return err
		}
		_, err = tx.ApprovalSnapshot(ctx, item.Candidate)
		return err
	})
	if !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("失效来源仍可确认: %v", err)
	}
	request := candidate.InspectionRequest{ItemID: item.Selection.ID, EvidenceDigest: digest, Decision: "issue", Reason: "测试：授权已失效，暂停展示", RequestID: "test_mysql_inspection_decision_0001"}
	decide := func(r candidate.InspectionRequest) error {
		return repo.WithinInspectionTx(ctx, func(tx candidate.InspectionTx) error {
			item, err := tx.LockItem(ctx, r.ItemID)
			if err != nil {
				return err
			}
			if previous, found, err := tx.EventByRequest(ctx, r.RequestID); err != nil {
				return err
			} else if found {
				if previous.Request == r && previous.ActorID == 8 {
					return nil
				}
				return candidate.ErrConflict
			}
			if _, found, err := tx.EventByItem(ctx, r.ItemID); err != nil {
				return err
			} else if found {
				return candidate.ErrConflict
			}
			mapping, err := tx.LockMapping(ctx, item.Original.MappingID, item.Candidate.ThemeID)
			if err != nil {
				return err
			}
			if !mapping.Matches(c) {
				return candidate.ErrEvidenceChanged
			}
			if err := tx.Quarantine(ctx, mapping.ID, 8); err != nil {
				return err
			}
			if _, err := tx.AppendInspection(ctx, r, 8, mapping.ID); err != nil {
				return err
			}
			return tx.RefreshThemeCount(ctx, c.ThemeID)
		})
	}
	e.mustExec(`CREATE TRIGGER tm_ai_inspection_test_fail BEFORE INSERT ON tm_ai_inspection_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'test inspection failure'`)
	if err := decide(request); err == nil {
		t.Fatal("复核审计故障没有中止")
	}
	e.mustExec("DROP TRIGGER tm_ai_inspection_test_fail")
	if e.queryInt("SELECT status FROM addon_quant_theme_stock WHERE id=?", original.MappingID) != 1 || e.queryInt("SELECT stock_count FROM addon_quant_theme WHERE id=?", c.ThemeID) != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_inspection_events") != 0 {
		t.Fatal("暂停失败没有整体回滚")
	}
	if err := decide(request); err != nil {
		t.Fatal(err)
	}
	if err := decide(request); err != nil {
		t.Fatalf("复核重放失败: %v", err)
	}
	if e.queryInt("SELECT status FROM addon_quant_theme_stock WHERE id=?", original.MappingID) != 0 || e.queryInt("SELECT stock_count FROM addon_quant_theme WHERE id=?", c.ThemeID) != 0 || e.queryInt("SELECT COUNT(*) FROM tm_ai_review_audits WHERE id=? AND decision='approve'", original.ID) != 1 {
		t.Fatal("问题暂停、计数或原审计错误")
	}
	issues, err := repo.ListIssues(ctx, 0, 20)
	if err != nil || len(issues) != 1 {
		t.Fatal("问题历史未返回")
	}
	finished, err := repo.ItemByID(ctx, item.Selection.ID)
	if err != nil || finished.Result == nil || finished.Result.Request != request {
		t.Fatal("条目终态无法回查")
	}
	for _, table := range []string{"tm_ai_inspection_batches", "tm_ai_inspection_items", "tm_ai_inspection_events"} {
		for _, query := range []string{"UPDATE " + table + " SET updated_at=UTC_TIMESTAMP(3)", "DELETE FROM " + table} {
			if _, err := db.ExecContext(ctx, query); err == nil {
				t.Fatalf("复核记录可以篡改: %s", table)
			}
		}
	}
	// 同一归属不同批次可再次抽查；同条目两种终态请求只允许一方提交。
	next := create("test_mysql_inspection_batch_0002")
	item, err = repo.ItemByID(ctx, next.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	digest, err = candidate.InspectionDigest(item)
	if err != nil {
		t.Fatal(err)
	}
	request.ItemID = item.Selection.ID
	request.EvidenceDigest = digest
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, id := range []string{"test_mysql_inspection_concurrent_1", "test_mysql_inspection_concurrent_2"} {
		r := request
		r.RequestID = id
		wg.Add(1)
		go func() { defer wg.Done(); outcomes <- decide(r) }()
	}
	wg.Wait()
	close(outcomes)
	ok := 0
	for err := range outcomes {
		if err == nil {
			ok++
		} else if !errors.Is(err, candidate.ErrConflict) {
			t.Fatal(err)
		}
	}
	if ok != 1 {
		t.Fatal("复核并发未互斥")
	}
}
