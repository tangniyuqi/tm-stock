//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 测试始终拒绝发布，不是可挪入产品的放行控制器。
type reportStartupDeny struct{}

func (reportStartupDeny) Recheck(context.Context, service.AIReportPublication) error {
	return finalreport.ErrUnavailable
}

func TestIntegrationAIReportReadStartup(t *testing.T) {
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置TM_TEST_DSN，跳过报告启动真库测试")
	}
	cfg, err := mysqldrv.ParseDSN(dsn)
	if err != nil {
		t.Fatal("测试连接解析失败")
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("管理连接失败")
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "tm_report_api_" + hex.EncodeToString(random[:8])
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatal("隔离库创建失败")
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
			t.Error("隔离库清理失败", err)
		}
	}()
	cfg.DBName = name
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("业务连接失败")
	}
	defer db.Close()
	var admissionDDL string
	for _, migration := range []string{"20261011_create_points_ledger.sql", "20261011_create_tm_ai_tasks.sql", "20261015_create_tm_ai_task_budgets.sql", "20261015_create_tm_ai_task_steps.sql", "20261016_create_tm_ai_report_commits.sql", "20261018_create_tm_ai_task_admissions.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		script := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(b), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;")
		if migration == "20261018_create_tm_ai_task_admissions.sql" {
			admissionDDL = script
		}
		for i := 0; i < 2; i++ {
			if _, err := db.ExecContext(ctx, script); err != nil {
				t.Fatal("隔离迁移失败", err)
			}
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL)`); err != nil {
		t.Fatal(err)
	}
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Grant(ctx, points.GrantRequest{UserID: 1, Key: "test_report_startup_grant", BizID: "test-report-startup-grant", Amount: 20, Source: points.Gift, EffectiveAt: time.Now().UTC().Truncate(time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepo(db), ledger)
	if err != nil {
		t.Fatal(err)
	}
	v, err := tasks.Create(ctx, task.CreateRequest{UserID: 1, Key: "test_report_startup_task", AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 3, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 120000})
	if err != nil {
		t.Fatal(err)
	}
	p, err := finalreport.NewPolicy(finalreport.Config{Ref: "test-only-policy", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{"evidence.example.test"}}, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("k", 40)
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: key, Issuer: "GVA", Audience: "GVA"}}
	for _, mode := range []string{"default_off", "missing_proof", "read_off", "configured", "cancelled_context", "missing_admission", "missing_marker_guard", "broken_schema"} {
		t.Run(mode, func(t *testing.T) {
			current := read
			enabled := mode != "default_off"
			var control service.AIReportReadControl = reportStartupDeny{}
			callCtx := ctx
			if mode == "missing_proof" {
				control = nil
			}
			if mode == "read_off" {
				current.Enabled = false
			}
			if mode == "cancelled_context" {
				ended, stop := context.WithCancel(ctx)
				stop()
				callCtx = ended
			}
			if mode == "missing_admission" {
				if _, err := db.ExecContext(ctx, `RENAME TABLE tm_ai_task_admissions TO test_only_admission_hidden`); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := db.ExecContext(ctx, `RENAME TABLE test_only_admission_hidden TO tm_ai_task_admissions`); err != nil {
						t.Error("恢复隔离测试表失败", err)
					}
				})
			}
			if mode == "missing_marker_guard" {
				if _, err := db.ExecContext(ctx, `DROP TRIGGER tm_ai_task_admission_identity`); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := db.ExecContext(ctx, admissionDDL); err != nil {
						t.Error("恢复隔离测试保护失败", err)
					}
				})
			}
			if mode == "broken_schema" {
				if _, err := db.ExecContext(ctx, `RENAME TABLE tm_ai_report_commits TO test_only_report_hidden`); err != nil {
					t.Fatal(err)
				}
			}
			r, err := prepareAIReportReads(callCtx, db, current, enabled, p, control)
			if mode != "default_off" && mode != "configured" {
				if err == nil || r != nil {
					t.Fatal("缺准入/保护或取消仍启动")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			if err := registerAIReportReads(mux, db, current, r); err != nil {
				t.Fatal(err)
			}
			path := "/api/v1/ai/tasks/" + strconv.FormatInt(v.TaskID, 10) + "/report"
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if mode == "default_off" {
				if w.Code != 404 {
					t.Fatal("默认开放报告")
				}
				return
			}
			if w.Code != 401 {
				t.Fatal("无身份访问报告")
			}
			claims := jwt.MapClaims{"ID": 1, "UserType": "client", "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
			if err != nil {
				t.Fatal(err)
			}
			req = httptest.NewRequest("GET", path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != 422 || strings.Contains(w.Body.String(), "sections") {
				t.Fatal("排队任务被包装成报告", w.Code, w.Body.String())
			}
			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_points_txns WHERE kind='settle'`).Scan(&count); err != nil || count != 0 {
				t.Fatal("启动/读取扣费", err)
			}
		})
	}
}
