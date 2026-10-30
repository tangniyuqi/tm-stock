package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func localFeedbackConfig() (*config.AIAdminConfig, *config.AIL0Config, *config.AILocalRuntime) {
	id := strings.Repeat("a", 32)
	local := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: "tm_ai_local_" + id, Listen: "127.0.0.1:18081", PollIntervalMS: 1000, BatchSize: 1}
	cfg := &config.AIAdminConfig{Addr: local.Listen, DSN: "test_placeholder@tcp(127.0.0.1:3306)/" + local.Database + "?parseTime=true", Reviewers: []config.AIAdminReviewer{{ActorID: 7, Token: strings.Repeat("test_placeholder_", 3)}}}
	return cfg, &config.AIL0Config{}, local
}

func TestLocalFeedbackAdminConfiguration(t *testing.T) {
	for _, mode := range []string{"valid", "missing_runtime", "missing_admin", "missing_l0", "model_enabled", "cross_database", "external_database", "external_listen", "wrong_listen", "real_calls", "bad_reviewer"} {
		t.Run(mode, func(t *testing.T) {
			cfg, l0, local := localFeedbackConfig()
			switch mode {
			case "missing_runtime":
				local = nil
			case "missing_admin":
				cfg = nil
			case "missing_l0":
				l0 = nil
			case "model_enabled":
				l0.Enabled = true
			case "cross_database":
				cfg.DSN = "test_placeholder@tcp(127.0.0.1:3306)/other?parseTime=true"
			case "external_database":
				cfg.DSN = strings.ReplaceAll(cfg.DSN, "127.0.0.1", "db.example.test")
			case "external_listen":
				local.Listen, cfg.Addr = "0.0.0.0:18081", "0.0.0.0:18081"
			case "wrong_listen":
				cfg.Addr = "127.0.0.1:18082"
			case "real_calls":
				local.RealCalls = true
			case "bad_reviewer":
				cfg.Reviewers = nil
			}
			err := validateLocalFeedbackAdmin(cfg, l0, local)
			if (err == nil) != (mode == "valid") || err != nil && strings.Contains(err.Error(), "test_placeholder") {
				t.Fatal("本地管理配置边界不符或泄漏输入")
			}
		})
	}
}

func TestLocalFeedbackAdminFailsBeforeServing(t *testing.T) {
	for _, mode := range []string{"disabled", "invalid_flag", "wrong_instance", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("TM_AI_FEEDBACK_ADMIN_ENABLED", "true")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "disabled" {
				t.Setenv("TM_AI_FEEDBACK_ADMIN_ENABLED", "false")
			}
			if mode == "invalid_flag" {
				t.Setenv("TM_AI_FEEDBACK_ADMIN_ENABLED", "unknown")
			}
			if mode == "cancelled" {
				cancel()
			}
			cfg, l0, local := localFeedbackConfig()
			db := sql.OpenDB(bootstrapConnector{})
			defer db.Close()
			served := false
			err := startAdminWithLocal(ctx, cfg, l0, db, func(context.Context, *http.Server) error { served = true; return nil }, local)
			if err == nil || served {
				t.Fatal("未确认隔离实例仍对外监听")
			}
		})
	}
}
