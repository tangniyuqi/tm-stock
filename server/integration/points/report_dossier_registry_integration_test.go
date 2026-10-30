//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 真库保存清单和当前AI用途，完整编排/模型质量/硬限额批准仍为显著测试控制器。
// HTTP身份使用测试签发JWT，前置阶段合成；不证明原密码登录、真实模型或公众读取。
func newDossierRegistryEnv(t *testing.T, level sql.IsolationLevel) (*supportSQLEnv, *service.AIDossierSupportRegistryService, *service.AIDossierSupportExecutionControl, *dossier.SupportManifest) {
	t.Helper()
	e := newSupportSQLEnv(t, level, 3, supportSQLReply("partial", "partial_support"))
	reportDDL(t, e.pointsTestEnv, "20261030_register_tm_ai_dossier_support_plans.sql")
	r, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(e.db, level), true)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := e.control.dossiers.ReadVersions(e.ctx, "test-member-1", 1, 1, []int64{1})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := dossier.PrepareSupport(e.ctx, e.control.cfg, e.ctl.p.SubjectName, bound[0], []sources.Snapshot{e.snap}, []string{"test_card"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepared.Manifest(1)
	if err != nil || m.ComponentDigest() != e.ctl.p.DossierSupportDigest {
		t.Fatal("原首次准入未绑定登记组件", err)
	}
	ctl := registryExecutionControl(t, e, level)
	return e, r, ctl, m
}
func registryExecutionControl(t *testing.T, e *supportSQLEnv, level sql.IsolationLevel) *service.AIDossierSupportExecutionControl {
	t.Helper()
	reader, err := service.NewAIDossierSupportRegistryService(repository.NewAIDossierSupportRepo(e.db, level), false)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := service.NewAIReportCallAuditService(e.store)
	if err != nil {
		t.Fatal(err)
	}
	ctl, err := service.NewAIDossierSupportExecutionControl(reader, e.repo, e.ctl, audit)
	if err != nil {
		t.Fatal(err)
	}
	return ctl
}
func registryWorker(t *testing.T, e *supportSQLEnv, control service.AIDossierSupportControl) *service.AIDossierSupportWorker {
	t.Helper()
	w, err := service.NewAIDossierSupportWorker(e.tasks, e.store, e.model, control, e.owner, service.AIReportCheckOptions{Enabled: true, Lease: task.RecoveryOptions{Lease: time.Minute, Heartbeat: 100 * time.Millisecond, Timeout: 10 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func registrySave(t *testing.T, e *supportSQLEnv, r *service.AIDossierSupportRegistryService, m *dossier.SupportManifest, enabled bool) {
	t.Helper()
	if digest, err := r.RegisterInternal(e.ctx, m); err != nil || digest != m.ComponentDigest() {
		t.Fatal("登记失败", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_uses(component_digest,approval_ref,valid_until,created_at,updated_at) VALUES(?,'test-fixture:ai-use',UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, m.ComponentDigest()); err != nil {
		t.Fatal(err)
	}
	if enabled {
		e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
	}
}

func TestIntegrationAIReportDossierRegistryLifecycle(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e, r, ctl, m := newDossierRegistryEnv(t, level)
			t.Run("并发同体登记", func(t *testing.T) {
				var wg sync.WaitGroup
				errs := make(chan error, 6)
				for i := 0; i < 6; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); _, err := r.RegisterInternal(e.ctx, m); errs <- err }()
				}
				wg.Wait()
				close(errs)
				for err := range errs {
					if err != nil {
						t.Fatal(err)
					}
				}
				if e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_uses`) != 0 {
					t.Fatal("登记重复或自动批准AI用途")
				}
			})
			t.Run("没有AI用途拒绝", func(t *testing.T) {
				if p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID); p != nil || err == nil {
					t.Fatal("只读权代替AI用途")
				}
			})
			registrySave(t, e, r, m, false)
			t.Run("用途默认关闭", func(t *testing.T) {
				if p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID); p != nil || err == nil {
					t.Fatal("未启用用途仍重建")
				}
			})
			e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
			t.Run("原任务重建并存证", func(t *testing.T) {
				p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID)
				if err != nil || p.InputDigest() != e.plan.InputDigest() {
					t.Fatal("真库清单未重建原计划", err)
				}
				got, err := registryWorker(t, e, ctl).RunOwned(e.ctx, e.request, p)
				if err != nil || got.Review == nil || got.Task.Outcome != "dossier_support_recorded" || got.Review.Results()[0].Verdict != "partial" {
					t.Fatal("真实登记控制器未完成核验", err)
				}
			})
			t.Run("丢弃内存后只读重建回放", func(t *testing.T) {
				fresh := registryExecutionControl(t, e, level)
				p, err := fresh.ExecutionPlan(e.ctx, 1, e.v.ID)
				if err != nil {
					t.Fatal(err)
				}
				got, err := registryWorker(t, e, fresh).RunOwned(e.ctx, e.request, p)
				if err != nil || got.Review == nil || p.InputDigest() != e.request.InputDigest || e.server.Count() != 1 {
					t.Fatal("新装配不能原键回放", err)
				}
			})
			t.Run("撤回用途拒绝缓存恢复", func(t *testing.T) {
				e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=0`)
				if p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID); p != nil || err == nil {
					t.Fatal("撤回后仍提供计划")
				}
				if got, err := registryWorker(t, e, ctl).RunOwned(e.ctx, e.request, e.plan); err == nil || got.Review != nil {
					t.Fatal("旧内存计划绕过撤回")
				}
				e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
				if got, err := registryWorker(t, e, ctl).RunOwned(e.ctx, e.request, e.plan); err != nil || got.Review == nil || e.server.Count() != 1 {
					t.Fatal("恢复用途后不能复用原捕获", err)
				}
			})
			if e.count(t, `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=?`, e.v.ID) != 2 {
				t.Fatal("清单重建丢调用审计")
			}
			e.assertUnsettled(t)
		})
	}
}

type registryAfterCall struct {
	*service.AIDossierSupportExecutionControl
	db *sql.DB
}

func (c registryAfterCall) Finish(ctx context.Context, r task.StageRequest, p provider.Response, e error) error {
	if err := c.AIDossierSupportExecutionControl.Finish(ctx, r, p, e); err != nil {
		return err
	}
	_, err := c.db.ExecContext(ctx, `UPDATE tm_ai_dossier_support_uses SET enabled=0`)
	return err
}

func TestIntegrationAIReportDossierRegistryBoundaries(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"missing_manifest", "expired_use", "source", "license", "grant", "publication", "member", "theme", "full_profile", "cross_member", "metadata", "strategy", "after_call"} {
				t.Run(mode, func(t *testing.T) {
					e, r, ctl, m := newDossierRegistryEnv(t, level)
					registrySave(t, e, r, m, true)
					uid := int64(1)
					switch mode {
					case "missing_manifest":
						e.mustExec(`DROP TABLE tm_ai_dossier_support_uses;DROP TABLE tm_ai_dossier_support_plans`)
					case "expired_use":
						e.mustExec(`UPDATE tm_ai_dossier_support_uses SET created_at=UTC_TIMESTAMP(3)-INTERVAL 2 HOUR,valid_until=UTC_TIMESTAMP(3)-INTERVAL 1 HOUR`)
					case "source":
						e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
					case "license":
						e.mustExec(`UPDATE tm_ai_sources SET license_ref='test-fixture:changed'`)
					case "grant":
						e.mustExec(`UPDATE tm_ai_dossier_grants SET enabled=0`)
					case "publication":
						e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0`)
					case "member":
						e.mustExec(`UPDATE addon_member SET status=0 WHERE id=1`)
					case "theme":
						e.mustExec(`UPDATE addon_quant_theme SET status=0 WHERE id=1`)
					case "full_profile":
						e.ctl.p.ExecutionDigest = task.StageDigest([]byte("test-only-changed-full-profile"))
					case "cross_member":
						uid = 2
					case "metadata":
						e.mustExec(`DROP TRIGGER tm_ai_dossier_support_no_update;UPDATE tm_ai_dossier_support_plans SET compiled_at=compiled_at+INTERVAL 1000 MICROSECOND`)
					case "strategy":
						var doc map[string]json.RawMessage
						if err := json.Unmarshal(m.Canonical(), &doc); err != nil {
							t.Fatal(err)
						}
						// 保持原规范字段顺序，只改配置版本并更新独立清单哈希；原组件仍不能冒充新策略。
						raw := []byte(strings.Replace(string(m.Canonical()), `"Version":"test-only-v1"`, `"Version":"test-only-v2"`, 1))
						if string(raw) == string(m.Canonical()) {
							t.Fatal("策略篡改未发生")
						}
						e.mustExec(`DROP TRIGGER tm_ai_dossier_support_no_update`)
						if _, err := e.db.ExecContext(e.ctx, `UPDATE tm_ai_dossier_support_plans SET canonical=?,manifest_digest=?`, raw, task.StageDigest(raw)); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "after_call" {
						w := registryWorker(t, e, registryAfterCall{AIDossierSupportExecutionControl: ctl, db: e.db})
						if got, err := w.RunOwned(e.ctx, e.request, e.plan); err == nil || got.Review != nil {
							t.Fatal("调用后撤回仍交付")
						}
						if e.server.Count() != 1 || e.count(t, `SELECT COUNT(*) FROM tm_ai_report_call_audits WHERE task_id=?`, e.v.ID) != 2 || e.count(t, `SELECT COUNT(*) FROM tm_ai_task_step_results WHERE task_id=? AND step_no=3`, e.v.ID) != 0 {
							t.Fatal("撤回后丢审计或存储结果")
						}
						e.mustExec(`UPDATE tm_ai_dossier_support_uses SET enabled=1`)
						if got, err := registryWorker(t, e, ctl).RunOwned(e.ctx, e.request, e.plan); err != nil || got.Review != nil || got.Task.Outcome != "response_unknown" || e.server.Count() != 1 {
							t.Fatal("用途恢复重发未知调用", err)
						}
					} else {
						if p, err := ctl.ExecutionPlan(e.ctx, uid, e.v.ID); p != nil || err == nil {
							t.Fatal("失效登记或权限仍重建")
						}
						if e.server.Count() != 0 {
							t.Fatal("重建执行了模型")
						}
					}
					e.assertUnsettled(t)
				})
			}
		})
	}
}

type registryUnknownStore struct {
	dossier.SupportRegistry
	once atomic.Bool
}

func (s *registryUnknownStore) WithinSupport(ctx context.Context, write bool, fn func(dossier.SupportRegistryTx) error) error {
	if err := s.SupportRegistry.WithinSupport(ctx, write, fn); err != nil {
		return err
	}
	if write && s.once.CompareAndSwap(false, true) {
		return errCheckTest
	}
	return nil
}

type registryClockStore struct{ dossier.SupportRegistry }
type registryClockTx struct {
	dossier.SupportRegistryTx
	db *registryClockDossier
}
type registryClockDossier struct {
	dossier.Tx
	calls int
}

func (s registryClockStore) WithinSupport(ctx context.Context, write bool, fn func(dossier.SupportRegistryTx) error) error {
	return s.SupportRegistry.WithinSupport(ctx, write, func(tx dossier.SupportRegistryTx) error {
		return fn(registryClockTx{SupportRegistryTx: tx, db: &registryClockDossier{Tx: tx.Dossiers()}})
	})
}
func (t registryClockTx) Dossiers() dossier.Tx { return t.db }
func (t *registryClockDossier) Now(ctx context.Context) (time.Time, error) {
	at, err := t.Tx.Now(ctx)
	t.calls++
	if t.calls >= 4 {
		at = at.Add(2 * time.Hour)
	}
	return at, err
}

func TestIntegrationAIReportDossierRegistryStorage(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"unknown_registration", "readonly", "immutable", "shared_use_lock", "end_clock_expiry"} {
				t.Run(mode, func(t *testing.T) {
					e, r, ctl, m := newDossierRegistryEnv(t, level)
					store := repository.NewAIDossierSupportRepo(e.db, level)
					if mode == "unknown_registration" {
						unknown := &registryUnknownStore{SupportRegistry: store}
						s, err := service.NewAIDossierSupportRegistryService(unknown, true)
						if err != nil {
							t.Fatal(err)
						}
						if digest, err := s.RegisterInternal(e.ctx, m); err == nil || digest != "" || !unknown.once.Load() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 {
							t.Fatal("提交未知仍报告成功或未真正提交")
						}
						if digest, err := r.RegisterInternal(e.ctx, m); err != nil || digest != m.ComponentDigest() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_uses`) != 0 {
							t.Fatal("原登记未恢复或补造许可", err)
						}
						e.assertUnsettled(t)
						return
					}
					registrySave(t, e, r, m, true)
					switch mode {
					case "readonly":
						reader, err := service.NewAIDossierSupportRegistryService(store, false)
						if err != nil {
							t.Fatal(err)
						}
						if digest, err := reader.RegisterInternal(e.ctx, m); err == nil || digest != "" {
							t.Fatal("只读服务允许登记")
						}
						err = store.WithinSupport(e.ctx, false, func(tx dossier.SupportRegistryTx) error {
							r, err := tx.SupportManifest(e.ctx, m.ComponentDigest())
							if err != nil {
								return err
							}
							if tx.InsertSupportManifest(e.ctx, r) == nil || tx.LockSupportTheme(e.ctx, 1) == nil || tx.Dossiers().Insert(e.ctx, dossier.StoredVersion{}) == nil {
								return errCheckTest
							}
							return nil
						})
						if err != nil {
							t.Fatal("只读事务保护错误", err)
						}
					case "immutable":
						for _, query := range []string{`UPDATE tm_ai_dossier_support_plans SET manifest_digest=REPEAT('f',64)`, `DELETE FROM tm_ai_dossier_support_plans`} {
							_, err := e.db.ExecContext(e.ctx, query)
							var me *mysql.MySQLError
							if !errors.As(err, &me) || me.Number != 1644 {
								t.Fatal("清单缺少实际不可变保护", err)
							}
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
							t.Fatal("用途锁没有阻止撤回写入", err)
						}
					case "end_clock_expiry":
						reader, err := service.NewAIDossierSupportRegistryService(registryClockStore{SupportRegistry: store}, false)
						if err != nil {
							t.Fatal(err)
						}
						audit, err := service.NewAIReportCallAuditService(e.store)
						if err != nil {
							t.Fatal(err)
						}
						ctl, err = service.NewAIDossierSupportExecutionControl(reader, e.repo, e.ctl, audit)
						if err != nil {
							t.Fatal(err)
						}
						if p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID); p != nil || err == nil {
							t.Fatal("末尾时钟跳变未拒绝")
						}
					}
					if mode != "end_clock_expiry" {
						if p, err := ctl.ExecutionPlan(e.ctx, 1, e.v.ID); p == nil || err != nil {
							t.Fatal("正常重建回归失败", err)
						}
					}
					e.assertUnsettled(t)
				})
			}
		})
	}
}

func TestIntegrationAIReportDossierRegistrySchema(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, mode := range []string{"null_protocol", "null_component", "null_draft_digest", "null_config", "null_keys", "null_draft", "empty_keys", "wrong_key_case", "wrong_analysis", "bad_component", "bad_manifest_hash", "wrong_version_hash", "not_object", "oversize", "future_compiled"} {
				t.Run(mode, func(t *testing.T) {
					e, r, _, m := newDossierRegistryEnv(t, level)
					var doc map[string]any
					if err := json.Unmarshal(m.Canonical(), &doc); err != nil {
						t.Fatal(err)
					}
					component, versionDigest, kind := m.ComponentDigest(), m.DraftDigest(), m.AnalysisType()
					created := time.UnixMilli(m.Identity().CreatedAt + 5000)
					compiled := time.UnixMilli(m.Identity().CreatedAt)
					wantCode := uint16(3819)
					switch mode {
					case "null_protocol":
						doc["protocol"] = nil
					case "null_component":
						doc["componentDigest"] = nil
					case "null_draft_digest":
						doc["draftDigest"] = nil
					case "null_config":
						doc["config"] = nil
					case "null_keys":
						doc["keys"] = nil
					case "null_draft":
						doc["draft"] = nil
					case "empty_keys":
						doc["keys"] = []any{}
					case "wrong_key_case":
						doc["Protocol"] = doc["protocol"]
						delete(doc, "protocol")
					case "wrong_analysis":
						kind = 2
					case "bad_component":
						component = "wrong"
					case "wrong_version_hash":
						versionDigest = strings.Repeat("f", 64)
						doc["draftDigest"] = versionDigest
						wantCode = 1452
					case "future_compiled":
						compiled = created.Add(time.Millisecond)
					}
					raw, err := json.Marshal(doc)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "not_object" {
						raw = []byte(`[]`)
					}
					if mode == "oversize" {
						raw = append([]byte(strings.Repeat(" ", dossier.MaxSupportManifestBytes+1)), raw...)
					}
					manifestHash := task.StageDigest(raw)
					if mode == "bad_manifest_hash" {
						manifestHash = "wrong"
					}
					_, err = e.db.ExecContext(e.ctx, `INSERT INTO tm_ai_dossier_support_plans(component_digest,theme_id,version_no,version_digest,analysis_type,manifest_digest,canonical,compiled_at,created_at,updated_at) VALUES(?,1,1,?,?,?,?,?,?,?)`, component, versionDigest, kind, manifestHash, raw, compiled, created, created)
					var me *mysql.MySQLError
					if !errors.As(err, &me) || me.Number != wantCode || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 0 {
						t.Fatal("登记结构未按指定约束拒绝", mode, err)
					}
					if digest, err := r.RegisterInternal(e.ctx, m); err != nil || digest != m.ComponentDigest() || e.queryInt(`SELECT COUNT(*) FROM tm_ai_dossier_support_plans`) != 1 {
						t.Fatal("结构正例被误拒", err)
					}
					e.assertUnsettled(t)
				})
			}
		})
	}
}
