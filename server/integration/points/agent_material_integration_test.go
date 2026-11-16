//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 复用隔离随机库和真实迁移，不创建任务、不发积分，也不装配模型或报告成功替身。
func agentMaterialFixture(t *testing.T) (*pointsTestEnv, *sql.DB, repository.AgentMaterialRequest, sources.Snapshot) {
	t.Helper()
	e, db, _, _, _, now := persistentTaskFixture(t, sql.LevelRepeatableRead)
	reportDDL(t, e, "20261010_create_ai_source_snapshots.sql")
	e.mustExec("CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL)")
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
	return e, db, request, snap
}

func agentMaterialAssertNoWrites(t *testing.T, e *pointsTestEnv) {
	t.Helper()
	for i, count := range taskProbeCounts(e) {
		if count != 0 {
			t.Fatalf("资料读取留下任务或积分行：%s=%d", taskProbeTables[i], count)
		}
	}
}

func agentMaterialAssertLoaded(t *testing.T, e *pointsTestEnv, db *sql.DB, request repository.AgentMaterialRequest, snap sources.Snapshot) {
	t.Helper()
	got, err := repository.NewAIAgentMaterialRepo(db).Load(e.ctx, request)
	if err != nil {
		t.Fatal("合法资料读取失败", err)
	}
	if got.SubjectName != "仅测试:合成资料题材" || got.Now.IsZero() || !got.Now.Before(request.ValidUntil) || len(got.Sources) != 1 || len(got.Snapshots) != 1 {
		t.Fatal("读取未返回完整当前对象、来源和快照")
	}
	current := got.Snapshots[0]
	if current.ID != snap.ID || current.Hash != snap.Hash || current.Body != snap.Body || !reflect.DeepEqual(current.Sentences, snap.Sentences) || got.Sources[0].ID != snap.SourceID || !got.Sources[0].HasValidAuthorization(got.Now) {
		t.Fatal("读取正文、句子索引或当前许可与真实入库资料不符")
	}
	agentMaterialAssertNoWrites(t, e)
}

func TestIntegrationAIReportAgentMaterialLoad(t *testing.T) {
	e, db, request, snap := agentMaterialFixture(t)
	agentMaterialAssertLoaded(t, e, db, request, snap)
	for _, tc := range []struct{ name, change, restore string }{
		{"会员禁用", "UPDATE addon_member SET status=0", "UPDATE addon_member SET status=1"},
		{"会员删除", "UPDATE addon_member SET deleted_at=UTC_TIMESTAMP(3)", "UPDATE addon_member SET deleted_at=NULL"},
		{"对象禁用", "UPDATE addon_quant_theme SET status=0", "UPDATE addon_quant_theme SET status=1"},
		{"对象删除", "UPDATE addon_quant_theme SET deleted_at=UTC_TIMESTAMP(3)", "UPDATE addon_quant_theme SET deleted_at=NULL"},
		{"来源撤权", "UPDATE tm_ai_sources SET enabled=0", "UPDATE tm_ai_sources SET enabled=1"},
		{"许可引用变更", "UPDATE tm_ai_sources SET license_ref='test-fixture:changed'", "UPDATE tm_ai_sources SET license_ref='test-fixture:agent-material'"},
		{"许可标签变更", "UPDATE tm_ai_sources SET license_tag='仅测试变更'", "UPDATE tm_ai_sources SET license_tag='仅测试'"},
		{"来源类别变更", "UPDATE tm_ai_sources SET category='annual_report'", "UPDATE tm_ai_sources SET category='official_catalog'"},
		{"许可过期", "UPDATE tm_ai_sources SET license_expires_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND", "UPDATE tm_ai_sources s JOIN tm_ai_source_snapshots p ON p.source_id=s.id SET s.license_expires_at=p.license_expires_snapshot"},
		{"许可期限变更", "UPDATE tm_ai_sources SET license_expires_at=license_expires_at+INTERVAL 1 HOUR", "UPDATE tm_ai_sources s JOIN tm_ai_source_snapshots p ON p.source_id=s.id SET s.license_expires_at=p.license_expires_snapshot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agentMaterialAssertLoaded(t, e, db, request, snap)
			e.mustExec(tc.change)
			got, err := repository.NewAIAgentMaterialRepo(db).Load(e.ctx, request)
			e.mustExec(tc.restore)
			if err == nil || len(got.Snapshots) != 0 {
				t.Fatal("当前权限变化后仍返回资料")
			}
			agentMaterialAssertNoWrites(t, e)
			agentMaterialAssertLoaded(t, e, db, request, snap)
		})
	}
	for _, name := range []string{"批准过期", "哈希错配", "专题拒绝", "正文超限", "许可未覆盖批准期限"} {
		t.Run(name, func(t *testing.T) {
			bad := request
			bad.Evidence = append([]repository.AgentMaterialEvidence(nil), request.Evidence...)
			switch name {
			case "批准过期":
				bad.ValidUntil = snap.FetchedAt
			case "哈希错配":
				bad.Evidence[0].Hash = strings.Repeat("0", 64)
			case "专题拒绝":
				bad.SubjectType = "topic"
			case "正文超限":
				bad.MaxEvidenceBytes = len(snap.Body) - 1
			case "许可未覆盖批准期限":
				bad.ValidUntil = snap.LicenseExpires.Add(time.Minute)
			}
			got, err := repository.NewAIAgentMaterialRepo(db).Load(e.ctx, bad)
			if err == nil || len(got.Snapshots) != 0 {
				t.Fatal("非法请求仍返回资料")
			}
			if name == "专题拒绝" && !errors.Is(err, agents.ErrInvalid) {
				t.Fatal("专题未在请求边界拒绝")
			}
			agentMaterialAssertNoWrites(t, e)
			agentMaterialAssertLoaded(t, e, db, request, snap)
		})
	}
}

// 既有专题账号 helper 会调用专题服务和专题表，不能用于此独立仓储；沿用随机账号和逐表授权方式。
func TestIntegrationAIReportAgentMaterialReadOnlyAccess(t *testing.T) {
	e, owner, request, snap := agentMaterialFixture(t)
	agentMaterialAssertLoaded(t, e, owner, request, snap)
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	user, password := "material_"+hex.EncodeToString(seed[:8]), hex.EncodeToString(seed[:])
	account := "'" + user + "'@'%'"
	// 账号和密码只来自随机十六进制，不接受外部 SQL 标识符，也不输出凭据。
	if _, err := e.conn.ExecContext(e.ctx, "CREATE USER "+account+" IDENTIFIED BY '"+password+"'"); err != nil {
		t.Fatal("创建资料只读账号失败")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := e.conn.ExecContext(ctx, "DROP USER "+account); err != nil {
			t.Error("清理资料只读账号失败")
		}
	})
	grantBody := "GRANT SELECT ON " + e.database + ".tm_ai_source_bodies TO " + account
	for _, table := range []string{"addon_member", "addon_quant_theme", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies"} {
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO " + account)
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("测试DSN解析失败")
	}
	cfg.DBName, cfg.User, cfg.Passwd = e.database, user, password
	open := func() *sql.DB {
		t.Helper()
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal("资料受限连接装配失败")
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error("关闭资料受限连接失败")
			}
		})
		return db
	}
	reader := open()
	agentMaterialAssertLoaded(t, e, reader, request, snap)
	for _, query := range []string{
		"CREATE TABLE material_forbidden(id int)",
		"UPDATE tm_ai_source_bodies SET body_text=body_text WHERE 1=0",
		"INSERT INTO tm_ai_source_snapshots SELECT * FROM tm_ai_source_snapshots WHERE 1=0",
		"SELECT * FROM tm_ai_tasks LIMIT 0", "SELECT * FROM tm_points_accounts LIMIT 0",
	} {
		_, err := reader.ExecContext(e.ctx, query)
		agentMaterialAssertPermissionDenied(t, err)
	}
	e.mustExec("REVOKE SELECT ON " + e.database + ".tm_ai_source_bodies FROM " + account)
	// 每次重新连接，防止 MySQL 会话权限缓存使撤权/恢复得到假阳性。
	denied := open()
	got, err := repository.NewAIAgentMaterialRepo(denied).Load(e.ctx, request)
	agentMaterialAssertPermissionDenied(t, err)
	if len(got.Snapshots) != 0 {
		t.Fatal("缺正文SELECT权限仍返回资料")
	}
	agentMaterialAssertNoWrites(t, e)
	e.mustExec(grantBody)
	restored := open()
	agentMaterialAssertLoaded(t, e, restored, request, snap)
}

func agentMaterialAssertPermissionDenied(t *testing.T, err error) {
	t.Helper()
	var denied *mysql.MySQLError
	if !errors.As(err, &denied) || (denied.Number != 1044 && denied.Number != 1142 && denied.Number != 1143) {
		t.Fatal("负例不是明确的MySQL权限拒绝")
	}
}
