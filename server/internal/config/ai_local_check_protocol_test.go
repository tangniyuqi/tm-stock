package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
)

func TestReportCheckProtocolLocalConfig(t *testing.T) {
	for _, mode := range []string{"legacy", "paired", "unknown", "null", "alias", "duplicate", "number", "changed"} {
		t.Run(mode, func(t *testing.T) {
			runtime, file := localPlansFixture()
			if mode != "legacy" {
				file.Plans[0].CheckProtocol = finalreport.PairedCheckProtocol
			}
			if mode == "unknown" {
				file.Plans[0].CheckProtocol = "unknown"
			}
			raw := mustLocalJSON(t, file)
			field := []byte(`"check_protocol":"` + finalreport.PairedCheckProtocol + `"`)
			switch mode {
			case "legacy":
				if bytes.Contains(raw, []byte("check_protocol")) {
					t.Fatal("旧计划编码增加空协议字段")
				}
			case "null":
				raw = bytes.Replace(raw, field, []byte(`"check_protocol":null`), 1)
			case "alias":
				raw = bytes.Replace(raw, []byte(`"check_protocol"`), []byte(`"CheckProtocol"`), 1)
			case "duplicate":
				raw = bytes.Replace(raw, field, append(append([]byte{}, field...), append([]byte(","), field...)...), 1)
			case "number":
				raw = bytes.Replace(raw, field, []byte(`"check_protocol":2`), 1)
			}
			path := filepath.Join(t.TempDir(), "plans.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			source, err := NewAILocalPlanSource(path, runtime)
			valid := mode == "legacy" || mode == "paired" || mode == "changed"
			if (err == nil) != valid {
				t.Fatal("判定协议本地配置边界不符")
			}
			if mode == "changed" {
				file.Plans[0].CheckProtocol = ""
				if err := os.WriteFile(path, mustLocalJSON(t, file), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := source.Current(context.Background()); err == nil {
					t.Fatal("运行中协议变更未撤销原来源身份")
				}
			}
		})
	}
}
