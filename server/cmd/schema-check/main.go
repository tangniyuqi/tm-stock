// Command schema-check 只读核对 tm-stock 依赖的目标数据库结构。
//
// 它不会执行迁移、不会建表，也不会写入任何业务数据。迁移前先跑它，避免把
// 本仓对上游 addon_quant_* 表的假设当成事实。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

var requiredUpstreamColumns = map[string][]string{
	"addon_quant_theme": {
		"id", "name", "level", "parent_id", "description", "sort", "status",
		"created_at", "updated_at", "deleted_at",
	},
	"addon_quant_base_stock": {
		"id", "ts_code", "symbol", "name", "industry", "cnspell", "market",
		"exchange", "list_status", "deleted_at",
	},
}

var mappingColumns = []string{
	"id", "theme_id", "stock_id", "ts_code", "source_type", "source_excerpt",
	"source_url", "collected_at", "audit_status", "status", "deleted_at", "alive",
}

func main() {
	if err := run(); err != nil {
		log.Printf("目标库核查未通过: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("连接目标数据库失败: %w", err)
	}

	version, err := databaseVersion(ctx, db)
	if err != nil {
		return err
	}
	fmt.Printf("MySQL 版本：%s\n", version)

	columns, err := loadColumns(ctx, db)
	if err != nil {
		return err
	}
	for table, required := range requiredUpstreamColumns {
		if err := requireColumns(table, columns[table], required); err != nil {
			return err
		}
		fmt.Printf("[OK] 上游表 %s 字段满足题材查询依赖\n", table)
	}

	// 拼音检索是规格要求，但上游表归属不在本项目控制内。此处只陈述真实结构，
	// 不会把股票表的 cnspell 错当成题材表能力。
	if _, ok := columns["addon_quant_theme"]["cnspell"]; ok {
		fmt.Println("[OK] 题材表具备 cnspell，可在确认数据质量后实现拼音检索")
	} else {
		fmt.Println("[注意] 题材表无 cnspell：当前只能保证中文题材搜索，拼音首字母搜索需上游提供字段或独立检索索引")
	}

	mapping, exists := columns["addon_quant_theme_stock"]
	if !exists {
		fmt.Println("[待执行] addon_quant_theme_stock 尚不存在；已核对上游表，可在获得授权后执行本仓迁移")
		return nil
	}
	if err := requireColumns("addon_quant_theme_stock", mapping, mappingColumns); err != nil {
		return err
	}
	if err := requireIndex(ctx, db, "addon_quant_theme_stock", "uk_theme_stock_alive"); err != nil {
		return err
	}
	if err := requireCheck(ctx, db, "addon_quant_theme_stock", "chk_theme_stock_evidence"); err != nil {
		return err
	}
	fmt.Println("[OK] addon_quant_theme_stock 字段、唯一索引和证据 CHECK 约束均存在")
	return nil
}

func databaseVersion(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", fmt.Errorf("查询数据库版本失败: %w", err)
	}
	return version, nil
}

func loadColumns(ctx context.Context, db *sql.DB) (map[string]map[string]struct{}, error) {
	const q = `SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name IN ('addon_quant_theme', 'addon_quant_base_stock', 'addon_quant_theme_stock')`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("读取目标库字段失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]struct{})
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return nil, fmt.Errorf("读取目标库字段失败: %w", err)
		}
		if result[table] == nil {
			result[table] = make(map[string]struct{})
		}
		result[table][column] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取目标库字段失败: %w", err)
	}
	return result, nil
}

func requireColumns(table string, actual map[string]struct{}, required []string) error {
	missing := make([]string, 0)
	for _, column := range required {
		if _, ok := actual[column]; !ok {
			missing = append(missing, column)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("表 %s 缺少必要字段: %s", table, strings.Join(missing, ", "))
}

func requireIndex(ctx context.Context, db *sql.DB, table, index string) error {
	const q = `SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`
	var count int
	if err := db.QueryRowContext(ctx, q, table, index).Scan(&count); err != nil {
		return fmt.Errorf("读取索引失败: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("表 %s 缺少必要索引 %s", table, index)
	}
	return nil
}

func requireCheck(ctx context.Context, db *sql.DB, table, check string) error {
	const q = `SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE table_schema = DATABASE() AND table_name = ?
		  AND constraint_type = 'CHECK' AND constraint_name = ?`
	var count int
	if err := db.QueryRowContext(ctx, q, table, check).Scan(&count); err != nil {
		return fmt.Errorf("读取 CHECK 约束失败: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("表 %s 缺少必要 CHECK 约束 %s", table, check)
	}
	return nil
}
