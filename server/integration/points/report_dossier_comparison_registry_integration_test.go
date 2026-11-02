//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 沿用原隔离环境，既有轻任务仅用于确认零新增副作用；不以它冒充深度执行批准。
func newComparisonRegistryEnv(t *testing.T, level sql.IsolationLevel) (*supportSQLEnv, *service.AIDossierSupportRegistryService, *dossier.CardComparisonManifest, *dossier.SupportManifest) {
	t.Helper()
	e, registry, _, old := newDossierRegistryEnv(t, level)
	u := dossier.Field{State: "unknown"}
	sentence := e.snap.Sentences[0]
	card := dossier.FactCard{Key: "comparison_a", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: e.snap.ID, Hash: e.snap.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}
	other := card
	other.Key = "comparison_b"
	draft := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 1, Version: 2, CreatedAt: e.snap.FetchedAt.Add(time.Millisecond).UnixMilli(), Cards: []dossier.FactCard{card, other}}
	if _, err := e.control.dossiers.StageInternal(e.ctx, draft); err != nil {
		t.Fatal("合成两卡版本保存失败", err)
	}
	e.mustExec(`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:comparison-publication',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no=2;
INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:comparison-member',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no=2`)
	bound, err := e.control.dossiers.ReadVersions(e.ctx, "test-member-1", 1, 1, []int64{2})
	if err != nil {
		t.Fatal(err)
	}
	cfg := e.control.cfg
	cfg.ReviewStep = 0
	cfg.MaxOutputTokens = 1024
	prepared, err := dossier.PrepareCardComparison(e.ctx, dossier.CardComparisonConfig{Model: cfg, Focus: "definition", ReviewStep: 5}, e.ctl.p.SubjectName, bound[0], []sources.Snapshot{e.snap}, []string{"comparison_b", "comparison_a"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepared.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	return e, registry, m, old
}

func comparisonRegistryScope() task.AdmissionScope {
	return task.AdmissionScope{UserID: 1, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 1}
}

func comparisonRegistrySave(t *testing.T, e *supportSQLEnv, registry *service.AIDossierSupportRegistryService, m *dossier.CardComparisonManifest) {
	t.Helper()
	if got, err := registry.RegisterComparisonInternal(e.ctx, m); err != nil || got != m.ComponentDigest() {
		t.Fatal("两卡登记失败", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_uses(component_digest,approval_ref,enabled,valid_until,created_at,updated_at) VALUES(?,'test-fixture:comparison-ai-use',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.ComponentDigest()); err != nil {
		t.Fatal(err)
	}
}

func requireComparisonNoSideEffects(t *testing.T, e *supportSQLEnv) {
	t.Helper()
	e.assertUnsettled(t)
	// 原HTTP夹具保留一条已取消任务，再建立一条隔离轻任务；两条均不得由本支线增加。
	if e.server.Count() != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`) != 2 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != 0 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_task_step_results WHERE step_no=5`) != 0 {
		t.Fatal("清单操作执行模型、创建任务或写比较结果")
	}
}

func TestIntegrationAIReportDossierComparisonRegistryLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, registry, m, old := newComparisonRegistryEnv(t, level)
			t.Run("旧结构拒绝新协议", func(t *testing.T) {
				if got, err := registry.RegisterComparisonInternal(e.ctx, m); err == nil || got != "" || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 0 {
					t.Fatal("旧结构接受新协议或留下登记", err)
				}
			})
			reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
			t.Run("并发登记不自动批准", func(t *testing.T) {
				var wg sync.WaitGroup
				errs := make(chan error, 6)
				for i := 0; i < 6; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); _, err := registry.RegisterComparisonInternal(e.ctx, m); errs <- err }()
				}
				wg.Wait()
				close(errs)
				for err := range errs {
					if err != nil {
						t.Fatal(err)
					}
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_uses`) != 0 {
					t.Fatal("重复登记或自动创建用途")
				}
			})
			t.Run("缺少AI用途拒绝", func(t *testing.T) {
				if p, err := registry.PrepareComparisonForScope(e.ctx, comparisonRegistryScope(), m.ComponentDigest()); err == nil || p != nil {
					t.Fatal("缺少AI用途仍提供输入")
				}
			})
			if _, err := e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_uses(component_digest,approval_ref,valid_until,created_at,updated_at) VALUES(?,'test-fixture:comparison-ai-use',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.ComponentDigest()); err != nil {
				t.Fatal(err)
			}
			t.Run("用途默认关闭", func(t *testing.T) {
				if p, err := registry.PrepareComparisonForScope(e.ctx, comparisonRegistryScope(), m.ComponentDigest()); err == nil || p != nil {
					t.Fatal("默认用途开启")
				}
			})
			e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
			t.Run("原文重建及登记重放", func(t *testing.T) {
				p, err := registry.PrepareComparisonForScope(e.ctx, comparisonRegistryScope(), m.ComponentDigest())
				if err != nil {
					t.Fatal(err)
				}
				got, err := p.ComponentDigest(task.DeepAnalysisType)
				if err != nil || got != m.ComponentDigest() {
					t.Fatal("没有重建原组件", err)
				}
				if got, err := registry.RegisterComparisonInternal(e.ctx, m); err != nil || got != m.ComponentDigest() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 {
					t.Fatal("同体登记未幂等", err)
				}
			})
			t.Run("新旧入口互拒", func(t *testing.T) {
				if p, err := registry.PrepareForScope(e.ctx, comparisonRegistryScope(), m.ComponentDigest()); err == nil || p != nil {
					t.Fatal("旧入口执行比较清单")
				}
				registrySave(t, e, registry, old, true)
				if p, err := registry.PrepareComparisonForScope(e.ctx, comparisonRegistryScope(), old.ComponentDigest()); err == nil || p != nil {
					t.Fatal("比较入口解释旧清单")
				}
				if p, err := registry.PrepareForScope(e.ctx, e.ctl.p.Scope, old.ComponentDigest()); err != nil || !p.Valid() {
					t.Fatal("旧重建回归", err)
				}
			})
			t.Run("再次迁移保留历史", func(t *testing.T) {
				reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
				if p, err := registry.PrepareComparisonForScope(e.ctx, comparisonRegistryScope(), m.ComponentDigest()); err != nil || !p.Valid() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 2 {
					t.Fatal("再次迁移损坏登记", err)
				}
			})
			t.Run("零执行与账务副作用", func(t *testing.T) { requireComparisonNoSideEffects(t, e) })
		})
	}
}

func TestIntegrationAIReportDossierComparisonRegistryBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"use_disabled", "use_expired", "source", "license", "grant", "publication", "member", "theme", "cross_member", "light_scope", "metadata", "strategy", "end_clock_expiry", "unknown_registration", "shared_use_lock"} {
				t.Run(mode, func(t *testing.T) {
					e, registry, m, _ := newComparisonRegistryEnv(t, level)
					reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
					store := repository.NewAIDossierSupportRepo(e.db, level)
					if mode == "unknown_registration" {
						unknown := &registryUnknownStore{SupportRegistry: store}
						writer, err := service.NewAIDossierSupportRegistryService(unknown, true)
						if err != nil {
							t.Fatal(err)
						}
						if got, err := writer.RegisterComparisonInternal(e.ctx, m); err == nil || got != "" || !unknown.once.Load() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 {
							t.Fatal("真实提交后未知被误报成功", err)
						}
						if got, err := registry.RegisterComparisonInternal(e.ctx, m); err != nil || got != m.ComponentDigest() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_uses`) != 0 {
							t.Fatal("登记原键恢复或用途隔离失败", err)
						}
						requireComparisonNoSideEffects(t, e)
						return
					}
					comparisonRegistrySave(t, e, registry, m)
					scope := comparisonRegistryScope()
					switch mode {
					case "use_disabled":
						e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=0`)
					case "use_expired":
						e.mustExec(`UPDATE tm_ai_dossier_support_uses SET created_at=UTC_TIMESTAMP(3)-INTERVAL 2 HOUR,valid_until=UTC_TIMESTAMP(3)-INTERVAL 1 HOUR`)
					case "source":
						e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
					case "license":
						e.mustExec(`UPDATE tm_ai_sources SET license_ref='test-fixture:changed'`)
					case "grant":
						e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=0 WHERE version_no=2`)
					case "publication":
						e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=2`)
					case "member":
						e.mustExec(`UPDATE addon_member SET status=0 WHERE id=1`)
					case "theme":
						e.mustExec(`UPDATE addon_quant_theme SET status=0 WHERE id=1`)
					case "cross_member":
						scope.UserID = 2
					case "light_scope":
						scope.AnalysisType = task.LightAnalysisType
					case "metadata":
						e.mustExec(`DROP TRIGGER tm_ai_dossier_support_no_update; UPDATE tm_ai_dossier_support_plans SET compiled_at=compiled_at+INTERVAL 1000 MICROSECOND`)
					case "strategy":
						raw := []byte(strings.Replace(string(m.Canonical()), `"Version":"test-only-v1"`, `"Version":"test-only-v2"`, 1))
						if string(raw) == string(m.Canonical()) {
							t.Fatal("策略未改变")
						}
						e.mustExec(`DROP TRIGGER tm_ai_dossier_support_no_update`)
						if _, err := e.db.ExecContext(e.ctx, `UPDATE tm_ai_dossier_support_plans SET canonical=?,manifest_digest=?`, raw, task.StageDigest(raw)); err != nil {
							t.Fatal(err)
						}
					case "end_clock_expiry":
						var err error
						registry, err = service.NewAIDossierSupportRegistryService(registryClockStore{SupportRegistry: store}, false)
						if err != nil {
							t.Fatal(err)
						}
					case "shared_use_lock":
						err := store.WithinSupport(e.ctx, false, func(tx dossier.SupportRegistryTx) error {
							if _, err := tx.SupportUse(e.ctx, m.ComponentDigest()); err != nil {
								return err
							}
							other, err := e.db.BeginTx(e.ctx, nil)
							if err != nil {
								return err
							}
							defer other.Rollback()
							var digest string
							err = other.QueryRowContext(e.ctx, `SELECT component_digest FROM tm_ai_dossier_support_uses WHERE component_digest=? FOR UPDATE NOWAIT`, m.ComponentDigest()).Scan(&digest)
							var me *mysql.MySQLError
							if !errors.As(err, &me) || me.Number != 3572 {
								return errCheckTest
							}
							return nil
						})
						if err != nil {
							t.Fatal("用途锁未阻止撤回写入", err)
						}
						if p, err := registry.PrepareComparisonForScope(e.ctx, scope, m.ComponentDigest()); err != nil || !p.Valid() {
							t.Fatal(err)
						}
						requireComparisonNoSideEffects(t, e)
						return
					}
					if p, err := registry.PrepareComparisonForScope(e.ctx, scope, m.ComponentDigest()); err == nil || p != nil {
						t.Fatal("失效后泄露比较输入", mode, err)
					}
					requireComparisonNoSideEffects(t, e)
				})
			}
		})
	}
}

func TestIntegrationAIReportDossierComparisonRegistrySchema(t *testing.T) {
	e, registry, m, _ := newComparisonRegistryEnv(t, sql.LevelReadCommitted)
	reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
	comparisonRegistrySave(t, e, registry, m)
	for _, query := range []string{`UPDATE tm_ai_dossier_support_plans SET updated_at=updated_at`, `DELETE FROM tm_ai_dossier_support_plans`} {
		_, err := e.db.ExecContext(e.ctx, query)
		var me *mysql.MySQLError
		if !errors.As(err, &me) || me.Number != 1644 {
			t.Fatal("清单不可变保护失效", err)
		}
	}
	for i, mode := range []string{"unknown_protocol", "light", "missing_card", "duplicate_card", "reverse_keys", "unknown_focus", "null_focus", "missing_focus", "wrong_step", "fraction_step", "missing_model", "model_type", "foreign_version"} {
		t.Run(mode, func(t *testing.T) {
			var doc map[string]any
			if json.Unmarshal(m.Canonical(), &doc) != nil {
				t.Fatal("清单解析失败")
			}
			component := task.StageDigest([]byte("test-only-schema-" + mode))
			doc["componentDigest"] = component
			cfg := doc["config"].(map[string]any)
			kind, version := 2, int64(2)
			switch mode {
			case "unknown_protocol":
				doc["protocol"] = "test-only-unknown"
			case "light":
				kind = 1
				doc["analysisType"] = 1
			case "missing_card":
				doc["keys"] = []string{"comparison_a"}
			case "duplicate_card":
				doc["keys"] = []string{"comparison_a", "comparison_a"}
			case "reverse_keys":
				doc["keys"] = []string{"comparison_b", "comparison_a"}
			case "unknown_focus":
				cfg["focus"] = "auto"
			case "null_focus":
				cfg["focus"] = nil
			case "missing_focus":
				delete(cfg, "focus")
			case "wrong_step":
				cfg["reviewStep"] = 4
			case "fraction_step":
				cfg["reviewStep"] = 5.5
			case "missing_model":
				delete(cfg, "model")
			case "model_type":
				cfg["model"] = "test-only"
			case "foreign_version":
				version = 999
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			at := time.UnixMilli(m.Identity().CreatedAt).UTC()
			_, err = e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_plans(component_digest,theme_id,version_no,version_digest,analysis_type,manifest_digest,canonical,compiled_at,created_at,updated_at) VALUES(?,1,?,?,?,?,?,?,?,?)`, component, version, m.DraftDigest(), kind, task.StageDigest(raw), raw, at, at, at)
			var me *mysql.MySQLError
			expected := uint16(3819)
			if mode == "foreign_version" {
				expected = 1452
			}
			if !errors.As(err, &me) || me.Number != expected {
				t.Fatal("新协议真实约束没有拒写", i, err)
			}
		})
	}
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 {
		t.Fatal("失败探针留下清单")
	}
	requireComparisonNoSideEffects(t, e)
}

// 取消不能被重建吞掉或返回已准备的私有输入。
func TestIntegrationAIReportDossierComparisonRegistryCancellation(t *testing.T) {
	e, registry, m, _ := newComparisonRegistryEnv(t, sql.LevelReadCommitted)
	reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
	comparisonRegistrySave(t, e, registry, m)
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	if p, err := registry.PrepareComparisonForScope(ctx, comparisonRegistryScope(), m.ComponentDigest()); p != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("取消仍提供输入", err)
	}
	requireComparisonNoSideEffects(t, e)
}

func TestIntegrationAIReportDossierComparisonRegistryReader(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, registry, m, _ := newComparisonRegistryEnv(t, level)
			reportDDL(t, e.pointsTestEnv, "20261102_register_dossier_card_comparison.sql")
			comparisonRegistrySave(t, e, registry, m)
			var seed [16]byte
			if _, err := rand.Read(seed[:]); err != nil {
				t.Fatal(err)
			}
			user, password := "compare_"+hex.EncodeToString(seed[:8]), hex.EncodeToString(seed[:])
			// 账号与口令均为本测试生成的十六进制，不接用户输入，不输出凭据。
			account := "'" + user + "'@'127.0.0.1'"
			if _, err := e.conn.ExecContext(e.ctx, "CREATE USER "+account+" IDENTIFIED BY '"+password+"'"); err != nil {
				t.Fatal("创建两卡只读账号失败")
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if _, err := e.conn.ExecContext(ctx, "DROP USER "+account); err != nil {
					t.Error("清理两卡只读账号失败")
				}
			})
			tables := []string{"addon_member", "addon_quant_theme", "tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies", "tm_ai_dossier_support_plans", "tm_ai_dossier_support_uses"}
			for _, table := range tables {
				e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
			}
			cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
			if err != nil {
				t.Fatal("隔离连接配置无效")
			}
			cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
			read := func() error {
				db, err := sql.Open("mysql", cfg.FormatDSN())
				if err != nil {
					return err
				}
				defer db.Close()
				reader, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(db, level), false)
				if err != nil {
					return err
				}
				p, err := reader.PrepareComparisonForScope(e.ctx, comparisonRegistryScope(), m.ComponentDigest())
				if err != nil {
					if p != nil {
						return errCheckTest
					}
					return err
				}
				if !p.Valid() {
					return errCheckTest
				}
				return nil
			}
			t.Run("只读连接实际重建", func(t *testing.T) {
				if err := read(); err != nil {
					t.Fatal("SELECT权限不能重建", err)
				}
			})
			t.Run("只读服务拒绝登记", func(t *testing.T) {
				reader, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(e.db, level), false)
				if err != nil {
					t.Fatal(err)
				}
				if got, err := reader.RegisterComparisonInternal(e.ctx, m); got != "" || err == nil {
					t.Fatal("只读服务允许登记")
				}
			})
			for _, table := range tables {
				t.Run(table, func(t *testing.T) {
					e.mustExec("REVOKE SELECT ON " + e.database + "." + table + " FROM " + account)
					err := read()
					var me *mysql.MySQLError
					if !errors.As(err, &me) || me.Number != 1142 && me.Number != 1143 {
						t.Fatal("缺少表权限仍重建或错误类别不符", table, err)
					}
					e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
					if err := read(); err != nil {
						t.Fatal("恢复表权限后不能重建", err)
					}
				})
			}
			requireComparisonNoSideEffects(t, e)
		})
	}
}
