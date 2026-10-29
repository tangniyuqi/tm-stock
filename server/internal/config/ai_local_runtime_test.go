package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const localRuntimeFixture = `{"version":1,"environment":"local-isolated","instance_id":"0123456789abcdef0123456789abcdef","database":"tm_ai_local_0123456789abcdef0123456789abcdef","listen":"127.0.0.1:8099","real_calls":false,"poll_interval_ms":1000,"batch_size":10}`

func TestAILocalRuntimeStrictFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	for name, raw := range map[string]string{
		"正常":   localRuntimeFixture,
		"重复":   strings.Replace(localRuntimeFixture, `"version":1`, `"version":1,"version":1`, 1),
		"大小写":  strings.Replace(localRuntimeFixture, `"version"`, `"Version"`, 1),
		"未知密钥": strings.Replace(localRuntimeFixture, `"version":1`, `"version":1,"api_key":"forbidden_value"`, 1),
		"缺少开关": strings.Replace(localRuntimeFixture, `"real_calls":false,`, ``, 1),
		"空开关":  strings.Replace(localRuntimeFixture, `"real_calls":false`, `"real_calls":null`, 1),
		"真实调用": strings.Replace(localRuntimeFixture, `"real_calls":false`, `"real_calls":true`, 1),
		"非隔离":  strings.Replace(localRuntimeFixture, `local-isolated`, `production`, 1),
		"公网":   strings.Replace(localRuntimeFixture, `127.0.0.1:8099`, `0.0.0.0:8099`, 1),
		"尾随":   localRuntimeFixture + `{}`,
		"过大":   strings.Repeat(" ", 8193),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			c, err := ReadAILocalRuntime(path)
			if (err == nil) != (name == "正常") {
				t.Fatalf("配置接受边界错误: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "forbidden_value") {
				t.Fatal("错误泄漏配置内容")
			}
			if name == "正常" && c.Database == "" {
				t.Fatal("缺少库身份")
			}
		})
	}
	if _, err := ReadAILocalRuntime("relative.json"); err == nil {
		t.Fatal("接受相对路径")
	}
	if _, err := ReadAILocalRuntime("//example.invalid/runtime.json"); err == nil {
		t.Fatal("接受网络共享路径")
	}
}

func TestAILocalRuntimeDSNAndDefaultOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, []byte(localRuntimeFixture), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := ReadAILocalRuntime(path)
	if err != nil {
		t.Fatal(err)
	}
	valid := "local:private_fixture@tcp(127.0.0.1:3307)/" + c.Database + "?parseTime=true"
	if err := c.CheckDSN(valid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(valid, "127.0.0.1", "localhost", 1), strings.Replace(valid, c.Database, "production", 1), strings.Replace(valid, "parseTime=true", "parseTime=false", 1), strings.Replace(valid, "127.0.0.1", "192.0.2.1", 1), "private_fixture"} {
		if err := c.CheckDSN(bad); err == nil || strings.Contains(err.Error(), "private_fixture") {
			t.Fatal("DSN边界或脱敏失败")
		}
	}
	t.Setenv("TM_AI_LOCAL_CONFIG", "")
	if c, err := LoadAILocalRuntime(""); c != nil || err != nil {
		t.Fatal("默认关闭读取了配置", err)
	}
	t.Setenv("TM_AI_LOCAL_CONFIG", path)
	if _, err := LoadAILocalRuntime(valid); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAILocalRuntime("private_fixture"); err == nil {
		t.Fatal("错误DSN未拒绝")
	}
}
