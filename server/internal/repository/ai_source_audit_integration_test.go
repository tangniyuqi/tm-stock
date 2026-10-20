//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// verifySourceAuditRepository 仅由使用可丢弃测试库的主集成用例调用。
func verifySourceAuditRepository(t *testing.T, e *convergeEnv, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()
	repo := NewAISourceRepo(db)
	registration := sources.Source{Domain: "audit.example.test", Category: sources.CategoryAnnualReport,
		LicenseTag: "integration-test", LicenseRef: "test-fixture:source-audit", MinInterval: time.Second}
	register := func(input sources.Source) (sources.Source, error) {
		var created sources.Source
		err := repo.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
			var err error
			created, err = tx.RegisterSource(ctx, input)
			if err != nil {
				return err
			}
			return tx.AppendSourceAudit(ctx, sources.SourceAudit{ActorID: 7, Operation: "register", After: created})
		})
		return created, err
	}
	created, err := register(registration)
	if err != nil || created.Enabled {
		t.Fatalf("带审计登记失败: %v", err)
	}
	state := func(actor int64, enabled bool) error {
		return repo.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
			before, err := tx.SourceByID(ctx, created.ID)
			if err != nil {
				return err
			}
			if err := tx.SetSourceEnabled(ctx, created.ID, enabled); err != nil {
				return err
			}
			after, err := tx.SourceByID(ctx, created.ID)
			if err != nil {
				return err
			}
			op := "disable"
			if enabled {
				op = "enable"
			}
			return tx.AppendSourceAudit(ctx, sources.SourceAudit{ActorID: actor, Operation: op, Before: &before, After: after})
		})
	}
	if err := state(8, true); err != nil {
		t.Fatal(err)
	}
	current, err := repo.SourceByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := sources.BuildSnapshot(current, "https://audit.example.test/report", "text/plain", []byte("集成测试资料：企业披露主营服务。"), time.Now().UTC(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	save := func(operation string) (sources.Snapshot, error) {
		var stored sources.Snapshot
		err := repo.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
			current, err := tx.SourceByID(ctx, created.ID)
			if err != nil {
				return err
			}
			stored, err = tx.SaveSnapshot(ctx, snapshot)
			if err != nil {
				return err
			}
			return tx.AppendSourceAudit(ctx, sources.SourceAudit{ActorID: 9, Operation: operation, After: current,
				SnapshotID: stored.ID, ContentHash: stored.Hash, URL: stored.URL})
		})
		return stored, err
	}
	// 插入审计失败时，登记、状态、正文和快照均不能单独提交。
	e.mustExec(`CREATE TRIGGER tm_ai_source_audit_test_fail BEFORE INSERT ON tm_ai_source_audits
	  FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'test source audit failure'`)
	failedSource := registration
	failedSource.Domain = "rollback.example.test"
	if _, err := register(failedSource); err == nil {
		t.Fatal("审计失败仍登记成功")
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_ai_sources WHERE domain = 'rollback.example.test'") != 0 {
		t.Fatal("来源登记没有回滚")
	}
	if err := state(8, false); err == nil {
		t.Fatal("审计失败仍停用成功")
	}
	if s, err := repo.SourceByID(ctx, created.ID); err != nil || !s.Enabled {
		t.Fatal("启停没有回滚")
	}
	if _, err := save("import"); err == nil {
		t.Fatal("审计失败仍固化成功")
	}
	if e.queryInt("SELECT COUNT(*) FROM tm_ai_source_snapshots WHERE source_id = ?", created.ID) != 0 ||
		e.queryInt("SELECT COUNT(*) FROM tm_ai_source_bodies WHERE content_hash = ?", snapshot.Hash) != 0 {
		t.Fatal("快照或正文没有回滚")
	}
	e.mustExec("DROP TRIGGER tm_ai_source_audit_test_fail")
	stored, err := save("import")
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := save("fetch")
	if err != nil || duplicate.ID != stored.ID {
		t.Fatal("同证据未复用快照")
	}
	events, err := repo.ListSourceAudits(ctx, created.ID, 0, 200)
	if err != nil || len(events) != 4 {
		t.Fatalf("成功操作审计不完整: %+v %v", events, err)
	}
	for i, op := range []string{"register", "enable", "import", "fetch"} {
		if events[i].Operation != op || events[i].After.ID != created.ID || events[i].Validate() != nil {
			t.Fatal("历史副本错误")
		}
	}
	afterJSON, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []struct {
		operation                            string
		before, after, snapshotID, hash, url any
	}{
		{"fetch", nil, string(afterJSON), nil, stored.Hash, stored.URL},
		{"fetch", nil, string(afterJSON), stored.ID, "bad-hash", stored.URL},
		{"disable", nil, string(afterJSON), nil, nil, nil},
		{"register", nil, "[]", nil, nil, nil},
	} {
		_, err := db.ExecContext(ctx, `INSERT INTO tm_ai_source_audits
		  (actor_id,operation,source_id,before_json,after_json,snapshot_id,content_hash,source_url)
		  VALUES (7,?,?,?,?,?,?,?)`, invalid.operation, created.ID, invalid.before, invalid.after, invalid.snapshotID, invalid.hash, invalid.url)
		var mysqlErr *mysql.MySQLError
		if !errors.As(err, &mysqlErr) || mysqlErr.Number != 3819 {
			t.Fatalf("来源审计 CHECK 未真正拒绝反例: %v", err)
		}
	}
	if page, err := repo.ListSourceAudits(ctx, created.ID, events[1].ID, 1); err != nil || len(page) != 1 || page[0].ID != events[2].ID {
		t.Fatal("分页错误")
	}
	for _, query := range []string{"UPDATE tm_ai_source_audits SET actor_id=99 WHERE source_id=?", "DELETE FROM tm_ai_source_audits WHERE source_id=?"} {
		if _, err := db.ExecContext(ctx, query, created.ID); err == nil {
			t.Fatal("审计允许更新/删除")
		}
	}
	// 追加必须绑定真实当前来源和已存在快照，不允许伪造引用。
	for _, wrongURL := range []string{"https://audit.example.test/not-the-snapshot", strings.Replace(stored.URL, "/report", "/REPORT", 1)} {
		err = repo.WithinSourceTx(ctx, func(tx sources.MutationTx) error {
			current, err := tx.SourceByID(ctx, created.ID)
			if err != nil {
				return err
			}
			return tx.AppendSourceAudit(ctx, sources.SourceAudit{ActorID: 9, Operation: "fetch", After: current,
				SnapshotID: stored.ID, ContentHash: stored.Hash, URL: wrongURL})
		})
		if !errors.Is(err, sources.ErrInvalidSourceAudit) {
			t.Fatalf("伪造或大小写不一致的快照引用未拒绝: %v", err)
		}
	}
	// 每个启停先锁来源，因此同来源审计不会丢失或出现前后状态断链。
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := state(int64(20+i), i%2 == 0); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	events, err = repo.ListSourceAudits(ctx, created.ID, 0, 200)
	if err != nil || len(events) != 12 {
		t.Fatalf("并发审计缺失: %v", err)
	}
	previous := true
	for _, event := range events[4:] {
		if event.Before == nil || event.Before.Enabled != previous {
			t.Fatal("启停审计前后断链")
		}
		previous = event.After.Enabled
	}
	// 授权已到期仍可停用且保留历史；不能再次启用或采集。
	e.mustExec("UPDATE tm_ai_sources SET license_expires_at = UTC_TIMESTAMP(3) - INTERVAL 1 SECOND WHERE id = ?", created.ID)
	if err := state(8, false); err != nil {
		t.Fatal(err)
	}
	if err := state(8, true); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatalf("过期来源重新启用: %v", err)
	}
	if _, err := save("import"); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatal("过期来源写入快照")
	}
	if events, err := repo.ListSourceAudits(ctx, created.ID, 0, 200); err != nil || len(events) != 13 {
		t.Fatalf("过期来源历史读取失败: %v", err)
	}
}
