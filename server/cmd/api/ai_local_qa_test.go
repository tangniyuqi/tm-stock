package main

import (
	"context"
	"database/sql"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestLocalQAAssemblyDefaultsAndMissingDependencies(t *testing.T) {
	t.Setenv("TM_AI_LOCAL_QA", "")
	if v, err := prepareAILocalQA(nil, nil, nil, config.AIQAConfig{}); err != nil || v != nil {
		t.Fatal("默认装配不关闭")
	}
	t.Setenv("TM_AI_LOCAL_QA", "relative-forbidden.json")
	db, err := sql.Open("mysql", "test@tcp(127.0.0.1:1)/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, mode := range []string{"context", "database", "runtime", "disabled", "invalid_file"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			conn := db
			runtime := &config.AILocalRuntime{}
			cfg := config.AIQAConfig{Read: true}
			switch mode {
			case "context":
				ctx = nil
			case "database":
				conn = nil
			case "runtime":
				runtime = nil
			case "disabled":
				cfg.Read = false
			}
			if _, err := prepareAILocalQA(ctx, conn, runtime, cfg); err == nil {
				t.Fatal("缺前置条件仍装配")
			}
		})
	}
}
