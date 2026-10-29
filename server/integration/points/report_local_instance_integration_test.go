//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func TestIntegrationAIReportLocalInstance(t *testing.T) {
	e, _ := setupPointsLedger(t)
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(random[:])
	name := "tm_ai_local_" + id
	// 只创建/删除本例刚生成的库；不触及DSN原目标库。
	e.mustExec("CREATE DATABASE `" + name + "`")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := e.conn.ExecContext(ctx, "DROP DATABASE `"+name+"`"); err != nil {
			t.Error("清理本例隔离库失败")
		}
	})
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试DSN无效")
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("本地实例连接失败")
	}
	t.Cleanup(func() { db.Close() })
	verify := repository.NewAILocalInstanceVerifier(db)
	file := filepath.Join(t.TempDir(), "runtime.json")
	meta, err := json.Marshal(config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: name, Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, meta, 0600); err != nil {
		t.Fatal(err)
	}
	command := func(t *testing.T, want bool) {
		t.Helper()
		cmd := exec.CommandContext(e.ctx, "go", "run", "./cmd/ai-worker", "check-local-db", "-config", file)
		cmd.Dir = filepath.Join("..", "..")
		// 子进程仅保留编译/系统环境与本次隔离DSN，不继承宿主模型或产品密钥。
		for _, key := range []string{"SystemRoot", "WINDIR", "PATH", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP", "HOME", "GOCACHE", "GOPATH", "GOMODCACHE", "CGO_ENABLED", "GOFLAGS", "GOMAXPROCS"} {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
		cmd.Env = append(cmd.Env, "TM_DB_DSN="+cfg.FormatDSN())
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatal("实际worker实例预检进程结果不符")
		}
		if want && !strings.Contains(string(out), "实例标记核验通过") {
			t.Fatal("缺实际进程核验结果")
		}
	}
	if err := verify.Verify(e.ctx, id); err == nil {
		t.Fatal("无标记也放行")
	}
	if _, err := db.ExecContext(e.ctx, `CREATE TABLE tm_ai_local_instance(instance_id varchar(32), database_name varchar(64), server_uuid varchar(36), environment varchar(32), contains_real_users int) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	var uuid string
	if err := db.QueryRowContext(e.ctx, "SELECT @@server_uuid").Scan(&uuid); err != nil {
		t.Fatal(err)
	}
	insert := func(instance, database, server, environment string, real int) {
		t.Helper()
		if _, err := db.ExecContext(e.ctx, "INSERT INTO tm_ai_local_instance VALUES(?,?,?,?,?)", instance, database, server, environment, real); err != nil {
			t.Fatal(err)
		}
	}
	if err := verify.Verify(e.ctx, id); err == nil {
		t.Fatal("空标记也放行")
	}
	for _, mode := range []string{"匹配", "错实例", "错库", "错服务", "非隔离", "真实用户", "多行"} {
		t.Run(mode, func(t *testing.T) {
			if _, err := db.ExecContext(e.ctx, "DELETE FROM tm_ai_local_instance"); err != nil {
				t.Fatal(err)
			}
			i, n, u, env, real := id, name, uuid, "local-isolated", 0
			switch mode {
			case "错实例":
				i = strings.Repeat("0", 32)
			case "错库":
				n = "other"
			case "错服务":
				u = strings.Repeat("0", 36)
			case "非隔离":
				env = "production"
			case "真实用户":
				real = 1
			}
			insert(i, n, u, env, real)
			if mode == "多行" {
				insert(i, n, u, env, real)
			}
			err := verify.Verify(e.ctx, id)
			if (err == nil) != (mode == "匹配") {
				t.Fatal("隔离标记边界不符", err)
			}
			if mode == "匹配" || mode == "错实例" {
				command(t, mode == "匹配")
			}
			var count int
			if err := db.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM tm_ai_local_instance").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if (mode == "多行" && count != 2) || (mode != "多行" && count != 1) {
				t.Fatal("核验修改了标记")
			}
		})
	}
	// 原业务库的DATABASE身份不同应拒绝，不借用当前标记证明该库隔离。
	_, wrongDB := setupPointsLedger(t)
	if err := repository.NewAILocalInstanceVerifier(wrongDB).Verify(e.ctx, id); err == nil {
		t.Fatal("跨库未拒绝")
	}
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	if err := verify.Verify(ctx, id); err != context.Canceled {
		t.Fatal("取消未中止", err)
	}
}
