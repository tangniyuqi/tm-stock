package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDossierManagementMigrationPolicy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20261103_create_tm_ai_dossier_management.sql"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?s)CREATE TRIGGER (\w+) BEFORE (INSERT|UPDATE|DELETE) ON (\w+) FOR EACH ROW\s+((?:BEGIN.*?END\$\$)|(?:SIGNAL[^\r\n]*;))`)
	matches := pattern.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != len(dossierManagementTriggers) {
		t.Fatal("管理迁移触发器数量未与启动策略同步")
	}
	seen := map[string]bool{}
	for _, match := range matches {
		body := strings.TrimSuffix(match[4], "$$")
		found := false
		for _, expected := range dossierManagementTriggers {
			if expected.name == match[1] {
				found = true
				if seen[expected.name] || expected.table != match[3] || expected.event != match[2] || task.StageDigest([]byte(qaCompactSQL(body))) != expected.digest {
					t.Fatal("管理迁移保护体与启动校验不同步", expected.name)
				}
				seen[expected.name] = true
			}
		}
		if !found {
			t.Fatal("管理迁移出现未登记保护")
		}
	}
}
