package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mysql "github.com/go-sql-driver/mysql"
	"strings"
	"testing"
)

func TestDossierManagementActorConfiguration(t *testing.T) {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	makeDSN := func(user, addr, db string) string {
		c := mysql.NewConfig()
		c.User = user
		c.Passwd = hex.EncodeToString(seed[:])
		c.Net = "tcp"
		c.Addr = addr
		c.DBName = db
		c.ParseTime = true
		return c.FormatDSN()
	}
	current := makeDSN("management_runtime", "127.0.0.1:3306", "test_only")
	prepare := makeDSN("management_prepare", "127.0.0.1:3306", "test_only")
	reviewers := []AIAdminReviewer{{ActorID: 7}, {ActorID: 8}}
	for _, tc := range []struct {
		name, flag, actors, dsn string
		valid, enabled          bool
	}{
		{"default", "", "", "", true, false}, {"enabled", "true", "7", prepare, true, true},
		{"subset", "true", "8,7", prepare, true, true}, {"flag", "1", "7", prepare, false, false},
		{"implicit_actors", "", "7", "", false, false}, {"implicit_prepare", "", "", prepare, false, false},
		{"missing_actors", "true", "", prepare, false, false}, {"missing_prepare", "true", "7", "", false, false},
		{"unregistered", "true", "9", prepare, false, false}, {"duplicate", "true", "7,7", prepare, false, false},
		{"noncanonical", "true", "07", prepare, false, false}, {"whitespace", "true", "7, 8", prepare, false, false},
		{"wrong_instance", "true", "7", makeDSN("management_prepare", "127.0.0.1:3307", "test_only"), false, false},
		{"wrong_database", "true", "7", makeDSN("management_prepare", "127.0.0.1:3306", "other"), false, false},
		{"same_account", "true", "7", current, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ADMIN_ENABLED", tc.flag)
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ACTORS", tc.actors)
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_PREPARE_DSN", tc.dsn)
			got, err := LoadAIDossierManagement(reviewers, current)
			if (err == nil) != tc.valid || tc.valid && (got != nil) != tc.enabled {
				t.Fatal("管理模式/角色装配判定错误")
			}
			if err != nil && strings.Contains(err.Error(), hex.EncodeToString(seed[:])) {
				t.Fatal("装配错误泄漏凭据")
			}
			if got != nil && strings.Contains(fmt.Sprintf("%v %#v", *got, *got), got.PrepareDSN) {
				t.Fatal("装配摘要泄漏准备连接")
			}
		})
	}
}
