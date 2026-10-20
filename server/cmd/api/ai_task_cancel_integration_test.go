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
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅在明确的可丢弃实例中新建随机库，不触碰 DSN 原库；启动装配不监听公网。
func TestIntegrationAITaskCancelStartup(t *testing.T) {
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 TM_TEST_DSN，跳过取消启动真库测试")
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
	database := "tm_cancel_api_" + hex.EncodeToString(random[:8])
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+database+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatal("创建隔离库失败", err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+database+"`"); err != nil {
			t.Error("清理本轮隔离库失败", err)
		}
	}()
	cfg.DBName = database
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("业务连接失败")
	}
	defer db.Close()
	for _, name := range []string{"20261011_create_points_ledger.sql", "20261011_create_tm_ai_tasks.sql", "20261015_create_tm_ai_task_budgets.sql", "20261015_create_tm_ai_task_steps.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
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
	if _, err := db.ExecContext(ctx, "CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3)); INSERT INTO addon_member VALUES(1,1,NULL)"); err != nil {
		t.Fatal(err)
	}
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := ledger.Grant(ctx, points.GrantRequest{UserID: 1, Key: "test_cancel_startup_grant", BizID: "test-cancel-startup-grant", Amount: 20, Source: points.Gift, EffectiveAt: now}); err != nil {
		t.Fatal(err)
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepo(db), ledger)
	if err != nil {
		t.Fatal(err)
	}
	v, err := tasks.Create(ctx, task.CreateRequest{UserID: 1, Key: "test_cancel_startup_task", AnalysisType: 1, SubjectType: task.Theme, SubjectID: 1, PriceVersion: "v1"}, points.Price{Version: "v1", Points: 7}, task.Budget{MaxAttempts: 3, MaxCalls: 4, MaxTokens: 1000, MaxPages: 3, MaxDurationMS: 120000})
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("k", 40)
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: key, Issuer: "GVA", Audience: "GVA"}}
	if err := repository.NewAITaskSchemaVerifier(db).Verify(ctx); err != nil {
		t.Fatal("取消启动健康基线失败", err)
	}
	for _, mode := range []string{"默认关闭", "仅只读", "无效鉴权", "父上下文结束", "受限账号", "保护缺失"} {
		t.Run(mode, func(t *testing.T) {
			mux := http.NewServeMux()
			currentDB := db
			currentRead := read
			enabled := true
			currentCtx := ctx
			if mode == "默认关闭" || mode == "仅只读" {
				enabled = false
			}
			if mode == "仅只读" {
				if err := registerAITaskReads(mux, db, read); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "无效鉴权" {
				currentRead.Auth.SigningKey = "short"
			}
			if mode == "父上下文结束" {
				ended, stop := context.WithCancel(ctx)
				stop()
				currentCtx = ended
			}
			if mode == "保护缺失" {
				if err := repository.NewAITaskSchemaVerifier(db).Verify(ctx); err != nil {
					t.Fatal("破坏前健康基线失败", err)
				}
				if _, err := db.ExecContext(ctx, "DROP TRIGGER tm_ai_step_account"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "受限账号" {
				user := "cancel_it_" + hex.EncodeToString(random[:6])
				pass := hex.EncodeToString(random[:])
				if _, err := admin.ExecContext(ctx, "CREATE USER '"+user+"'@'%' IDENTIFIED BY '"+pass+"'"); err != nil {
					t.Fatal("创建测试受限账号失败")
				}
				defer func() {
					if _, err := admin.ExecContext(ctx, "DROP USER '"+user+"'@'%'"); err != nil {
						t.Error("清理本轮受限账号失败")
					}
				}()
				for _, table := range []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps"} {
					if _, err := admin.ExecContext(ctx, "GRANT SELECT, INSERT, UPDATE, DELETE ON `"+database+"`."+table+" TO '"+user+"'@'%'"); err != nil {
						t.Fatal("授予隔离 DML 失败")
					}
				}
				if _, err := admin.ExecContext(ctx, "GRANT SELECT ON `"+database+"`.addon_member TO '"+user+"'@'%'"); err != nil {
					t.Fatal(err)
				}
				restricted := *cfg
				restricted.User = user
				restricted.Passwd = pass
				currentDB, err = sql.Open("mysql", restricted.FormatDSN())
				if err != nil {
					t.Fatal("受限连接失败")
				}
				defer currentDB.Close()
				if _, err := currentDB.ExecContext(ctx, "CREATE TABLE cancel_forbidden_ddl(id INT)"); err == nil {
					t.Fatal("受限账号有 DDL 权限")
				} else {
					permission, ok := err.(*mysqldrv.MySQLError)
					if !ok || permission.Number != 1142 {
						t.Fatal("不是 DDL 权限拒绝")
					}
				}
			}
			err := registerAITaskCancels(currentCtx, mux, currentDB, currentRead, enabled)
			wantError := mode == "无效鉴权" || mode == "父上下文结束" || mode == "保护缺失"
			if (err != nil) != wantError {
				t.Fatal("取消启动结果不符", err)
			}
			if err != nil && ((cfg.Passwd != "" && strings.Contains(err.Error(), cfg.Passwd)) || strings.Contains(err.Error(), "tm_ai_step_account")) {
				t.Fatal("启动详情泄漏")
			}
			r := httptest.NewRequest("POST", "/api/v1/ai/tasks/"+strconv.FormatInt(v.TaskID, 10)+"/cancel", strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json")
			if mode == "受限账号" {
				claims := jwt.MapClaims{"ID": 1, "UserType": "client", "AuthorityId": 0, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
				token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
				if err != nil {
					t.Fatal(err)
				}
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			want := 404
			if mode == "受限账号" {
				want = 200
			}
			if w.Code != want {
				t.Fatal("启动后取消路由不符", w.Code, w.Body.String())
			}
			if mode == "受限账号" {
				var frozen int
				if err := db.QueryRowContext(ctx, "SELECT frozen FROM tm_points_accounts WHERE user_id=1").Scan(&frozen); err != nil || frozen != 0 {
					t.Fatal("受限装配未释放", err)
				}
			}
		})
	}
}
