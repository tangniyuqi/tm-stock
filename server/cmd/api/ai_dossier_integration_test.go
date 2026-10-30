//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestIntegrationAIReportDossierStartup(t *testing.T) {
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("需要隔离MySQL")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("测试连接无效")
	}
	cfg.DBName = ""
	cfg.MultiStatements = true
	cfg.ParseTime = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("管理连接失败")
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(random[:])
	name := "tm_ai_local_" + id
	exec := func(db *sql.DB, q string, args ...any) {
		t.Helper()
		if _, e := db.ExecContext(ctx, q, args...); e != nil {
			t.Fatal("隔离夹具操作失败", e)
		}
	}
	exec(admin, "CREATE DATABASE "+name+" CHARACTER SET utf8mb4 COLLATE utf8mb4_bin")
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, e := admin.ExecContext(c, "DROP DATABASE "+name); e != nil {
			t.Error("清理隔离库失败")
		}
	}()
	cfg.DBName = name
	prepDSN := cfg.FormatDSN()
	db, err := sql.Open("mysql", prepDSN)
	if err != nil {
		t.Fatal("测试库连接失败")
	}
	defer db.Close()
	for _, file := range []string{"20261010_create_ai_source_snapshots.sql", "20261030_create_tm_ai_dossiers.sql"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "migrations", file))
		if e != nil {
			t.Fatal(e)
		}
		exec(db, string(b))
	}
	exec(db, `CREATE TABLE tm_ai_local_instance(instance_id char(32) PRIMARY KEY,database_name varchar(64),server_uuid char(36),environment varchar(32),contains_real_users tinyint)`)
	exec(db, `INSERT INTO tm_ai_local_instance VALUES(?,?,@@server_uuid,'local-isolated',0)`, id, name)
	exec(db, `CREATE TABLE addon_member(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL,deleted_at datetime(3));INSERT INTO addon_member VALUES(1,1,NULL),(2,1,NULL);CREATE TABLE addon_quant_theme(id bigint unsigned PRIMARY KEY,status tinyint NOT NULL);INSERT INTO addon_quant_theme VALUES(7,1)`)
	sourceRepo := repository.NewAISourceRepo(db)
	src, err := sourceRepo.RegisterSource(ctx, sources.Source{Domain: "example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试", LicenseRef: "test-fixture:private-license", MinInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err = sourceRepo.SetSourceEnabled(ctx, src.ID, true); err != nil {
		t.Fatal(err)
	}
	src, err = sourceRepo.SourceByID(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	signing := hex.EncodeToString(random[:]) + hex.EncodeToString(random[:])
	read := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: signing, Issuer: "GVA", Audience: "GVA"}}
	verifier, err := memberauth.NewVerifier(read.Auth)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(db))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewAIDossierService(repository.NewAIDossierRepo(db, sql.LevelReadCommitted), sessions, true)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	var texts []string
	var drafts []dossier.Draft
	for v, body := range []string{"仅测试：测试主体甲处于送样阶段。", "仅测试：测试主体甲处于验证阶段。"} {
		snap, e := sources.BuildSnapshot(src, "https://example.test/test-only", sources.MediaPlain, []byte(body), base.Add(time.Duration(v)*time.Second), 4096)
		if e != nil {
			t.Fatal(e)
		}
		snap, e = sourceRepo.SaveSnapshot(ctx, snap)
		if e != nil {
			t.Fatal(e)
		}
		sentence := snap.Sentences[0]
		texts = append(texts, snap.Body)
		ref := dossier.SentenceRef{SnapshotID: snap.ID, Hash: snap.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}
		u := dossier.Field{State: "unknown"}
		subject := ref
		subject.StartByte = strings.Index(snap.Body, "测试主体甲")
		subject.EndByte = subject.StartByte + len("测试主体甲")
		d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 7, Version: int64(v + 1), CreatedAt: base.Add(time.Duration(v+3) * time.Second).UnixMilli(), Cards: []dossier.FactCard{{Key: "progress_a", Section: "progress", Statement: ref, Subject: dossier.Field{State: "known", Ref: subject}, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}, Gaps: []dossier.Gap{{Key: "time_gap", Section: "progress", Reason: "time_unverified"}}}
		drafts = append(drafts, d)
		if _, e = svc.StageInternal(ctx, d); e != nil {
			t.Fatal(e)
		}
	}
	exec(db, `INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:private-publication',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions;INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:private-grant',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions`)
	user := "dossier_api_" + id[:12]
	account := "'" + user + "'@'127.0.0.1'"
	exec(admin, "CREATE USER "+account+" IDENTIFIED BY '"+signing+"'")
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, e := admin.ExecContext(c, "DROP USER "+account); e != nil {
			t.Error("清理读取账号失败")
		}
	}()
	for _, table := range []string{"tm_ai_local_instance", "addon_member", "addon_quant_theme", "tm_ai_sources", "tm_ai_source_snapshots", "tm_ai_source_bodies", "tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants"} {
		exec(admin, "GRANT SELECT ON "+name+"."+table+" TO "+account)
	}
	cfg.User, cfg.Passwd = user, signing
	readerDSN := cfg.FormatDSN()
	reader, err := sql.Open("mysql", readerDSN)
	if err != nil {
		t.Fatal("读取连接失败")
	}
	defer reader.Close()
	runtime := &config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: id, Database: name, Listen: "127.0.0.1:55555", PollIntervalMS: 1000, BatchSize: 1}
	t.Setenv("TM_AI_LOCAL_DOSSIER_READ_ENABLED", "true")
	t.Setenv("TM_AI_LOCAL_DOSSIER_PREPARE_DSN", prepDSN)
	mux := http.NewServeMux()
	t.Run("受限账号启动", func(t *testing.T) {
		if e := registerAILocalDossiers(ctx, mux, reader, read, runtime, readerDSN); e != nil {
			t.Fatal(e)
		}
	})
	token := func(uid int64, kind string) string {
		t.Helper()
		s, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"ID": uid, "UserType": kind, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte(signing))
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	request := func(token, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	path := "/api/v1/ai/themes/7/dossiers/1?compareVersion=2"
	t.Run("本人双版原文与字段", func(t *testing.T) {
		w := request(token(1, "client"), path)
		var envelope struct {
			Code int               `json:"code"`
			Data dto.AIDossierView `json:"data"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &envelope) != nil || len(envelope.Data.Versions) != 2 {
			t.Fatal("双版响应无效", w.Code)
		}
		c := envelope.Data.Versions[0].Cards[0]
		if envelope.Data.Comparison == nil || len(envelope.Data.Comparison.Changed) != 1 || !envelope.Data.Comparison.Changed[0].QuoteChanged {
			t.Fatal("双版摘录变化未接入")
		}
		if c.Fields[0].Quote == nil || c.Fields[0].Quote.Text != "测试主体甲" || c.Fields[1].State != "unknown" || c.Fields[1].Quote != nil || c.Quote.SourceURL != "https://example.test/test-only" || c.Quote.Text != texts[0] {
			t.Fatal("字段或出处失真")
		}
		for _, private := range []string{"private-license", "private-publication", "private-grant", "canonical", "digest"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatal("泄露内部记录")
			}
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("响应可缓存")
		}
	})
	listPath := "/api/v1/ai/themes/7/dossiers"
	readList := func(uid int64) dto.AIDossierVersionList {
		t.Helper()
		w := request(token(uid, "client"), listPath)
		var envelope struct {
			Data dto.AIDossierVersionList `json:"data"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &envelope) != nil {
			t.Fatal("列表读取失败", w.Code)
		}
		return envelope.Data
	}
	t.Run("当前会员版本列表", func(t *testing.T) {
		list := readList(1)
		if len(list.Items) != 2 || list.Items[0].Version != 2 || list.Items[1].Version != 1 || list.HasMore {
			t.Fatal("本人版本列表错误")
		}
		if other := readList(2); len(other.Items) != 0 || other.HasMore || other.NextBeforeVersion != 0 {
			t.Fatal("跨会员列表泄露")
		}
	})
	for _, tc := range []struct {
		name   string
		uid    int64
		kind   string
		status int
	}{{"跨会员", 2, "client", 404}, {"后台令牌", 1, "admin", 401}, {"匿名", 0, "", 401}} {
		t.Run(tc.name, func(t *testing.T) {
			bearer := ""
			if tc.uid != 0 {
				bearer = token(tc.uid, tc.kind)
			}
			w := request(bearer, path)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "versions") {
				t.Fatal("身份隔离失败", w.Code)
			}
		})
	}
	t.Run("旧版撤权整组隐藏", func(t *testing.T) {
		exec(db, `UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=1`)
		w := request(token(1, "client"), path)
		if w.Code != 404 || strings.Contains(w.Body.String(), "versions") {
			t.Fatal("撤权泄露双版")
		}
		if list := readList(1); len(list.Items) != 1 || list.Items[0].Version != 2 {
			t.Fatal("撤权版本未从目录隐藏")
		}
		exec(db, `UPDATE tm_ai_dossier_publications SET enabled=1 WHERE version_no=1`)
	})
	t.Run("来源撤回不返回旧正文", func(t *testing.T) {
		if e := sourceRepo.SetSourceEnabled(ctx, src.ID, false); e != nil {
			t.Fatal(e)
		}
		w := request(token(1, "client"), path)
		if w.Code == 200 || strings.Contains(w.Body.String(), "送样") {
			t.Fatal("来源撤回泄露")
		}
		if list := readList(1); len(list.Items) != 0 || list.HasMore || list.NextBeforeVersion != 0 {
			t.Fatal("来源撤回后仍泄露版本元数据")
		}
		if e := sourceRepo.SetSourceEnabled(ctx, src.ID, true); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("旧快照回用与同文编制升级", func(t *testing.T) {
		for _, v := range []int64{3, 4} {
			d := drafts[0]
			d.Version = v
			d.CreatedAt = base.Add(time.Duration(v+3) * time.Second).UnixMilli()
			if _, err := svc.StageInternal(ctx, d); err != nil {
				t.Fatal(err)
			}
			exec(db, `INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:reused',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no=?`, v)
			exec(db, `INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:reused',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions WHERE version_no=?`, v)
		}
		for _, c := range []struct {
			path    string
			changed int
		}{{"/api/v1/ai/themes/7/dossiers/2?compareVersion=3", 1}, {"/api/v1/ai/themes/7/dossiers/1?compareVersion=4", 0}} {
			w := request(token(1, "client"), c.path)
			var envelope struct {
				Data dto.AIDossierView `json:"data"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &envelope) != nil || envelope.Data.Comparison == nil || len(envelope.Data.Comparison.Changed) != c.changed {
				t.Fatal("旧快照回用或同文升级对照错误", w.Code)
			}
		}
	})
	for _, mode := range []string{"准备复用读取账号", "准备连接不同端口", "禁止真实调用配置", "隔离标记失效", "缺少正文读取权限"} {
		t.Run(mode, func(t *testing.T) {
			current := *runtime
			activeReader := reader
			switch mode {
			case "准备复用读取账号":
				t.Setenv("TM_AI_LOCAL_DOSSIER_PREPARE_DSN", readerDSN)
			case "准备连接不同端口":
				bad, e := mysql.ParseDSN(prepDSN)
				if e != nil {
					t.Fatal("解析测试配置失败")
				}
				bad.Addr = "127.0.0.1:65534"
				if cfg.Addr == bad.Addr {
					bad.Addr = "127.0.0.1:65533"
				}
				t.Setenv("TM_AI_LOCAL_DOSSIER_PREPARE_DSN", bad.FormatDSN())
			case "禁止真实调用配置":
				current.RealCalls = true
			case "隔离标记失效":
				exec(db, `UPDATE tm_ai_local_instance SET contains_real_users=1`)
				defer exec(db, `UPDATE tm_ai_local_instance SET contains_real_users=0`)
			case "缺少正文读取权限":
				exec(admin, "REVOKE SELECT ON "+name+".tm_ai_source_bodies FROM "+account)
				defer exec(admin, "GRANT SELECT ON "+name+".tm_ai_source_bodies TO "+account)
				var e error
				activeReader, e = sql.Open("mysql", readerDSN)
				if e != nil {
					t.Fatal("新读取连接失败")
				}
				defer activeReader.Close()
			}
			fresh := http.NewServeMux()
			if e := registerAILocalDossiers(ctx, fresh, activeReader, read, &current, readerDSN); e == nil {
				t.Fatal("启动边界未拒绝")
			}
			w := httptest.NewRecorder()
			fresh.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			if w.Code != 404 {
				t.Fatal("启动失败仍挂路由")
			}
		})
	}
	t.Run("损坏结构不注册接口", func(t *testing.T) {
		exec(db, `DROP TRIGGER tm_ai_dossier_versions_no_update`)
		fresh := http.NewServeMux()
		if e := registerAILocalDossiers(ctx, fresh, reader, read, runtime, readerDSN); e == nil {
			t.Fatal("结构损坏仍启动")
		}
		w := httptest.NewRecorder()
		fresh.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("失败仍挂路由")
		}
	})
}
