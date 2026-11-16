//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/materialpack"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 复用本地实例夹具的随机库、真实标记及迁移；资料初始化沿用 agentMaterialFixture
// 的合成来源和快照口径。原夹具固定非本地库名，故这里只复制必要的资料初始化，
// 不修改已有夹具，不创建任务、不发积分、不装配模型或执行器。
func agentMaterialServiceFixture(t *testing.T) (*pointsTestEnv, repository.AgentMaterialRequest, sources.Snapshot) {
	t.Helper()
	e, db, _, _, _, now := persistentTaskFixtureWithLocal(t, sql.LevelRepeatableRead, true)
	reportDDL(t, e, "20261010_create_ai_source_snapshots.sql")
	e.mustExec("CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL)")
	e.mustExec("CREATE TABLE addon_quant_theme(id bigint unsigned PRIMARY KEY,name varchar(100),status tinyint,deleted_at datetime(3));INSERT INTO addon_quant_theme VALUES(7,'仅测试：合成资料题材',1,NULL)")
	repo := repository.NewAISourceRepo(db)
	expires := now.Add(time.Hour)
	src, err := repo.RegisterSource(e.ctx, sources.Source{Domain: "material.example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试", LicenseRef: "test-fixture:agent-material", LicenseExpires: &expires, MinInterval: time.Second})
	if err != nil {
		t.Fatal("登记合成来源失败", err)
	}
	if err := repo.SetSourceEnabled(e.ctx, src.ID, true); err != nil {
		t.Fatal("启用合成来源失败", err)
	}
	src, err = repo.SourceByID(e.ctx, src.ID)
	if err != nil {
		t.Fatal("回读合成来源失败", err)
	}
	snap, err := sources.BuildSnapshot(src, "https://material.example.test/test-only", sources.MediaPlain, []byte("仅测试：合成资料记载样品处于送样阶段。仅测试：资料未记载后续阶段。"), now.Add(-time.Second), 4096)
	if err != nil {
		t.Fatal("构建合成快照失败", err)
	}
	snap, err = repo.SaveSnapshot(e.ctx, snap)
	if err != nil {
		t.Fatal("保存合成快照失败", err)
	}
	request := repository.AgentMaterialRequest{MemberID: 1, SubjectType: "theme", SubjectID: 7, ValidUntil: now.Add(30 * time.Minute), Evidence: []repository.AgentMaterialEvidence{{SnapshotID: snap.ID, Hash: snap.Hash}}, MaxSources: 1, MaxEvidenceBytes: 4096}
	agentMaterialAssertNoWrites(t, e)
	return e, request, snap
}

// 沿用资料仓储集成测试的随机账号与逐表 SELECT 授权方法；不授予任务或积分权限。
func agentMaterialServiceReader(t *testing.T, e *pointsTestEnv) *sql.DB {
	t.Helper()
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	user, password := "material_"+hex.EncodeToString(seed[:8]), hex.EncodeToString(seed[:])
	account := "'" + user + "'@'%'"
	// SQL 标识符仅由内部随机十六进制生成，不接受请求输入，不记录凭据。
	if _, err := e.conn.ExecContext(e.ctx, "CREATE USER "+account+" IDENTIFIED BY '"+password+"'"); err != nil {
		t.Fatal("创建资料服务只读账号失败")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := e.conn.ExecContext(ctx, "DROP USER "+account); err != nil {
			t.Error("清理资料服务只读账号失败")
		}
	})
	for _, table := range []string{"addon_member", "addon_quant_theme", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies", "tm_ai_local_instance"} {
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试DSN解析失败")
	}
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
	reader, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("资料服务只读连接装配失败")
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error("关闭资料服务只读连接失败")
		}
	})
	if err := reader.PingContext(e.ctx); err != nil {
		t.Fatal("资料服务只读连接失败")
	}
	for _, query := range []string{
		"CREATE TABLE material_service_forbidden(id int)",
		"UPDATE tm_ai_source_bodies SET body_text=body_text WHERE 1=0",
		"UPDATE tm_ai_local_instance SET contains_real_users=contains_real_users WHERE 1=0",
		"INSERT INTO tm_ai_source_snapshots SELECT * FROM tm_ai_source_snapshots WHERE 1=0",
		"SELECT * FROM tm_ai_tasks LIMIT 0", "SELECT * FROM tm_points_accounts LIMIT 0",
	} {
		_, err := reader.ExecContext(e.ctx, query)
		agentMaterialAssertPermissionDenied(t, err)
	}
	return reader
}

func TestIntegrationAIReportAgentMaterialService(t *testing.T) {
	e, request, snap := agentMaterialServiceFixture(t)
	reader := agentMaterialServiceReader(t, e)
	repo := repository.NewAIAgentMaterialRepo(reader)
	id := strings.TrimPrefix(e.database, "tm_ai_local_")
	runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: "tm_ai_local_" + id, Listen: "127.0.0.1:18081", PollIntervalMS: 1000, BatchSize: 1}
	approval := config.AILocalAgentMaterial{
		Version: 1, InstanceID: id, Revision: "v1", Scope: config.AgentMaterialScope,
		IssuedAtMS: snap.FetchedAt.UnixMilli(), ExpiresAtMS: request.ValidUntil.UnixMilli(), Members: []int64{1, 2},
		Use: config.AILocalAgentMaterialUse{DatasetDigest: strings.Repeat("b", 64), ApprovalRef: "local-eval:synthetic-agent-service", Purpose: config.AgentMaterialPurpose,
			Bindings: []config.AILocalAgentMaterialBinding{{SubjectType: request.SubjectType, SubjectID: request.SubjectID, ScopeApproval: "local-eval:synthetic-theme", Evidence: []config.AILocalAgentMaterialEvidence{{SnapshotID: snap.ID, Hash: snap.Hash}}}}},
		Limits: config.AILocalAgentMaterialLimits{MaxSources: request.MaxSources, MaxEvidenceBytes: request.MaxEvidenceBytes},
	}
	path := filepath.Join(t.TempDir(), "synthetic-agent-material.json")
	writeApproval := func(value config.AILocalAgentMaterial) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal("写入合成用途批准失败", err)
		}
	}
	writeApproval(approval)
	source, err := config.NewAILocalAgentMaterialSource(path, runtime)
	if err != nil {
		t.Fatal("装配真实批准来源失败", err)
	}
	svc, err := service.NewAIAgentMaterialService(source, repository.NewAILocalInstanceVerifier(reader), repo)
	if err != nil {
		t.Fatal("装配资料联合链失败", err)
	}
	prepare := func(t *testing.T, member int64) *materialpack.Pack {
		t.Helper()
		pack, err := svc.Prepare(e.ctx, member, request.SubjectType, request.SubjectID)
		if err != nil || pack == nil {
			t.Fatal("真实联合链准备失败", err)
		}
		agentMaterialAssertNoWrites(t, e)
		return pack
	}
	original := prepare(t, 1)
	originalScope, originalSnapshots := original.Scope(), original.Snapshots()
	originalMaterials, originalContent := original.MaterialsDigest(), original.ContentDigest()
	assertOriginal := func(t *testing.T) {
		t.Helper()
		if original.MaterialsDigest() != originalMaterials || original.ContentDigest() != originalContent || original.Scope() != originalScope || !reflect.DeepEqual(original.Snapshots(), originalSnapshots) {
			t.Fatal("复核改写原包")
		}
		agentMaterialAssertNoWrites(t, e)
	}
	recheck := func(t *testing.T, pack *materialpack.Pack, allowed bool) {
		t.Helper()
		err := svc.Recheck(e.ctx, pack)
		if allowed && err != nil {
			t.Fatal("原包合法复核失败", err)
		}
		if !allowed && !errors.Is(err, agents.ErrInvalid) {
			t.Fatalf("变化后未返回统一拒绝：%v", err)
		}
		assertOriginal(t)
	}
	t.Run("首次包", func(t *testing.T) {
		wantDigest, err := approval.MaterialsDigest(approval.Use.Bindings[0])
		if err != nil {
			t.Fatal(err)
		}
		scope := original.Scope()
		snapshots := original.Snapshots()
		if scope.MemberID != 1 || scope.SubjectType != request.SubjectType || scope.SubjectID != request.SubjectID || scope.SubjectName != "仅测试:合成资料题材" || scope.IssuedAtMS != approval.IssuedAtMS || scope.ExpiresAtMS != approval.ExpiresAtMS || scope.ScopeApproval != approval.Use.Bindings[0].ScopeApproval || original.MaterialsDigest() != wantDigest || !agents.ValidDigest(original.ContentDigest()) || len(snapshots) != 1 {
			t.Fatal("首包范围、批准或摘要不匹配")
		}
		if snapshots[0].ID != snap.ID || snapshots[0].Hash != snap.Hash || snapshots[0].Body != snap.Body || !reflect.DeepEqual(snapshots[0].Sentences, snap.Sentences) {
			t.Fatal("首包未携带真实入库快照")
		}
		recheck(t, original, true)
	})
	t.Run("数据库时钟推进且摘要稳定", func(t *testing.T) {
		before, err := repo.Load(e.ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		// 等待真实数据库时钟自然推进，不注入仓储时间，也不改数据库全局时钟。
		waitTaskTime(t, e, before.Now.Add(30*time.Millisecond))
		after, err := repo.Load(e.ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if !after.Now.After(before.Now) {
			t.Fatal("未观察到数据库时钟推进")
		}
		next := prepare(t, 1)
		if next.MaterialsDigest() != originalMaterials || next.ContentDigest() != originalContent || next.Scope() != originalScope {
			t.Fatal("仅时钟推进改变资料包身份")
		}
		recheck(t, original, true)
	})
	for _, tc := range []struct{ name, change, restore string }{
		{"实例标记失效后恢复", "UPDATE tm_ai_local_instance SET contains_real_users=1", "UPDATE tm_ai_local_instance SET contains_real_users=0"},
		{"会员禁用后恢复", "UPDATE addon_member SET status=0 WHERE id=1", "UPDATE addon_member SET status=1 WHERE id=1"},
		{"来源禁用后恢复", "UPDATE tm_ai_sources SET enabled=0", "UPDATE tm_ai_sources SET enabled=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e.mustExec(tc.change)
			defer e.mustExec(tc.restore)
			recheck(t, original, false)
			if p, err := svc.Prepare(e.ctx, 1, request.SubjectType, request.SubjectID); p != nil || !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("撤权后仍可创建新包")
			}
			e.mustExec(tc.restore)
			recheck(t, original, true)
		})
	}
	for _, mode := range []string{"批准撤回", "批准改写"} {
		t.Run(mode, func(t *testing.T) {
			defer writeApproval(approval)
			if mode == "批准撤回" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				changed := approval
				changed.Revision = "v2"
				writeApproval(changed)
			}
			recheck(t, original, false)
			if p, err := svc.Prepare(e.ctx, 1, request.SubjectType, request.SubjectID); p != nil || !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("批准变更后仍返回包")
			}
			writeApproval(approval)
			recheck(t, original, true)
		})
	}
	t.Run("题材改名拒绝原包", func(t *testing.T) {
		e.mustExec("UPDATE addon_quant_theme SET name='仅测试：改名资料题材' WHERE id=7")
		defer e.mustExec("UPDATE addon_quant_theme SET name='仅测试：合成资料题材' WHERE id=7")
		changed := prepare(t, 1)
		if changed.Scope().SubjectName != "仅测试:改名资料题材" || changed.MaterialsDigest() != originalMaterials || changed.ContentDigest() == originalContent {
			t.Fatal("题材改名未仅改变完整内容身份")
		}
		recheck(t, original, false)
		recheck(t, changed, true)
		e.mustExec("UPDATE addon_quant_theme SET name='仅测试：合成资料题材' WHERE id=7")
		recheck(t, original, true)
	})
	t.Run("两会员包隔离", func(t *testing.T) {
		second := prepare(t, 2)
		if second.Scope().MemberID != 2 || original.Scope().MemberID != 1 || second.ContentDigest() == originalContent || second.MaterialsDigest() != originalMaterials {
			t.Fatal("会员完整摘要未隔离或相同资料声明漂移")
		}
		recheck(t, original, true)
		recheck(t, second, true)
		for _, member := range []int64{1, 2} {
			e.mustExec("UPDATE addon_member SET status=0 WHERE id=?", member)
			// 每个会员仅影响自身原包；Recheck 不是当前调用者鉴权接口。
			recheck(t, original, member != 1)
			recheck(t, second, member != 2)
			e.mustExec("UPDATE addon_member SET status=1 WHERE id=?", member)
			recheck(t, original, true)
			recheck(t, second, true)
		}
	})
	assertOriginal(t)
}
