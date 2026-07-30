// Package repository 是数据访问层。
//
// 依赖方向：service → repository（单向）。本层【不得】反向引用 service
// （check-architecture.sh 规则 4）。
//
// 只依赖标准库 database/sql —— MySQL 驱动由 main 注册，本包不 import 它，
// 这样本包无需外部依赖即可编译与测试。
//
// ⚠️ DSN 必须带 parseTime=true，否则 DATETIME 列扫进 time.Time 会报
//
//	"unsupported Scan, storing driver.Value type []uint8 into type *time.Time"。
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// 🔴 合规命门：映射查询的基础约束
//
// 这两个常量是【唯一】允许用来查 theme_stock_mapping 的片段。
// 拆成常量而不是各处手写 SQL，是为了让「漏掉 status 过滤」这件事
// 无法悄悄发生 —— 并且能被单元测试断言（见 theme_sql_test.go）。
//
// 两条约束缺一不可：
//
//	status = 'APPROVED'  只有审核通过的才对外可见
//	证据四项非空        无依据的映射不构成客观归属（ADR-0003）
//
// 服务层还会再校验一次（model.HasCompleteEvidence）。不是重复劳动：
// 任意一层被改坏时，其余层仍然拦得住。
// ─────────────────────────────────────────────────────────────────────────────
const mappingBaseWhere = `
    status = 'APPROVED'
    AND source_type    <> ''
    AND source_excerpt <> ''
    AND source_url     <> ''`

const mappingColumns = `
    id, theme_id, chain_node_id, stock_code,
    source_type, source_excerpt, source_url, collected_at,
    status, sort_order`

// listApprovedMappingsSQL / findApprovedMappingSQL 抽成函数而非内联，
// 是为了让 theme_sql_test.go 能直接断言「过滤条件确实在 SQL 里」——
// 不需要数据库即可守护本项目最关键的一条合规约束。
func listApprovedMappingsSQL() string {
	return `SELECT` + mappingColumns + `
	      FROM theme_stock_mapping
	      WHERE theme_id = ? AND` + mappingBaseWhere + `
	      ORDER BY chain_node_id, sort_order, stock_code`
}

func findApprovedMappingSQL() string {
	return `SELECT` + mappingColumns + `
	      FROM theme_stock_mapping
	      WHERE theme_id = ? AND stock_code = ? AND` + mappingBaseWhere + `
	      LIMIT 1`
}

// ThemeRepo 基于 database/sql 的题材数据访问实现。
type ThemeRepo struct {
	db *sql.DB
}

// NewThemeRepo 构造。
func NewThemeRepo(db *sql.DB) *ThemeRepo { return &ThemeRepo{db: db} }

// GetTheme 查题材。不存在时返回 (nil, nil)，由 service 决定语义。
func (r *ThemeRepo) GetTheme(ctx context.Context, id int64) (*model.Theme, error) {
	const q = `SELECT id, name, category_id, summary, data_source, updated_at
	           FROM theme WHERE id = ?`
	var t model.Theme
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.Name, &t.CategoryID, &t.Summary, &t.DataSource, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查题材失败: %w", err)
	}
	return &t, nil
}

// GetCategoryPath 自三级分类向上拼出「一级 › 二级 › 三级」路径。
//
// 层级最多 3 层，因此用有界循环而不是递归 CTE —— 可读性优先，
// 且 MySQL 5.7 兼容（虽然本项目用 8.0，但没必要为 3 层写 CTE）。
func (r *ThemeRepo) GetCategoryPath(ctx context.Context, categoryID int64) (string, error) {
	const q = `SELECT name, parent_id FROM theme_category WHERE id = ?`
	names := make([]string, 0, 3)
	id := categoryID
	for i := 0; i < 3 && id != 0; i++ {
		var name string
		var parent int64
		err := r.db.QueryRowContext(ctx, q, id).Scan(&name, &parent)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			return "", fmt.Errorf("查分类路径失败: %w", err)
		}
		names = append(names, name)
		id = parent
	}
	// 自下而上收集，反转成自上而下
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return strings.Join(names, " › "), nil
}

// ListChainNodes 查题材的产业链环节，按 sort_order。
func (r *ThemeRepo) ListChainNodes(ctx context.Context, themeID int64) ([]model.ChainNode, error) {
	const q = `SELECT id, theme_id, name, description, sort_order
	           FROM theme_chain_node WHERE theme_id = ? ORDER BY sort_order, id`
	rows, err := r.db.QueryContext(ctx, q, themeID)
	if err != nil {
		return nil, fmt.Errorf("查产业链环节失败: %w", err)
	}
	defer rows.Close()

	out := make([]model.ChainNode, 0, 8)
	for rows.Next() {
		var n model.ChainNode
		if err := rows.Scan(&n.ID, &n.ThemeID, &n.Name, &n.Description, &n.SortOrder); err != nil {
			return nil, fmt.Errorf("扫描产业链环节失败: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ListApprovedMappings 查题材下【已审核通过且证据齐全】的映射。
//
// 方法名带 Approved、且过滤条件写死在 SQL 里（不接受 status 入参），
// 是刻意的设计：把它做成可传参的通用方法，漏传一次就是一次合规泄漏。
func (r *ThemeRepo) ListApprovedMappings(ctx context.Context, themeID int64) ([]model.ThemeStockMapping, error) {
	rows, err := r.db.QueryContext(ctx, listApprovedMappingsSQL(), themeID)
	if err != nil {
		return nil, fmt.Errorf("查归属映射失败: %w", err)
	}
	defer rows.Close()

	out := make([]model.ThemeStockMapping, 0, 32)
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// FindApprovedMapping 查单条映射，用于依据浮层。同样写死过滤条件。
func (r *ThemeRepo) FindApprovedMapping(ctx context.Context, themeID int64, stockCode string) (*model.ThemeStockMapping, error) {
	rows, err := r.db.QueryContext(ctx, findApprovedMappingSQL(), themeID, stockCode)
	if err != nil {
		return nil, fmt.Errorf("查归属映射失败: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}
	m, err := scanMapping(rows)
	if err != nil {
		return nil, err
	}
	return &m, rows.Err()
}

// scanMapping 集中一处扫描，避免列顺序在两个查询里走样。
func scanMapping(rows *sql.Rows) (model.ThemeStockMapping, error) {
	var m model.ThemeStockMapping
	err := rows.Scan(
		&m.ID, &m.ThemeID, &m.ChainNodeID, &m.StockCode,
		&m.SourceType, &m.SourceExcerpt, &m.SourceURL, &m.CollectedAt,
		&m.Status, &m.SortOrder)
	if err != nil {
		return m, fmt.Errorf("扫描归属映射失败: %w", err)
	}
	return m, nil
}

// ListStocks 按代码批量查个股。返回 map 便于调用方按需取；查不到的键直接缺失。
func (r *ThemeRepo) ListStocks(ctx context.Context, codes []string) (map[string]model.Stock, error) {
	out := map[string]model.Stock{}
	if len(codes) == 0 {
		return out, nil
	}
	// 动态占位符。代码来自库内映射而非用户输入，但仍走参数绑定——
	// 不拼接字面量是底线，不因"这次数据可信"而破例。
	ph := strings.TrimSuffix(strings.Repeat("?,", len(codes)), ",")
	q := `SELECT code, name, market, delisted FROM stock WHERE code IN (` + ph + `)`

	args := make([]any, 0, len(codes))
	for _, c := range codes {
		args = append(args, c)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("查个股失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s model.Stock
		if err := rows.Scan(&s.Code, &s.Name, &s.Market, &s.Delisted); err != nil {
			return nil, fmt.Errorf("扫描个股失败: %w", err)
		}
		out[s.Code] = s
	}
	return out, rows.Err()
}

// ListEvents 查题材关联事件，按发布时间倒序。
func (r *ThemeRepo) ListEvents(ctx context.Context, themeID int64) ([]model.ThemeEvent, error) {
	const q = `SELECT id, theme_id, title, source, source_url, publish_at
	           FROM theme_event WHERE theme_id = ? ORDER BY publish_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, q, themeID)
	if err != nil {
		return nil, fmt.Errorf("查题材事件失败: %w", err)
	}
	defer rows.Close()

	out := make([]model.ThemeEvent, 0, 16)
	for rows.Next() {
		var e model.ThemeEvent
		if err := rows.Scan(&e.ID, &e.ThemeID, &e.Title, &e.Source, &e.SourceURL, &e.PublishAt); err != nil {
			return nil, fmt.Errorf("扫描题材事件失败: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetChainNodeName 取环节名，用于依据浮层标题。
func (r *ThemeRepo) GetChainNodeName(ctx context.Context, nodeID int64) (string, error) {
	const q = `SELECT name FROM theme_chain_node WHERE id = ?`
	var name string
	err := r.db.QueryRowContext(ctx, q, nodeID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("查环节名失败: %w", err)
	}
	return name, nil
}
