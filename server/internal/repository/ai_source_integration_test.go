//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestIntegrationAISourceRepositoryAndImmutableSnapshots(t *testing.T) {
	e := newConvergeEnv(t)
	cleanupAISourceArtifacts(t, e)
	t.Cleanup(func() { cleanupAISourceArtifacts(t, e) })
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_ai_source_snapshots.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_ai_source_snapshots.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_extraction_candidates.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_extraction_candidates.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_l0_extraction_runs.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_l0_extraction_runs.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_review_inspections.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_review_inspections.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_source_audits.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261010_create_tm_ai_source_audits.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", convergeOriginalMig))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261011_add_theme_stock_excerpt_trimmed.sql"))
	e.mustExecFile(filepath.Join("..", "..", "migrations", "20261011_add_theme_stock_excerpt_trimmed.sql"))
	e.mustExec("ALTER TABLE addon_quant_theme ADD COLUMN stock_count int DEFAULT 0")

	db, err := sql.Open("mysql", os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAISourceRepo(db)
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour).Truncate(time.Millisecond)
	created, err := repo.RegisterSource(ctx, sources.Source{
		Domain:         "EXAMPLE.TEST.",
		Category:       sources.CategoryAnnualReport,
		LicenseTag:     "integration-test",
		LicenseRef:     "test-fixture:written-authorization",
		LicenseExpires: &expiresAt,
		MinInterval:    time.Second,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("登记来源失败: %v", err)
	}
	if created.Enabled {
		t.Fatal("登记来源必须默认关闭")
	}
	if err := repo.SetSourceEnabled(ctx, created.ID, true); err != nil {
		t.Fatalf("启用已登记授权来源失败: %v", err)
	}
	created, err = repo.SourceByID(ctx, created.ID)
	if err != nil || !created.Enabled {
		t.Fatalf("未读到数据库实际启用状态: %v", err)
	}
	if err := repo.ValidateEnabledSources(ctx); err != nil {
		t.Fatalf("有效来源未通过启动校验: %v", err)
	}
	if _, err := sources.NewService(ctx, repo, sources.NewFetcher(), true); err != nil {
		t.Fatalf("有效来源下生产服务无法初始化: %v", err)
	}
	e.mustExec(`INSERT INTO addon_quant_theme (id, name, level, parent_id, status, created_at, updated_at)
	  VALUES (100001, '算力', 1, 0, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))`)
	e.mustExec(`INSERT INTO addon_quant_base_stock (id, ts_code, symbol, name, list_status, created_at, updated_at)
	  VALUES (1, '688001.SH', '688001', '星河科技', 'L', UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))`)

	snapshot, err := sources.BuildSnapshot(created, "https://example.test/annual-report", "text/plain; charset=utf-8",
		[]byte("星河科技在年报中披露主营算力服务与数据中心运营。"), now, 1024)
	if err != nil {
		t.Fatalf("构造快照失败: %v", err)
	}
	stored, err := repo.SaveSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("保存快照失败: %v", err)
	}
	loaded, err := repo.SnapshotByID(ctx, stored.ID)
	if err != nil {
		t.Fatalf("按编号读取快照失败: %v", err)
	}
	if loaded.Hash != snapshot.Hash || loaded.Body != snapshot.Body || loaded.URL != snapshot.URL || loaded.SourceCategory != snapshot.SourceCategory || loaded.LicenseRef != snapshot.LicenseRef || len(loaded.Sentences) != len(snapshot.Sentences) {
		t.Fatalf("往返读取内容不一致: got=%#v want=%#v", loaded, snapshot)
	}
	duplicate, err := repo.SaveSnapshot(ctx, snapshot)
	if err != nil || duplicate.ID != stored.ID || !duplicate.FetchedAt.Equal(stored.FetchedAt) {
		t.Fatalf("重复快照未幂等返回既有记录: got=%#v err=%v, want id=%d", duplicate, err, stored.ID)
	}
	if _, err := repo.SnapshotForExtraction(ctx, stored.ID); err != nil {
		t.Fatalf("有效授权来源的快照不可抽取: %v", err)
	}
	queue := NewAIExtractionRepo(db)
	quote := "星河科技在年报中披露主营算力服务与数据中心运营。"
	item := candidate.Candidate{
		SnapshotID: stored.ID, ClaimIndex: 0, ThemeID: 100001, ThemeKey: "算力",
		StockID: 1, EntityKey: "星河科技", TsCode: "688001.SH", Predicate: "DISCLOSES_BUSINESS", SourceType: 2,
		SourceExcerpt: quote, SentenceIDs: []int{1}, QuoteSegments: []string{quote},
		SourceURL: stored.URL, CollectedAt: stored.FetchedAt, Provider: "integration-test",
		Model: "fixture-model", PromptVersion: "l0-extraction-v1",
	}
	savedCandidates, err := queue.SavePendingCandidates(ctx, []candidate.Candidate{item})
	if err != nil || len(savedCandidates) != 1 || savedCandidates[0].ID <= 0 {
		t.Fatalf("保存待审候选失败: candidates=%+v err=%v", savedCandidates, err)
	}
	duplicateCandidate, err := queue.SavePendingCandidates(ctx, []candidate.Candidate{item})
	if err != nil || len(duplicateCandidate) != 1 || duplicateCandidate[0].ID != savedCandidates[0].ID {
		t.Fatalf("重复抽取没有幂等返回: candidates=%+v err=%v", duplicateCandidate, err)
	}
	if got := e.queryInt("SELECT COUNT(*) FROM tm_ai_extraction_candidates c WHERE NOT EXISTS (SELECT 1 FROM tm_ai_review_audits a WHERE a.candidate_id = c.id)"); got != 1 {
		t.Fatalf("候选必须只进入待审队列，数量=%d", got)
	}
	runs := NewAIL0RunRepo(db)
	runRequest := pipeline.RunRequest{SnapshotID: stored.ID, RequestID: "test_mysql_extraction_00001"}
	policy := pipeline.RunPolicy{Provider: item.Provider, Model: item.Model, ApprovalRef: "test-written-approval"}
	run, started, err := runs.BeginRun(ctx, runRequest, 7, policy)
	if err != nil || !started || run.State != "running" {
		t.Fatalf("抽取请求登记失败: %+v %v", run, err)
	}
	if replay, started, err := runs.BeginRun(ctx, runRequest, 7, policy); err != nil || started || replay.ID != run.ID {
		t.Fatalf("抽取登记不幂等: %v", err)
	}
	if _, _, err := runs.BeginRun(ctx, runRequest, 8, policy); !errors.Is(err, pipeline.ErrRunConflict) {
		t.Fatal("抽取请求身份未绑定")
	}
	runResult := pipeline.ExtractionResult{Candidates: []candidate.Candidate{item}, Attempts: 1, Model: item.Model}
	if _, err := runs.FinishRun(ctx, run.ID, runResult, ""); !errors.Is(err, pipeline.ErrAuditUnavailable) {
		t.Fatalf("无调用审计仍能成功: %v", err)
	}
	for _, phase := range []string{"start", "finish"} {
		if err := runs.AppendCallEvent(ctx, pipeline.CallEvent{RunID: run.ID, Attempt: 1, Phase: phase, Payload: []byte(`{"test_fixture":true}`)}); err != nil {
			t.Fatal(err)
		}
	}
	finished, err := runs.FinishRun(ctx, run.ID, runResult, "")
	if err != nil || finished.State != "succeeded" || len(finished.Result.Candidates) != 1 || finished.Result.Candidates[0].ID != savedCandidates[0].ID {
		t.Fatalf("抽取候选与终态提交失败: %+v %v", finished, err)
	}
	if _, err := runs.FinishRun(ctx, run.ID, runResult, ""); !errors.Is(err, pipeline.ErrRunConflict) {
		t.Fatal("抽取终态允许再次提交")
	}
	if err := runs.AppendCallEvent(ctx, pipeline.CallEvent{RunID: run.ID, Attempt: 2, Phase: "start", Payload: []byte(`{}`)}); !errors.Is(err, pipeline.ErrRunConflict) {
		t.Fatal("终态仍可追加模型调用")
	}
	for _, query := range []string{"UPDATE tm_ai_l0_call_events SET payload='tampered' WHERE run_id=?", "DELETE FROM tm_ai_l0_call_events WHERE run_id=?", "DELETE FROM tm_ai_l0_extraction_runs WHERE id=?"} {
		if _, err := db.ExecContext(ctx, query, run.ID); err == nil {
			t.Fatalf("抽取审计可被篡改: %s", query)
		}
	}
	if replay, started, err := runs.BeginRun(ctx, runRequest, 7, policy); err != nil || started || replay.State != "succeeded" {
		t.Fatalf("抽取成功重放失败: %v", err)
	}
	catalog := NewAIExtractionCatalog(db)
	vocabulary, err := catalog.Vocabulary(ctx)
	if err != nil || len(vocabulary.Themes["算力"]) != 1 || vocabulary.Themes["算力"][0] != "算力" {
		t.Fatalf("目录词汇读取失败: vocabulary=%+v err=%v", vocabulary, err)
	}
	stockRef, found, err := catalog.StockByAlias(ctx, "星河科技")
	if err != nil || !found || stockRef.ID != 1 || stockRef.TsCode != "688001.SH" {
		t.Fatalf("股票精确解析失败: ref=%+v found=%v err=%v", stockRef, found, err)
	}
	themeRef, found, err := catalog.ThemeByKey(ctx, "算力")
	if err != nil || !found || themeRef.ID != 100001 {
		t.Fatalf("题材精确解析失败: ref=%+v found=%v err=%v", themeRef, found, err)
	}
	forged := item
	forged.SourceExcerpt = "星河科技主营算力业务，建议买入。"
	forged.QuoteSegments = []string{forged.SourceExcerpt}
	if _, err := queue.SavePendingCandidates(ctx, []candidate.Candidate{forged}); err == nil {
		t.Fatal("不属于快照或含红线的伪造候选未被拒绝")
	}

	var bodyRef, contentHash string
	if err := db.QueryRowContext(ctx, `SELECT body_ref, content_hash FROM tm_ai_source_snapshots WHERE id = ?`, stored.ID).
		Scan(&bodyRef, &contentHash); err != nil {
		t.Fatalf("读取正文引用失败: %v", err)
	}
	if bodyRef != snapshot.Hash || contentHash != snapshot.Hash {
		t.Fatalf("body_ref 未按哈希寻址: body_ref=%q content_hash=%q", bodyRef, contentHash)
	}
	for _, test := range []struct {
		query string
		arg   any
	}{
		{`UPDATE tm_ai_source_snapshots SET url = 'https://example.test/changed' WHERE id = ?`, stored.ID},
		{`DELETE FROM tm_ai_source_snapshots WHERE id = ?`, stored.ID},
		{`UPDATE tm_ai_source_bodies SET body_text = '篡改' WHERE content_hash = ?`, snapshot.Hash},
		{`DELETE FROM tm_ai_source_bodies WHERE content_hash = ?`, snapshot.Hash},
		{`UPDATE tm_ai_extraction_candidates SET source_excerpt = '篡改' WHERE id = ?`, savedCandidates[0].ID},
		{`DELETE FROM tm_ai_extraction_candidates WHERE id = ?`, savedCandidates[0].ID},
	} {
		if _, err := db.ExecContext(ctx, test.query, test.arg); err == nil {
			t.Errorf("不可变数据写入未被拒绝: %s", test.query)
		}
	}

	// 本段实测 Repository 的 SQL 事务；Service 状态机另由 ai_review_test.go 验证。
	reviews := NewAIReviewRepo(db)
	pending, err := reviews.ListReviewQueue(ctx, 0, 20)
	if err != nil || len(pending) != 1 {
		t.Fatalf("待审队列读取失败: %+v %v", pending, err)
	}
	requestFor := func(item candidate.Candidate, requestID string) candidate.ReviewRequest {
		digest, err := candidate.EvidenceDigest(item)
		if err != nil {
			t.Fatal(err)
		}
		return candidate.ReviewRequest{CandidateID: item.ID, EvidenceDigest: digest, Decision: "approve", RequestID: requestID}
	}
	approve := func(request candidate.ReviewRequest) (candidate.AuditEvent, error) {
		var event candidate.AuditEvent
		err := reviews.WithinReviewTx(ctx, func(tx candidate.ReviewTx) error {
			record, err := tx.LockCandidate(ctx, request.CandidateID)
			if err != nil {
				return err
			}
			previous, found, err := tx.AuditByRequest(ctx, request.RequestID)
			if err != nil {
				return err
			}
			if found {
				event = previous
				return nil
			}
			if record.Status != candidate.Pending {
				return candidate.ErrConflict
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
			mappingID, err := tx.InsertMapping(ctx, record.Candidate, 7)
			if err != nil {
				return err
			}
			event, err = tx.AppendAudit(ctx, request, 7, mappingID)
			if err != nil {
				return err
			}
			return tx.RefreshThemeCount(ctx, record.Candidate.ThemeID)
		})
		return event, err
	}
	request := requestFor(savedCandidates[0], "test_mysql_review_00001")
	approved, err := approve(request)
	if err != nil || approved.MappingID <= 0 {
		t.Fatalf("审核事务通过失败: %+v %v", approved, err)
	}
	replayed, err := approve(request)
	if err != nil || replayed != approved {
		t.Fatalf("审核事务重放不幂等: %+v %v", replayed, err)
	}
	if e.queryInt("SELECT COUNT(*) FROM addon_quant_theme_stock WHERE audit_status = 2 AND source_excerpt <> '' AND source_url <> '' AND source_type = 2 AND collected_at IS NOT NULL") != 1 ||
		e.queryInt("SELECT stock_count FROM addon_quant_theme WHERE id = 100001") != 1 ||
		e.queryInt("SELECT COUNT(*) FROM tm_ai_review_audits") != 1 {
		t.Fatal("审核事务的归属、审计或计数不一致")
	}
	if got := e.queryString("SELECT source_excerpt FROM addon_quant_theme_stock WHERE id = ?", approved.MappingID); got != quote {
		t.Fatalf("入库引文被改写: %q", got)
	}
	pending, err = reviews.ListReviewQueue(ctx, 0, 20)
	if err != nil || len(pending) != 0 {
		t.Fatalf("通过后仍在待审队列: %+v %v", pending, err)
	}
	for _, query := range []string{
		"UPDATE tm_ai_review_audits SET actor_id = 8 WHERE id = ?",
		"DELETE FROM tm_ai_review_audits WHERE id = ?",
	} {
		if _, err := db.ExecContext(ctx, query, approved.ID); err == nil {
			t.Fatalf("审计可被篡改: %s", query)
		}
	}
	newCandidate := func(url string, themeID int64) candidate.Candidate {
		t.Helper()
		nextSnapshot := snapshot
		nextSnapshot.URL = url
		stored, err := repo.SaveSnapshot(ctx, nextSnapshot)
		if err != nil {
			t.Fatal(err)
		}
		next := item
		next.SnapshotID, next.SourceURL, next.CollectedAt, next.ThemeID = stored.ID, stored.URL, stored.FetchedAt, themeID
		saved, err := queue.SavePendingCandidates(ctx, []candidate.Candidate{next})
		if err != nil {
			t.Fatal(err)
		}
		return saved[0]
	}
	conflictItem := newCandidate("https://example.test/report-conflict", 100001)
	if _, err := approve(requestFor(conflictItem, "test_mysql_review_00002")); !errors.Is(err, candidate.ErrConflict) {
		t.Fatalf("已有归属被覆盖: %v", err)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_ai_review_audits WHERE candidate_id = ?", conflictItem.ID) != 0 {
		t.Fatal("冲突事务留下审计终态")
	}

	e.mustExec(`INSERT INTO addon_quant_theme (id, name, level, parent_id, status) VALUES (100002, '算力', 1, 0, 1)`)
	faultItem := newCandidate("https://example.test/report-fault", 100002)
	faultRequest := requestFor(faultItem, "test_mysql_review_00003")
	e.mustExec(`CREATE TRIGGER tm_ai_review_test_fail BEFORE INSERT ON tm_ai_review_audits FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'test audit failure'`)
	if _, err := approve(faultRequest); err == nil {
		t.Fatal("审计故障没有中止事务")
	}
	e.mustExec("DROP TRIGGER tm_ai_review_test_fail")
	if e.queryInt("SELECT COUNT(*) FROM addon_quant_theme_stock WHERE theme_id = 100002") != 0 ||
		e.queryInt("SELECT COUNT(*) FROM tm_ai_review_audits WHERE candidate_id = ?", faultItem.ID) != 0 {
		t.Fatal("审计故障未回滚归属")
	}
	if _, err := approve(faultRequest); err != nil {
		t.Fatalf("故障恢复后同请求不能重试: %v", err)
	}

	e.mustExec(`INSERT INTO addon_quant_theme (id, name, level, parent_id, status) VALUES (100003, '算力', 1, 0, 1)`)
	concurrentItem := newCandidate("https://example.test/report-concurrent", 100003)
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, id := range []string{"test_mysql_review_00004", "test_mysql_review_00005"} {
		r := requestFor(concurrentItem, id)
		wg.Add(1)
		go func() { defer wg.Done(); _, err := approve(r); outcomes <- err }()
	}
	wg.Wait()
	close(outcomes)
	successes := 0
	for err := range outcomes {
		if err == nil {
			successes++
		} else if !errors.Is(err, candidate.ErrConflict) {
			t.Fatalf("并发审核出现非冲突错误: %v", err)
		}
	}
	if successes != 1 || e.queryInt("SELECT COUNT(*) FROM tm_ai_review_audits WHERE candidate_id = ?", concurrentItem.ID) != 1 {
		t.Fatal("并发审核不是唯一终态")
	}

	rejectItem := newCandidate("https://example.test/report-reject", 100001)

	if err := repo.SetSourceEnabled(ctx, created.ID, false); err != nil {
		t.Fatalf("停用来源失败: %v", err)
	}
	if _, err := repo.SnapshotForExtraction(ctx, stored.ID); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("停用来源快照仍可用于抽取: %v", err)
	}
	if _, err := queue.SavePendingCandidates(ctx, []candidate.Candidate{item}); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("停用来源仍可写待审候选: %v", err)
	}
	if _, err := repo.SaveSnapshot(ctx, snapshot); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("停用来源仍可固化快照，错误=%v", err)
	}
	if _, err := approve(requestFor(rejectItem, "test_mysql_review_00006")); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("来源停用仍可通过审核: %v", err)
	}
	rejectRequest := requestFor(rejectItem, "test_mysql_review_00007")
	rejectRequest.Decision, rejectRequest.Reason = "reject", "来源授权已停用"
	err = reviews.WithinReviewTx(ctx, func(tx candidate.ReviewTx) error {
		if _, err := tx.LockCandidate(ctx, rejectItem.ID); err != nil {
			return err
		}
		_, err := tx.AppendAudit(ctx, rejectRequest, 7, 0)
		return err
	})
	if err != nil {
		t.Fatalf("来源停用后不能驳回: %v", err)
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_ai_review_audits WHERE candidate_id = ? AND decision = 'reject' AND mapping_id IS NULL", rejectItem.ID) != 1 {
		t.Fatal("驳回审计不完整")
	}

	if err := repo.SetSourceEnabled(ctx, created.ID, true); err != nil {
		t.Fatalf("重新启用来源失败: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE tm_ai_sources SET license_expires_at = UTC_TIMESTAMP(3) - INTERVAL 1 SECOND WHERE id = ?`, created.ID); err != nil {
		t.Fatalf("准备过期授权反例失败: %v", err)
	}
	if err := repo.ValidateEnabledSources(ctx); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("授权过期的启用来源未阻止启动，错误=%v", err)
	}
	if expired, err := repo.SourceByID(ctx, created.ID); err != nil || !expired.Enabled {
		t.Fatalf("过期来源仍应允许内部读取核对: %v", err)
	}
	verifyInspectionRepository(t, e, db, savedCandidates[0], approved)
	verifySourceAuditRepository(t, e, db)
	verifyTrimmedMappingRepository(t, e, db, savedCandidates[0])
}

func cleanupAISourceArtifacts(t *testing.T, e *convergeEnv) {
	t.Helper()
	for _, trigger := range []string{
		"tm_ai_source_snapshots_no_update",
		"tm_ai_source_snapshots_no_delete",
		"tm_ai_source_bodies_no_update",
		"tm_ai_source_bodies_no_delete",
		"tm_ai_candidates_no_update",
		"tm_ai_candidates_no_delete",
		"tm_ai_review_audits_no_update",
		"tm_ai_review_audits_no_delete",
		"tm_ai_review_test_fail",
		"tm_ai_l0_call_events_no_update",
		"tm_ai_l0_call_events_no_delete",
		"tm_ai_l0_runs_no_delete",
		"tm_ai_inspection_batches_no_update", "tm_ai_inspection_batches_no_delete",
		"tm_ai_inspection_items_no_update", "tm_ai_inspection_items_no_delete",
		"tm_ai_inspection_events_no_update", "tm_ai_inspection_events_no_delete",
		"tm_ai_inspection_test_fail",
		"tm_ai_source_audits_no_update", "tm_ai_source_audits_no_delete", "tm_ai_source_audit_test_fail",
	} {
		if err := e.exec("DROP TRIGGER IF EXISTS `" + trigger + "`"); err != nil {
			t.Errorf("清理 AI 资料触发器 %s 失败: %v", trigger, err)
		}
	}
	for _, table := range []string{"tm_ai_source_audits", "tm_ai_inspection_events", "tm_ai_inspection_items", "tm_ai_inspection_batches", "tm_ai_l0_call_events", "tm_ai_l0_extraction_runs", "tm_ai_review_audits", "tm_ai_extraction_candidates", "tm_ai_source_snapshots", "tm_ai_source_bodies", "tm_ai_sources"} {
		if err := e.exec("DROP TABLE IF EXISTS `" + table + "`"); err != nil {
			t.Errorf("清理 AI 资料表 %s 失败: %v", table, err)
		}
	}
}
