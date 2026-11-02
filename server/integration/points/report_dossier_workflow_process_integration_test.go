//go:build integration

package points_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

// 两个原二进制及原密码登录；CN/模型仍为回环替身，不作浏览器或真实模型证据。
func TestIntegrationAIReportDossierWorkflowProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					verifyLocalAPIWorker(t, level, kind, api, worker, login, localProcessCase{dossier: true, configure: func(t *testing.T, f *localPublicationFixture, execution *config.AILocalExecution) {
						configureDossierLocalFixture(t, f, "partial")
						execution.Model.BaseURL = f.env.server.URL()
					}})
				})
			}
		})
	}
}

func grantDossierProcessPermissions(t *testing.T, e *workflowEnv, dsn string) {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != e.database || !strings.HasPrefix(cfg.User, "local_run_") || len(cfg.User) != 26 {
		t.Fatal("只允许本轮隔离进程账号")
	}
	for _, r := range strings.TrimPrefix(cfg.User, "local_run_") {
		if r < '0' || r > '9' && r < 'a' || r > 'f' {
			t.Fatal("测试账号标识符异常")
		}
	}
	account := "'" + cfg.User + "'@'127.0.0.1'"
	for _, table := range []string{"tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants", "tm_ai_dossier_support_plans", "tm_ai_dossier_support_uses"} {
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
	}
	e.mustExec("GRANT SELECT(id,status,deleted_at) ON " + e.database + ".addon_member TO " + account)
	// 显式六步启动探针复用原调用审计正反例，全部回滚；不开放资料或用途写入。
	e.mustExec("GRANT SELECT,INSERT,UPDATE,DELETE ON " + e.database + ".tm_ai_report_call_audits TO " + account)
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("隔离权限复核连接失败")
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, query := range []string{`CREATE TABLE dossier_run_forbidden(id int)`, `SELECT password FROM addon_member`, `UPDATE tm_ai_sources SET enabled=enabled`, `UPDATE tm_ai_dossier_support_uses SET enabled=enabled`, `UPDATE tm_ai_dossier_support_plans SET component_digest=component_digest`} {
		_, err := conn.ExecContext(ctx, query)
		var denied *mysql.MySQLError
		if !errors.As(err, &denied) || denied.Number != 1044 && denied.Number != 1142 && denied.Number != 1143 {
			t.Fatal("六步进程最终权限超出隔离白名单")
		}
	}
}
