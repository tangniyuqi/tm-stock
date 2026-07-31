//go:build integration

// 本文件是【集成测试】：真连 MySQL，把 repository 的每一条 SQL 实际执行一遍。
//
// 为什么必须有它：
//
//	theme_sql_test.go 只能断言"SQL 字符串里含某些片段"，
//	它查不出【列名拼错、表名写错、类型不匹配、JOIN 条件写反】这类错误——
//	那些只有真执行才会暴露。而这些错误一旦上线，表现是接口 500 或数据错位。
//
// 默认不参与 `go test ./...`（构建标签 integration），
// 因为它需要一个可连的 MySQL。跑法：
//
//	bash scripts/dev/verify-repository.sh          # 自动起容器并跑
//	TM_TEST_DSN='user:pw@tcp(host:port)/db?...' go test -tags=integration ./internal/repository/
package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/model"
)

// ⚠️ 上游两表由现有系统维护（ADR-0007），本仓库不含其建表脚本。
// 这里是【测试专用】最小复刻，只保留本项目实际读取的列，不是 schema 权威来源。
const upstreamTestDDL = `
DROP TABLE IF EXISTS addon_quant_theme_stock;
DROP TABLE IF EXISTS addon_quant_theme;
DROP TABLE IF EXISTS addon_quant_base_stock;
CREATE TABLE addon_quant_theme (
  id bigint UNSIGNED NOT NULL AUTO_INCREMENT, name varchar(100) DEFAULT NULL,
  code varchar(100) DEFAULT NULL, level tinyint DEFAULT 0, parent_id bigint DEFAULT 0,
  description varchar(250) DEFAULT NULL, sort int DEFAULT 0, status tinyint DEFAULT 1,
  created_at datetime(3) DEFAULT NULL, updated_at datetime(3) DEFAULT NULL,
  deleted_at datetime(3) DEFAULT NULL, PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE addon_quant_base_stock (
  id bigint UNSIGNED NOT NULL AUTO_INCREMENT, ts_code varchar(20) DEFAULT NULL,
  symbol varchar(10) DEFAULT NULL, name varchar(50) DEFAULT NULL,
  industry varchar(50) DEFAULT NULL, cnspell varchar(50) DEFAULT NULL,
  market varchar(20) DEFAULT NULL, exchange varchar(10) DEFAULT NULL,
  list_status varchar(10) DEFAULT NULL,
  created_at datetime(3) DEFAULT NULL, updated_at datetime(3) DEFAULT NULL,
  deleted_at datetime(3) DEFAULT NULL, PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

// 固定装置。刻意覆盖【应被过滤掉】的各种情形，
// 光造合格数据的集成测试证明不了过滤条件真的在起作用。
const fixtureSQL = `
INSERT INTO addon_quant_theme(id,name,level,parent_id,description,sort,status,created_at,updated_at) VALUES
  (100001,'光刻机',1,0,'光刻机产业链',0,1,NOW(3),NOW(3)),
  (100010,'光源',2,100001,'光源环节',1,1,NOW(3),NOW(3)),
  (100011,'物镜',2,100001,'物镜环节',2,1,NOW(3),NOW(3)),
  (100012,'已删环节',2,100001,'',3,1,NOW(3),NOW(3)),
  (100002,'MLCC',1,0,'',0,1,NOW(3),NOW(3));
UPDATE addon_quant_theme SET deleted_at=NOW(3) WHERE id=100012;

INSERT INTO addon_quant_base_stock(id,ts_code,symbol,name,industry,cnspell,market,exchange,list_status,created_at,updated_at) VALUES
  (1,'688502.SH','688502','茂莱光学','光学元件','MLGX','科创板','SSE','L',NOW(3),NOW(3)),
  (2,'300346.SZ','300346','南大光电','半导体材料','NDGD','创业板','SZSE','L',NOW(3),NOW(3)),
  (3,'000001.SZ','000001','已删股票','银行','YSGP','主板','SZSE','L',NOW(3),NOW(3));
UPDATE addon_quant_base_stock SET deleted_at=NOW(3) WHERE id=3;

-- 应可见：证据齐全 + 已审核 + 启用
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,created_at)
 VALUES (100010,1,'688502.SH',2,'光源模组业务收入占比34.2%','https://e.com/1',NOW(3),2,1,0,NOW(3));
-- 应可见：同一股票挂到另一题材（多对多）
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,created_at)
 VALUES (100002,1,'688502.SH',1,'公司公告涉及MLCC材料','https://e.com/2',NOW(3),2,1,0,NOW(3));
-- 不可见：未审核（草稿）
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,created_at)
 VALUES (100011,2,'300346.SZ',2,'物镜相关业务','https://e.com/3',NOW(3),0,1,0,NOW(3));
-- 不可见：已停用
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,created_at)
 VALUES (100011,1,'688502.SH',2,'停用的归属','https://e.com/4',NOW(3),2,0,0,NOW(3));
-- 不可见：软删除
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,deleted_at,created_at)
 VALUES (100001,2,'300346.SZ',2,'已删的归属','https://e.com/5',NOW(3),2,1,0,NOW(3),NOW(3));
-- 不可见：关联到【已软删的股票】——考验 JOIN 是否过滤了上游软删除
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,created_at)
 VALUES (100010,3,'000001.SZ',2,'挂在已删股票上','https://e.com/6',NOW(3),2,1,0,NOW(3));
-- 不可见：关联到【已软删的环节】——考验 JOIN 是否过滤了上游软删除
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,audit_status,status,sort,created_at)
 VALUES (100012,2,'300346.SZ',2,'挂在已删环节上','https://e.com/7',NOW(3),2,1,0,NOW(3));`

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 TM_TEST_DSN，跳过集成测试（用 scripts/dev/verify-repository.sh 跑）")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("打开连接失败: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("连不上数据库: %v", err)
	}

	// 建上游测试表
	if _, err := db.Exec(upstreamTestDDL); err != nil {
		t.Fatalf("建上游测试表失败（DSN 是否带 multiStatements=true？）: %v", err)
	}
	// 跑本项目的正式迁移 —— 测的就是它，不能用另写一份 DDL 代替
	mig, err := os.ReadFile(filepath.Join("..", "..", "migrations", "20260730_addon_quant_theme_stock.sql"))
	if err != nil {
		t.Fatalf("读迁移脚本失败: %v", err)
	}
	if _, err := db.Exec(string(mig)); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}
	if _, err := db.Exec(fixtureSQL); err != nil {
		t.Fatalf("灌固定装置失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestIntegration_GetTheme(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	ctx := context.Background()

	got, err := repo.GetTheme(ctx, 100001)
	if err != nil {
		t.Fatalf("GetTheme 报错: %v", err)
	}
	if got == nil {
		t.Fatal("应查到题材 100001")
	}
	if got.Name != "光刻机" || got.Level != 1 || got.ParentID != 0 {
		t.Errorf("字段错位或取值错误: %+v", got)
	}
	if got.Description != "光刻机产业链" {
		t.Errorf("Description 应为「光刻机产业链」，得到 %q", got.Description)
	}

	// 已软删的题材不得查到
	if n, _ := repo.GetTheme(ctx, 100012); n != nil {
		t.Errorf("已软删的题材不应查到: %+v", n)
	}
	// 不存在返回 (nil, nil)
	if n, err := repo.GetTheme(ctx, 999999); n != nil || err != nil {
		t.Errorf("不存在应返回 (nil,nil)，得到 (%v,%v)", n, err)
	}
}

func TestIntegration_ListChainNodes(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	nodes, err := repo.ListChainNodes(context.Background(), 100001)
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	// 100010 光源、100011 物镜；100012 已软删不应出现
	if len(nodes) != 2 {
		t.Fatalf("应有 2 个环节（已软删的不算），实际 %d: %+v", len(nodes), nodes)
	}
	if nodes[0].Name != "光源" || nodes[1].Name != "物镜" {
		t.Errorf("应按 sort 排序为 光源/物镜，得到 %q/%q", nodes[0].Name, nodes[1].Name)
	}
}

// 本项目最关键的一条 SQL：四重过滤 + 上游软删除过滤，全部真执行一遍。
func TestIntegration_ListVisibleMappings_FiltersEverything(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	// 题材自身 + 全部环节（含已软删的 100012，考验 JOIN 过滤）
	got, err := repo.ListVisibleMappings(context.Background(),
		[]int64{100001, 100010, 100011, 100012})
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("7 条映射里只有 1 条该可见，实际 %d 条:\n%+v", len(got), got)
	}
	m := got[0]
	if m.ThemeID != 100010 || m.TsCode != "688502.SH" {
		t.Errorf("可见的应是 100010/688502.SH，得到 %d/%s", m.ThemeID, m.TsCode)
	}
	// 列错位会在这里暴露：若 Scan 顺序与列清单不一致，这些值会串
	if m.SourceType != model.SourceTypeAnnualReport {
		t.Errorf("SourceType 应为 2（年报），得到 %d —— 疑似列错位", m.SourceType)
	}
	if m.SourceExcerpt != "光源模组业务收入占比34.2%" {
		t.Errorf("SourceExcerpt 错误（疑似列错位）: %q", m.SourceExcerpt)
	}
	if m.AuditStatus != model.AuditStatusApproved || m.Status != model.StatusEnabled {
		t.Errorf("状态字段错位: audit=%d status=%d", m.AuditStatus, m.Status)
	}
	if m.CollectedAt.IsZero() {
		t.Error("CollectedAt 为零值 —— DSN 可能缺 parseTime=true")
	}
}

// 一只股票可同时属于多个题材（董事长明确的核心诉求）。
func TestIntegration_OneStockMultipleThemes(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	got, err := repo.ListVisibleMappings(context.Background(), []int64{100001, 100010, 100002})
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	themes := map[int64]bool{}
	for _, m := range got {
		if m.TsCode == "688502.SH" {
			themes[m.ThemeID] = true
		}
	}
	if len(themes) != 2 {
		t.Errorf("688502.SH 应同时属于 100010 与 100002 两个节点，实际 %v", themes)
	}
}

func TestIntegration_FindVisibleMapping(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	ctx := context.Background()

	// 传题材 id，应能命中挂在其子环节上的映射
	got, err := repo.FindVisibleMapping(ctx, 100001, "688502.SH")
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	if got == nil {
		t.Fatal("按题材 id 应能命中挂在子环节上的映射")
	}
	if got.SourceURL != "https://e.com/1" {
		t.Errorf("SourceURL 错误（疑似列错位）: %q", got.SourceURL)
	}

	// 未审核的不得命中
	if n, _ := repo.FindVisibleMapping(ctx, 100001, "300346.SZ"); n != nil {
		t.Errorf("未审核/已停用的映射不应命中: %+v", n)
	}
	// 不存在的代码
	if n, _ := repo.FindVisibleMapping(ctx, 100001, "999999.SH"); n != nil {
		t.Errorf("不存在的代码不应命中: %+v", n)
	}
}

func TestIntegration_ListStocksByID(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	got, err := repo.ListStocksByID(context.Background(), []int64{1, 2, 3, 999})
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	// 3 已软删、999 不存在 → 只应返回 1 和 2
	if len(got) != 2 {
		t.Fatalf("应返回 2 只（已软删与不存在的排除），实际 %d: %+v", len(got), got)
	}
	s := got[1]
	if s.TsCode != "688502.SH" || s.Symbol != "688502" || s.Name != "茂莱光学" {
		t.Errorf("股票字段错位: %+v", s)
	}
	if s.CnSpell != "MLGX" || s.Market != "科创板" || s.Exchange != "SSE" {
		t.Errorf("股票扩展字段错位: cnspell=%q market=%q exchange=%q", s.CnSpell, s.Market, s.Exchange)
	}
	if _, ok := got[3]; ok {
		t.Error("已软删的股票不应返回")
	}
}

func TestIntegration_SearchThemes(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	got, err := repo.SearchThemes(context.Background(), "光", 10)
	if err != nil {
		t.Fatalf("报错: %v", err)
	}
	// 只搜 level<=1：光刻机 命中；光源/物镜（level 2）不该出现
	names := map[string]bool{}
	for _, t2 := range got {
		names[t2.Name] = true
	}
	if !names["光刻机"] {
		t.Errorf("应搜到「光刻机」，实际 %v", names)
	}
	if names["光源"] || names["物镜"] {
		t.Errorf("产业链环节不应出现在题材搜索结果里，实际 %v", names)
	}
}

// 空入参不得拼出 IN () 这种语法错误。
func TestIntegration_EmptyInputsAreSafe(t *testing.T) {
	repo := NewThemeRepo(setupDB(t))
	ctx := context.Background()

	if got, err := repo.ListVisibleMappings(ctx, nil); err != nil || len(got) != 0 {
		t.Errorf("空题材列表应安全返回空，得到 (%v, %v)", got, err)
	}
	if got, err := repo.ListStocksByID(ctx, nil); err != nil || len(got) != 0 {
		t.Errorf("空 id 列表应安全返回空，得到 (%v, %v)", got, err)
	}
}
