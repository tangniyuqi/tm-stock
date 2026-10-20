package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAIAdminConfigFailClosed(t *testing.T) {
	t.Setenv("TM_DB_DSN", "test_user:test_password@tcp(127.0.0.1:3306)/tm_test?parseTime=true")
	t.Setenv("TM_AI_ADMIN_ADDR", "")
	reviewers := []AIAdminReviewer{{ActorID: 7, Token: strings.Repeat("test_", 8)}}
	raw, err := json.Marshal(reviewers)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TM_AI_ADMIN_REVIEWERS", string(raw))
	cfg, err := LoadAIAdmin()
	if err != nil || cfg.Addr != "127.0.0.1:8081" || cfg.Reviewers[0].ActorID != 7 {
		t.Fatalf("合法内部配置失败: %v", err)
	}
	for _, invalid := range []string{"", "null", "[]", "{}", string(raw) + " {}", `[{"actor_id":7,"token":"test_short"}]`, `[{"actor_id":7,"token":"test_short","extra":1}]`} {
		t.Setenv("TM_AI_ADMIN_REVIEWERS", invalid)
		if _, err := LoadAIAdmin(); err == nil {
			t.Fatalf("无效凭据配置被接受: %q", invalid)
		}
	}
	t.Setenv("TM_AI_ADMIN_REVIEWERS", string(raw))
	t.Setenv("TM_DB_DSN", "")
	if _, err := LoadAIAdmin(); err == nil {
		t.Fatal("缺数据库配置未拒绝")
	}
}

func TestAIAdminAddressAndCredentialBounds(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8081", "[::1]:8081"} {
		if err := ValidateAIAdminAddress(addr); err != nil {
			t.Fatal(err)
		}
	}
	for _, addr := range []string{":8081", "0.0.0.0:8081", "[::]:8081", "localhost:8081", "192.168.1.2:8081", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:abc"} {
		if err := ValidateAIAdminAddress(addr); err == nil {
			t.Fatalf("非限定地址被接受: %q", addr)
		}
	}
	valid := AIAdminReviewer{ActorID: 7, Token: strings.Repeat("test_", 8)}
	for _, reviewers := range [][]AIAdminReviewer{nil, {valid, valid}, {{ActorID: 0, Token: valid.Token}}, {{ActorID: 7, Token: strings.Repeat("test_", 52)}}, {{ActorID: 7, Token: valid.Token + "\n"}}, make([]AIAdminReviewer, 101)} {
		if err := ValidateAIAdminReviewers(reviewers); err == nil {
			t.Fatal("错误凭据未拒绝")
		}
	}
}
