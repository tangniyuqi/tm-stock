//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅启动测试：Ready可选，但所有新任务计划均拒绝，不能作为生产放行。
type testCreateStartupGate struct {
	deny  bool
	calls int
}

func (g *testCreateStartupGate) Ready(ctx context.Context) error {
	g.calls++
	if g.deny {
		return task.ErrAdmissionUnavailable
	}
	return ctx.Err()
}
func (*testCreateStartupGate) Plan(context.Context, task.AdmissionScope) (task.AdmissionPlan, error) {
	return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
}
func (*testCreateStartupGate) Recheck(context.Context, task.AdmissionPlan) error {
	return task.ErrAdmissionUnavailable
}

func (g *testCreateStartupGate) ReadyDeep(ctx context.Context) error { return g.Ready(ctx) }

func TestIntegrationAITaskCreateStartup(t *testing.T) {
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置TM_TEST_DSN，跳过创建启动真库测试")
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
	name := "tm_create_api_" + hex.EncodeToString(random[:8])
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
	for _, migration := range []string{"20261011_create_points_ledger.sql", "20261011_create_tm_ai_tasks.sql", "20261015_create_tm_ai_task_budgets.sql", "20261015_create_tm_ai_task_steps.sql", "20261018_create_tm_ai_task_admissions.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		script := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(raw), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;")
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
	if _, err := ledger.Grant(ctx, points.GrantRequest{UserID: 1, Key: "test_create_startup_grant", BizID: "test-create-startup-grant", Amount: 20, Source: points.Gift, EffectiveAt: time.Now().UTC().Truncate(time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepo(db), ledger)
	if err != nil {
		t.Fatal(err)
	}
	requestKey := "test_create_startup_existing"
	v, err := tasks.Create(ctx, task.CreateRequest{UserID: 1, Key: requestKey, AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 3, MaxCalls: 5, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 120000})
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("k", 40)
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: key, Issuer: "GVA", Audience: "GVA"}}
	claims := jwt.MapClaims{"ID": 1, "UserType": "client", "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
	bearer, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	// 旧库不得接受深度创建；加载新增量后只允许启动，计划仍由拒绝型控制器阻止。
	for _, migration := range []string{"20261016_create_tm_ai_report_commits.sql", "20261023_allow_deep_ai_report_commits.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		script := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(raw), "DELIMITER $$", ""), "DELIMITER ;", ""), "END$$", "END;")
		if _, err := db.ExecContext(ctx, script); err != nil {
			t.Fatal("报告隔离迁移失败", err)
		}
		svc, err := prepareAITaskCreatesWithDeep(ctx, db, read, true, true, &testCreateStartupGate{})
		if migration == "20261016_create_tm_ai_report_commits.sql" {
			if err == nil || svc != nil {
				t.Fatal("旧报告库允许深度创建启动")
			}
		} else if err != nil || svc == nil {
			t.Fatal("升级后深度创建启动失败", err)
		}
	}
	for _, mode := range []string{"all_off", "read_only", "missing_control", "read_off", "denied", "configured", "cancelled_context", "broken_schema"} {
		t.Run(mode, func(t *testing.T) {
			current := read
			enabled := mode != "all_off" && mode != "read_only"
			gate := &testCreateStartupGate{deny: mode == "denied"}
			var ctl service.AITaskAdmissionControl = gate
			callCtx := ctx
			if mode == "all_off" || mode == "read_off" {
				current.Enabled = false
			}
			if mode == "missing_control" {
				ctl = nil
			}
			if mode == "cancelled_context" {
				ended, stop := context.WithCancel(ctx)
				stop()
				callCtx = ended
			}
			if mode == "broken_schema" {
				if _, err := db.ExecContext(ctx, "RENAME TABLE tm_ai_tasks TO test_only_tasks_hidden"); err != nil {
					t.Fatal(err)
				}
			}
			s, err := prepareAITaskCreates(callCtx, db, current, enabled, ctl)
			if mode != "all_off" && mode != "read_only" && mode != "configured" {
				if err == nil || s != nil {
					t.Fatal("缺证明/保护仍开放")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			if err := registerAITaskCreates(mux, db, current, s, enabled); err != nil {
				t.Fatal(err)
			}
			query := func(method, path, body, auth string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, path, strings.NewReader(body))
				r.Header.Set("Authorization", auth)
				r.Header.Set("Idempotency-Key", requestKey)
				if method == "POST" {
					r.Header.Set("Content-Type", "application/json")
				}
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, r)
				return w
			}
			if mode == "all_off" {
				if query("GET", "/api/v1/ai/task-receipt", "", "").Code != 404 {
					t.Fatal("默认开放核查")
				}
				return
			}
			if query("GET", "/api/v1/ai/task-receipt", "", "").Code != 401 {
				t.Fatal("核查不要求身份")
			}
			response := query("GET", "/api/v1/ai/task-receipt", "", "Bearer "+bearer)
			var receipt struct {
				Data dto.AITaskCreation `json:"data"`
			}
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &receipt) != nil || receipt.Data.TaskID != v.TaskID {
				t.Fatal("启动后不能核查原冻结", response.Code, response.Body.String())
			}
			quote := query("POST", "/api/v1/ai/tasks/quote", `{"analysis_type":"light","subject_type":"theme","subject_id":1}`, "Bearer "+bearer)
			if mode == "read_only" && quote.Code != 404 || mode == "configured" && quote.Code != 422 {
				t.Fatal("只读或拒绝计划仍创建", quote.Code)
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tm_points_txns WHERE kind='freeze'").Scan(&count); err != nil || count != 1 {
				t.Fatal("启动/读取新增冻结", err)
			}
		})
	}
}
