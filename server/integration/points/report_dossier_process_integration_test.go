//go:build integration

package points_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type dossierProcessFixture struct {
	f                           *localPublicationFixture
	login                       *h5MemberLoginBridge
	client                      *http.Client
	base                        string
	apiUser                     string
	baselineTasks, baselineTxns int
}

func newDossierProcessFixture(t *testing.T, lifetime time.Duration, references ...bool) *dossierProcessFixture {
	t.Helper()
	if len(references) > 1 {
		t.Fatal("引用夹具配置重复")
	}
	saved := len(references) == 1 && references[0]
	return newDossierProcessWithQA(t, lifetime, saved, nil)
}

func newDossierProcessWithQA(t *testing.T, lifetime time.Duration, saved bool, prepareQA func(*localPublicationFixture, string) string, corrections ...bool) *dossierProcessFixture {
	t.Helper()
	if len(corrections) > 1 {
		t.Fatal("更正读取夹具配置重复")
	}
	correctionsEnabled := len(corrections) == 1 && corrections[0]
	api, _, login := buildLocalReportBinaries(t)
	f := newLocalPublicationFixture(t, sql.LevelReadCommitted, 1, "local")
	e := f.env
	ctx, cancel := context.WithTimeout(context.Background(), lifetime)
	t.Cleanup(cancel)
	e.pointsTestEnv.ctx = ctx
	localAPISchema(t, e)
	e.mustExec(`ALTER TABLE addon_quant_theme ADD code varchar(32) NULL, ADD level tinyint NOT NULL DEFAULT 1, ADD parent_id bigint NOT NULL DEFAULT 0, ADD description text, ADD sort int NOT NULL DEFAULT 0, ADD updated_at datetime(3) NULL, ADD created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)`)
	reportDDL(t, e.pointsTestEnv, "20261030_create_tm_ai_dossiers.sql")
	if correctionsEnabled {
		reportDDL(t, e.pointsTestEnv, "20261103_create_tm_ai_dossier_management.sql")
		reportDDL(t, e.pointsTestEnv, "20261103_create_tm_ai_dossier_corrections.sql")
		if err := repository.NewAIDossierCorrectionSchemaVerifier(e.db).Verify(e.ctx); err != nil {
			t.Fatal("更正结构及实际探针失败", err)
		}
	}
	if saved {
		reportDDL(t, e.pointsTestEnv, "20261030_link_tm_ai_conversation_dossiers.sql")
	}
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	signing := hex.EncodeToString(seed[:])
	bridge := startIsolatedMemberLogin(t, ctx, e.database, signing, login)
	sr := repository.NewAISourceRepo(e.db)
	source, err := sr.RegisterSource(ctx, sources.Source{Domain: "dossier.example.test", Category: sources.CategoryOfficialList, LicenseTag: "仅测试：合成档案", LicenseRef: "test-fixture:dossier-process", MinInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err = sr.SetSourceEnabled(ctx, source.ID, true); err != nil {
		t.Fatal(err)
	}
	source, err = sr.SourceByID(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewAIDossierService(repository.NewAIDossierRepo(e.db, sql.LevelReadCommitted), dossierTestAuth{}, true)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	for i, body := range []string{"仅测试：样本主体甲的样品处于送样阶段。", "仅测试：样本主体甲的样品处于验证阶段。"} {
		if correctionsEnabled && i == 1 {
			body = "仅测试：更正声明将样本主体甲的样品阶段由送样更正为验证。"
		}
		snap, er := sources.BuildSnapshot(source, "https://dossier.example.test/synthetic-only", sources.MediaPlain, []byte(body), at.Add(time.Duration(i)*time.Second), 4096)
		if er != nil {
			t.Fatal(er)
		}
		snap, er = sr.SaveSnapshot(ctx, snap)
		if er != nil {
			t.Fatal(er)
		}
		sentence := snap.Sentences[0]
		ref := dossier.SentenceRef{SnapshotID: snap.ID, Hash: snap.Hash, Number: sentence.Number, StartByte: sentence.StartByte, EndByte: sentence.EndByte}
		field := ref
		field.StartByte = strings.Index(snap.Body, "样本主体甲")
		field.EndByte = field.StartByte + len("样本主体甲")
		unknown := dossier.Field{State: "unknown"}
		d := dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 1, Version: int64(i + 1), CreatedAt: at.Add(time.Duration(i+3) * time.Second).UnixMilli(), Cards: []dossier.FactCard{{Key: "progress_a", Section: "progress", Statement: ref, Subject: dossier.Field{State: "known", Ref: field}, PublishedDate: unknown, EffectiveDate: unknown, Period: unknown, Scope: unknown, Stage: unknown, Basis: unknown, Unit: unknown}}, Gaps: []dossier.Gap{{Key: "time_gap", Section: "progress", Reason: "time_unverified"}}}
		if _, er = svc.StageInternal(ctx, d); er != nil {
			t.Fatal(er)
		}
	}
	e.mustExec(`INSERT INTO tm_ai_dossier_publications SELECT theme_id,version_no,digest,'test-fixture:dossier-h5-pub',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions;INSERT INTO tm_ai_dossier_grants SELECT 1,theme_id,version_no,digest,'test-fixture:dossier-h5-member',1,UTC_TIMESTAMP(3)+INTERVAL 1 HOUR,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM tm_ai_dossier_versions`)
	dsn := localAPIDSN(t, e)
	qaFile := ""
	if prepareQA != nil {
		qaFile = prepareQA(f, dsn)
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("读取配置无效")
	}
	for _, table := range []string{"tm_ai_dossier_versions", "tm_ai_dossier_evidence", "tm_ai_dossier_publications", "tm_ai_dossier_grants"} {
		e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO '" + cfg.User + "'@'127.0.0.1'")
	}
	if correctionsEnabled {
		if err := repository.NewAIDossierManagementSchemaVerifier(e.db).Verify(e.ctx); err != nil {
			t.Fatal("更正读取管理结构前置失败", err)
		}
		for _, table := range []string{"tm_ai_dossier_management_heads", "tm_ai_dossier_management_events", "tm_ai_dossier_correction_events"} {
			e.mustExec("GRANT SELECT ON " + e.database + "." + table + " TO '" + cfg.User + "'@'127.0.0.1'")
		}
		readDB, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatal("更正读取连接失败")
		}
		accessErr := repository.NewAIDossierCorrectionRepo(readDB).VerifyAccess(e.ctx, false)
		closeErr := readDB.Close()
		if accessErr != nil || closeErr != nil {
			t.Fatal("更正实际读取账号权限失败", accessErr)
		}
	}
	if saved {
		e.mustExec("GRANT SELECT,INSERT ON " + e.database + ".tm_ai_conversation_dossiers TO '" + cfg.User + "'@'127.0.0.1'")
	}
	prepare, err := mysql.ParseDSN(os.Getenv("TM_TEST_DSN"))
	if err != nil {
		t.Fatal("准备配置无效")
	}
	prepare.DBName = e.database
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.runtime.Listen = listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	runtimeFile := filepath.Join(t.TempDir(), "runtime.json")
	raw, err := json.Marshal(f.runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(runtimeFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	requestTimeout := 5 * time.Second
	if prepareQA != nil {
		requestTimeout = 28 * time.Second
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(client.CloseIdleConnections)
	base := "http://" + f.runtime.Listen
	startLocalAPIConfigured(t, e, api, dsn, signing, runtimeFile, f.path, client, base, false, qaFile, false, prepare.FormatDSN(), saved, correctionsEnabled)
	return &dossierProcessFixture{f: f, login: bridge, client: client, base: base, apiUser: cfg.User, baselineTasks: e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`), baselineTxns: e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)}
}
func (p *dossierProcessFixture) verifyNoGeneration(t *testing.T) {
	t.Helper()
	e := p.f.env
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`) != p.baselineTasks || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != p.baselineTxns || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_commits`) != 0 || len(e.server.Requests()) != 0 || e.submits.Load() != 0 {
		t.Fatal("档案查询触发了任务、账务或模型/Agent调用")
	}
}

func TestIntegrationAIReportDossierProcess(t *testing.T) {
	p := newDossierProcessFixture(t, 2*time.Minute)
	e := p.f.env
	tokens, _ := localPasswordTokens(t, e.ctx, p.login, p.client)
	call := func(token, path string) (int, []byte) {
		t.Helper()
		r, err := http.NewRequestWithContext(e.ctx, "GET", p.base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := p.client.Do(r)
		if err != nil {
			t.Fatal("原API请求失败")
		}
		defer res.Body.Close()
		body, err := io.ReadAll(io.LimitReader(res.Body, 1048577))
		if err != nil || len(body) > 1048576 {
			t.Fatal("原API响应无效")
		}
		if res.StatusCode != 200 && (strings.Contains(string(body), `"sourceRelations"`) || strings.Contains(string(body), `"groups"`)) {
			t.Fatal("拒绝响应泄漏来源关系或数量")
		}
		return res.StatusCode, body
	}
	t.Run("原登录与版本目录", func(t *testing.T) {
		status, body := call(tokens[0], "/api/v1/ai/themes/1/dossiers")
		var out struct {
			Data dto.AIDossierVersionList `json:"data"`
		}
		if status != 200 || json.Unmarshal(body, &out) != nil || len(out.Data.Items) != 2 {
			t.Fatal("正式进程版本列表失败", status)
		}
	})
	t.Run("单版与原文出处", func(t *testing.T) {
		status, body := call(tokens[0], "/api/v1/ai/themes/1/dossiers/1")
		var out struct {
			Data dto.AIDossierView `json:"data"`
		}
		if status != 200 || json.Unmarshal(body, &out) != nil || len(out.Data.Versions) != 1 || out.Data.Versions[0].Cards[0].Quote.SourceURL != "https://dossier.example.test/synthetic-only" {
			t.Fatal("正式进程原文读取失败", status)
		}
		if r := out.Data.SourceRelations; r.Groups == nil || len(r.Groups) != 0 || r.Independence != "unverified" || r.ReprintChain != "unverified" || r.ExplicitCorrection != "unverified" {
			t.Fatal("单份资料被虚构来源独立或更正结论")
		}
	})
	t.Run("双版摘录对照", func(t *testing.T) {
		status, body := call(tokens[0], "/api/v1/ai/themes/1/dossiers/1?compareVersion=2")
		var out struct {
			Data dto.AIDossierView `json:"data"`
		}
		if status != 200 || json.Unmarshal(body, &out) != nil || out.Data.Comparison == nil || len(out.Data.Comparison.Changed) != 1 || !out.Data.Comparison.Changed[0].QuoteChanged {
			t.Fatal("正式进程对照失败", status)
		}
		r := out.Data.SourceRelations
		if len(r.Groups) != 2 || r.Groups[0].Kind != "same_url" || r.Groups[1].Kind != "same_registered_source" || r.ExplicitCorrection != "unverified" {
			t.Fatal("不同正文误判相同或摘录变化误判更正")
		}
		for _, g := range r.Groups {
			if len(g.SnapshotIDs) != 2 || g.SnapshotIDs[0] != out.Data.Versions[0].Cards[0].Quote.SnapshotID || g.SnapshotIDs[1] != out.Data.Versions[1].Cards[0].Quote.SnapshotID {
				t.Fatal("来源组越出当前授权引文")
			}
		}
	})
	t.Run("跨会员与匿名拒绝", func(t *testing.T) {
		for _, token := range []string{tokens[1], ""} {
			status, body := call(token, "/api/v1/ai/themes/1/dossiers/1?compareVersion=2")
			if status != 404 && status != 401 || strings.Contains(string(body), "comparison") {
				t.Fatal("正式进程身份隔离失败", status)
			}
		}
	})
	t.Run("撤权后不返回旧原文", func(t *testing.T) {
		e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=0 WHERE version_no=1`)
		status, body := call(tokens[0], "/api/v1/ai/themes/1/dossiers/1?compareVersion=2")
		if status != 404 || strings.Contains(string(body), "样本主体甲") {
			t.Fatal("撤权后泄露原文")
		}
		e.mustExec(`UPDATE tm_ai_dossier_publications SET enabled=1 WHERE version_no=1`)
	})
	t.Run("来源与第二版读取许可撤回恢复", func(t *testing.T) {
		for _, statement := range []string{`UPDATE tm_ai_sources SET enabled=%d WHERE domain='dossier.example.test'`, `UPDATE tm_ai_dossier_grants SET enabled=%d WHERE version_no=2`} {
			e.mustExec(fmt.Sprintf(statement, 0))
			status, _ := call(tokens[0], "/api/v1/ai/themes/1/dossiers/1?compareVersion=2")
			if status != 404 {
				t.Fatal("撤回后来源关系仍可读取")
			}
			e.mustExec(fmt.Sprintf(statement, 1))
			status, body := call(tokens[0], "/api/v1/ai/themes/1/dossiers/1?compareVersion=2")
			var out struct {
				Data dto.AIDossierView `json:"data"`
			}
			if status != 200 || json.Unmarshal(body, &out) != nil || len(out.Data.SourceRelations.Groups) != 2 {
				t.Fatal("恢复后未重建当前来源关系")
			}
		}
	})
	p.verifyNoGeneration(t)
	p.login.finishAs(t, e.ctx, false)
}
