package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"net/http"
	"testing"
)

func TestDossierManagementAssembly(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	makeDSN := func(user string) string {
		c := mysql.NewConfig()
		c.User = user
		c.Passwd = hex.EncodeToString(seed[:])
		c.Net = "tcp"
		c.Addr = "127.0.0.1:3306"
		c.DBName = "test_only"
		c.ParseTime = true
		return c.FormatDSN()
	}
	cfg := &config.AIAdminConfig{Addr: "127.0.0.1:8081", DSN: makeDSN("runtime"), Reviewers: []config.AIAdminReviewer{{ActorID: 7, Token: hex.EncodeToString(seed[:])}}}
	for _, name := range []string{"local_feedback_conflict", "invalid_operator", "missing_dependencies"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ADMIN_ENABLED", "true")
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ACTORS", "7")
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_PREPARE_DSN", makeDSN("prepare"))
			called := false
			serve := func(context.Context, *http.Server) error { called = true; return nil }
			var err error
			switch name {
			case "local_feedback_conflict":
				err = startAdminWithLocal(context.Background(), cfg, nil, nil, serve, &config.AILocalRuntime{})
			case "invalid_operator":
				t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ACTORS", "8")
				err = startAdminWithLocal(context.Background(), cfg, nil, nil, serve, nil)
			case "missing_dependencies":
				err = startDossierManagementAdmin(context.Background(), cfg, &config.AIDossierManagementConfig{}, nil, serve)
			}
			if err == nil || called {
				t.Fatal("越界装配启动了服务")
			}
		})
	}
}
