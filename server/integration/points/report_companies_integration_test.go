//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 仅使用示例来源和测试控制器，不启用真实Agent、模型或任何公网服务。
func newCompanyReportEnv(t *testing.T, level sql.IsolationLevel) *submissionHTTPEnv {
	return newCompanyReportEnvWithDuration(t, level, 0)
}

func newCompanyReportEnvWithDuration(t *testing.T, level sql.IsolationLevel, duration int64) *submissionHTTPEnv {
	return newCompanyReportEnvWithPlan(t, level, duration, nil)
}
func newCompanyReportEnvWithPlan(t *testing.T, level sql.IsolationLevel, duration int64, prepare func(*submissionHTTPEnv)) *submissionHTTPEnv {
	t.Helper()
	e := newSubmissionHTTPEnvWithPlan(t, level, "pass", func(e *submissionHTTPEnv) {
		prepareCompanyReportPlan(t, e, duration)
		if prepare != nil {
			prepare(e)
		}
	})
	created := decodeCreation(t, e.mustRequest(t, "POST", "/api/v1/ai/tasks", e.key, sqlSubmissionBody, e.web.token(t, 1, "client"), 200))
	e.bindNewTask(t, created.TaskID)
	return e
}

func prepareCompanyReportPlan(t *testing.T, e *submissionHTTPEnv, duration int64) {
	t.Helper()
	e.mustExec(`ALTER TABLE addon_quant_theme ADD parent_id bigint UNSIGNED NOT NULL DEFAULT 0;
 CREATE INDEX idx_company_test_parent ON addon_quant_theme(parent_id,status,deleted_at);
 INSERT INTO addon_quant_theme(id,name,status,parent_id) VALUES(2,'示例环节',1,1),(3,'其他题材',1,0),(4,'其他环节',1,3);
 CREATE TABLE addon_quant_base_stock(id bigint UNSIGNED PRIMARY KEY,ts_code varchar(20),symbol varchar(20),name varchar(100),deleted_at datetime(3));
 INSERT INTO addon_quant_base_stock(id,ts_code,symbol,name) VALUES(1,'600002.SH','600002','示例公司'),(2,'600001.SH','600001','另一示例公司')`)
	reportDDL(t, e.pointsTestEnv, "20260730_addon_quant_theme_stock.sql")
	reportDDL(t, e.pointsTestEnv, "20261011_add_theme_stock_excerpt_trimmed.sql")
	snap, err := sources.BuildSnapshot(e.source, "https://evidence.example.test/company-example", sources.MediaPlain, []byte("示例题材是本测试资料的主题。示例题材的事项发生于资料记载日期。示例题材包含资料所述环节。示例公司在示例题材的资料中披露业务事项。另一示例公司在示例环节披露业务事项。"), e.snap.FetchedAt, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	snap, err = repository.NewAISourceRepo(e.db).SaveSnapshot(e.ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	e.snap = snap
	e.planner.p.Snapshots = []sources.Snapshot{snap}
	// 重新固化测试执行计划，而不是在创建之后悄悄增加材料范围。
	e.ctl.p.Materials = []task.AdmissionMaterial{{SnapshotID: snap.ID, Digest: snap.Hash, Domain: e.source.Domain, Category: snap.SourceCategory, URL: snap.URL, CollectedAt: snap.FetchedAt}}
	e.ctl.p.ExecutionDigest, err = e.planner.p.AdmissionDigest()
	if err != nil {
		t.Fatal(err)
	}
	if duration > 0 {
		e.ctl.p.Budget.MaxDurationMS = duration
	}
	for i, sid := range []int64{1, 2} {
		code, node := "600002.SH", int64(1)
		if sid == 2 {
			code, node = "600001.SH", 2
		}
		_, err := e.db.ExecContext(e.ctx, `INSERT INTO addon_quant_theme_stock(id,theme_id,stock_id,ts_code,source_type,source_excerpt,excerpt_trimmed,source_url,collected_at,audit_status,audit_by,audit_at,status,sort)
 VALUES(?,?,?,?,2,?,1,?,?,2,11,UTC_TIMESTAMP(3),1,?)`, sid, node, sid, code, snap.Sentences[i+3].Text, snap.URL, snap.FetchedAt.UTC().Format("2006-01-02 15:04:05.000"), -int(sid))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegrationAIReportCompaniesReadOnly(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCompanyReportEnv(t, level)
			before := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
			d, err := e.svc.PrepareCompanies(e.ctx, 1, e.v.ID)
			if err != nil || d == nil || len(d.Rows()) != 2 || d.Rows()[0].Row.Code != "600001" || d.Rows()[1].Row.Code != "600002" {
				t.Fatal("真库公司行准备失败", err)
			}
			for _, n := range d.Rows() {
				if n.Row.Change != nil || !n.Row.Evidence[0].Trimmed {
					t.Fatal("行情假值或节选丢失")
				}
			}
			e.mustExec(`UPDATE addon_quant_theme_stock SET sort=sort+99`)
			again, err := e.svc.PrepareCompanies(e.ctx, 1, e.v.ID)
			if err != nil || again.Digest() != d.Digest() {
				t.Fatal("人工排序改变报告公司", err)
			}
			for _, ids := range [][2]int64{{2, e.v.ID}, {1, e.v.ID + 99}} {
				if out, err := e.svc.PrepareCompanies(e.ctx, ids[0], ids[1]); err == nil || out != nil {
					t.Fatal("本人隔离失败")
				}
			}
			if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != before || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.calls.Load() != 0 || e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
				t.Fatal("准备公司修改账务或调用模型")
			}
			requirePointAudit(t, e.pointsTestEnv, e.ledger)
		})
	}
}

type companyUnknownCommitStore struct{ *repository.AITaskRepo }

func (s companyUnknownCommitStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	if err := s.AITaskRepo.WithinReportTx(ctx, fn); err != nil {
		return err
	}
	return errCheckTest
}

func TestIntegrationAIReportCompaniesBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"off", "withdrawn", "license", "master_code", "master_name", "mapping_code", "excerpt", "type", "time", "reviewer", "review_time", "missing_admission", "cancel", "expired", "unknown_commit", "limit", "unreviewed", "disabled", "deleted", "deleted_stock", "deleted_node", "disabled_node", "wrong_parent", "empty"} {
				t.Run(mode, func(t *testing.T) {
					var e *submissionHTTPEnv
					if mode == "expired" {
						e = newCompanyReportEnvWithDuration(t, level, 1000)
					} else {
						e = newCompanyReportEnv(t, level)
					}
					wantCount := -1
					switch mode {
					case "off":
						var err error
						e.svc, err = service.NewAIReportService(e.repo, e.ledger, nil, false)
						if err != nil {
							t.Fatal(err)
						}
						e.mustExec(`DROP TABLE addon_quant_theme_stock`)
					case "withdrawn":
						e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
					case "license":
						e.mustExec(`UPDATE tm_ai_sources SET license_expires_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND`)
					case "master_code":
						e.mustExec(`UPDATE addon_quant_base_stock SET ts_code='600009.SH' WHERE id=1`)
					case "master_name":
						e.mustExec(`UPDATE addon_quant_base_stock SET name='未披露公司' WHERE id=1`)
					case "mapping_code":
						e.mustExec(`UPDATE addon_quant_theme_stock SET ts_code='600009.SH' WHERE id=1`)
					case "excerpt":
						e.mustExec(`UPDATE addon_quant_theme_stock SET source_excerpt=CONCAT(source_excerpt,'另一句话') WHERE id=1`)
					case "type":
						e.mustExec(`UPDATE addon_quant_theme_stock SET source_type=1 WHERE id=1`)
					case "time":
						e.mustExec(`UPDATE addon_quant_theme_stock SET collected_at=collected_at-INTERVAL 1 SECOND WHERE id=1`)
					case "reviewer":
						e.mustExec(`UPDATE addon_quant_theme_stock SET audit_by=NULL WHERE id=1`)
					case "review_time":
						e.mustExec(`UPDATE addon_quant_theme_stock SET audit_at=NULL WHERE id=1`)
					case "missing_admission":
						e.mustExec(`DROP TRIGGER tm_ai_admission_no_delete;DELETE FROM tm_ai_task_admissions`)
					case "cancel":
						e.mustExec(`UPDATE tm_ai_tasks SET cancel_req=1 WHERE status=1`)
					case "expired":
						if e.queryInt(`SELECT SLEEP(1.1)`) != 0 {
							t.Fatal("数据库等待到期失败")
						}
					case "unknown_commit":
						var err error
						e.svc, err = service.NewAIReportService(companyUnknownCommitStore{e.repo}, e.ledger, e.policy, true)
						if err != nil {
							t.Fatal(err)
						}
					case "limit":
						e.mustExec(`DELETE FROM addon_quant_theme_stock`)
						for i := 1; i <= finalreport.MaxCompanyMappings+1; i++ {
							code := fmt.Sprintf("600%03d", i)
							_, err := e.db.ExecContext(e.ctx, `INSERT INTO addon_quant_base_stock(id,ts_code,symbol,name) VALUES(?,?,?,'示例公司')`, i+10, code+".SH", code)
							if err != nil {
								t.Fatal(err)
							}
							_, err = e.db.ExecContext(e.ctx, `INSERT INTO addon_quant_theme_stock(theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,audit_by,audit_at,status) VALUES(1,?,?,2,?,?,?,2,11,UTC_TIMESTAMP(3),1)`, i+10, code+".SH", e.snap.Sentences[0].Text, e.snap.URL, e.snap.FetchedAt.UTC().Format("2006-01-02 15:04:05.000"))
							if err != nil {
								t.Fatal(err)
							}
						}
					case "unreviewed":
						e.mustExec(`UPDATE addon_quant_theme_stock SET audit_status=1 WHERE id=1`)
						wantCount = 1
					case "disabled":
						e.mustExec(`UPDATE addon_quant_theme_stock SET status=0 WHERE id=1`)
						wantCount = 1
					case "deleted":
						e.mustExec(`UPDATE addon_quant_theme_stock SET deleted_at=UTC_TIMESTAMP(3) WHERE id=1`)
						wantCount = 1
					case "deleted_stock":
						e.mustExec(`UPDATE addon_quant_base_stock SET deleted_at=UTC_TIMESTAMP(3) WHERE id=1`)
						wantCount = 1
					case "deleted_node":
						e.mustExec(`UPDATE addon_quant_theme SET deleted_at=UTC_TIMESTAMP(3) WHERE id=2`)
						wantCount = 1
					case "disabled_node":
						e.mustExec(`UPDATE addon_quant_theme SET status=0 WHERE id=2`)
						wantCount = 1
					case "wrong_parent":
						e.mustExec(`UPDATE addon_quant_theme SET parent_id=3 WHERE id=2`)
						wantCount = 1
					case "empty":
						e.mustExec(`UPDATE addon_quant_theme_stock SET status=0`)
						wantCount = 0
					}
					before := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
					d, err := e.svc.PrepareCompanies(e.ctx, 1, e.v.ID)
					if wantCount < 0 {
						if err == nil || d != nil {
							t.Fatal("无效当前资料未整批关闭", mode)
						}
					} else if err != nil || d == nil || len(d.Rows()) != wantCount {
						t.Fatal("可见范围错误", mode, err)
					}
					if e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != before || e.queryInt(`SELECT frozen FROM tm_points_accounts WHERE user_id=1`) != 7 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || e.calls.Load() != 0 || e.submits.Load() != 0 || len(e.server.Requests()) != 0 {
						t.Fatal("失败准备破坏冻结或触发调用")
					}
				})
			}
		})
	}
}

func TestIntegrationAIReportCompaniesSharedLocks(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newCompanyReportEnv(t, level)
			err := e.repo.WithinReportTx(e.ctx, func(tx finalreport.Tx) error {
				if _, _, err := tx.ReportEvidence(e.ctx, []int64{e.snap.ID}, 1); err != nil {
					return err
				}
				if _, err := tx.(finalreport.CompanyReader).ReportCompanies(e.ctx, 1); err != nil {
					return err
				}
				for _, q := range []string{`SELECT id FROM addon_quant_theme_stock WHERE id=1 FOR UPDATE NOWAIT`, `SELECT id FROM addon_quant_base_stock WHERE id=1 FOR UPDATE NOWAIT`, `SELECT id FROM addon_quant_theme WHERE id=2 FOR UPDATE NOWAIT`, `SELECT id FROM tm_ai_sources WHERE id=1 FOR UPDATE NOWAIT`, fmt.Sprintf(`SELECT id FROM tm_ai_source_snapshots WHERE id=%d FOR UPDATE NOWAIT`, e.snap.ID)} {
					other, err := e.db.BeginTx(e.ctx, nil)
					if err != nil {
						return err
					}
					var id int64
					probe := other.QueryRowContext(e.ctx, q).Scan(&id)
					if err := other.Rollback(); err != nil {
						return err
					}
					if probe == nil || !strings.Contains(probe.Error(), "3572") {
						return fmt.Errorf("共享锁未保护当前资料: %v", probe)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
