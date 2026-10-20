package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestLoadAITaskReadDefaultOffAndStrict(t *testing.T) {
	for _, k := range []string{"TM_AI_TASK_READ_ENABLED", "TM_JWT_SIGNING_KEY", "TM_JWT_SIGNING_KEY_PREVIOUS", "TM_JWT_ISSUER", "TM_JWT_AUDIENCE"} {
		t.Setenv(k, "")
	}
	if c, err := LoadAITaskRead(); err != nil || c.Enabled {
		t.Fatal(c, err)
	}
	t.Setenv("TM_AI_TASK_READ_ENABLED", "yes")
	if _, err := LoadAITaskRead(); err == nil {
		t.Fatal("模糊开关放行")
	}
	t.Setenv("TM_AI_TASK_READ_ENABLED", "true")
	if _, err := LoadAITaskRead(); err == nil {
		t.Fatal("缺少配置放行")
	}
	t.Setenv("TM_JWT_SIGNING_KEY", strings.Repeat("k", 40))
	t.Setenv("TM_JWT_ISSUER", "GVA")
	t.Setenv("TM_JWT_AUDIENCE", "GVA")
	if c, err := LoadAITaskRead(); err != nil || !c.Enabled {
		t.Fatal(c, err)
	}
	c, err := LoadAITaskRead()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{fmt.Sprint(c), fmt.Sprintf("%#v", c)} {
		if strings.Contains(s, c.Auth.SigningKey) {
			t.Fatal("配置日志泄漏密钥")
		}
	}
	t.Setenv("TM_JWT_SIGNING_KEY_PREVIOUS", "short")
	if _, err := LoadAITaskRead(); err == nil {
		t.Fatal("弱旧密钥放行")
	}
	t.Setenv("TM_AI_TASK_READ_ENABLED", "false")
	if c, err := LoadAITaskRead(); err != nil || c.Auth.SigningKey != "" {
		t.Fatal(c, err)
	}
}
