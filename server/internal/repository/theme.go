// Package repository 是数据访问层。
//
// 依赖方向：service → repository（单向）。本层【不得】反向引用 service
// （check-architecture.sh 规则 4）。
//
// 只依赖标准库 database/sql —— MySQL 驱动由 main 注册，本包不 import 它，
// 这样本包无需外部依赖即可编译与测试。
//
// 表归属见 ADR-0007：上游两表由现有系统维护，本项目只读。
//
// ⚠️ DSN 必须带 parseTime=true，否则 datetime(3) 列扫进 time.Time 会报
//
//	"unsupported Scan, storing driver.Value type []uint8 into type *time.Time"
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// 🔴 合规命门：关联表查询的基础约束
//
// 这个片段是【唯一】允许用来查 addon_quant_theme_stock 的过滤条件。
// 收敛成常量而不是各处手写，是为了让「漏掉某一条」无法悄悄发生——
// 并且能被单元测试断言（见 theme_sql_test.go）。
//
// 四条缺一不可，漏任意一条都是【静默泄漏】：不报错，只是把不该露的露出去。
//
//	deleted_at IS NULL   现有表全都是软删除，漏了就带出已删除数据
//	audit_status = 2     未审核的归属不得对外（ADR-0007）
//	status = 1           已停用的不得对外
//	证据三项非空          无依据的归属不构成客观事实（ADR-0003）
//
// 服务层还会再校验一次（model.HasCompleteEvidence / IsVisibleToPublic）。
// 不是重复劳动：任意一层被改坏时，其余层仍然拦得住。
// ─────────────────────────────────────────────────────────────────────────────
const mappingBaseWhere = `
    ts.deleted_at IS NULL
    AND ts.audit_status = 2
    AND ts.status = 1
    AND ts.source_type > 0
    AND ts.source_excerpt <> ''
    AND ts.source_url <> ''`

// 上游两表同样是软删除，JOIN 时必须一并过滤，否则会带出已删除的题材/股票。
const upstreamJoin = `
    JOIN addon_quant_base_stock s ON s.id = ts.stock_id AND s.deleted_at IS NULL
    JOIN addon_quant_theme      t ON t.id = ts.theme_id AND t.deleted_at IS NULL`

const mappingColumns = `
    ts.id, ts.theme_id, ts.stock_id, ts.ts_code,
    ts.source_type, ts.source_excerpt, ts.source_url, ts.collected_at,
    ts.audit_status, ts.status, ts.sort`

// listVisibleMappingsSQL / findVisibleMappingSQL 抽成函数而非内联，
// 是为了让 theme_sql_test.go 能直接断言「过滤条件确实在 SQL 里」——
// 不需要数据库即可守护本项目最关键的一条合规约束。
//
// 查询按【题材及其所有子节点（产业链环节）】取，因此 theme_id 用 IN (?, 子节点…)。
// 这里返回带一个占位符的模板，调用方按子节点数量扩展。
func listVisibleMappingsSQL(themeIDCount int) string {
	ph := placeholders(themeIDCount)
	return `SELECT` + mappingColumns + `
	      FROM addon_quant_theme_stock ts` + upstreamJoin + `
	      WHERE ts.theme_id IN (` + ph + `) AND` + mappingBaseWhere + `
	      ORDER BY ts.theme_id, ts.sort, s.ts_code`
}

func findVisibleMappingSQL() string {
	return `SELECT` + mappingColumns + `
	      FROM addon_quant_theme_stock ts` + upstreamJoin + `
	      WHERE ts.theme_id IN (SELECT id FROM addon_quant_theme
	                             WHERE deleted_at IS NULL AND (id = ? OR parent_id = ?))
	        AND ts.ts_code = ? AND` + mappingBaseWhere + `
	      LIMIT 1`
}

// placeholders 生成 "?,?,?"。至少一个，避免 IN () 语法错误。
func placeholders(n int) string {
	if n < 1 {
		n = 1
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// ThemeRepo 基于 database/sql 的题材数据访问实现。
type ThemeRepo struct {
	db *sql.DB
}

// NewThemeRepo 构造。
func NewThemeRepo(db *sql.DB) *ThemeRepo { return &ThemeRepo{db: db} }

// GetTheme 查题材节点。不存在时返回 (nil, nil)，由 service 决定语义。
func (r *ThemeRepo) GetTheme(ctx context.Context, id int64) (*model.Theme, error) {
	const q = `SELECT id, name, IFNULL(code,''), IFNULL(level,0), IFNULL(parent_id,0),
	                  IFNULL(description,''), IFNULL(sort,0), IFNULL(status,1),
	                  IFNULL(updated_at, created_at)
	             FROM addon_quant_theme
	            WHERE id = ? AND deleted_at IS NULL`
	var t model.Theme
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.Name, &t.Code, &t.Level, &t.ParentID,
		&t.Description, &t.Sort, &t.Status, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查题材失败: %w", err)
	}
	return &t, nil
}

// ListChainNodes 查题材下的产业链环节（level 2 子节点）。
func (r *ThemeRepo) ListChainNodes(ctx context.Context, themeID int64) ([]model.Theme, error) {
	const q = `SELECT id, name, IFNULL(code,''), IFNULL(level,0), IFNULL(parent_id,0),
	                  IFNULL(description,''), IFNULL(sort,0), IFNULL(status,1),
	                  IFNULL(updated_at, created_at)
	             FROM addon_quant_theme
	            WHERE parent_id = ? AND deleted_at IS NULL AND IFNULL(status,1) = 1
	            ORDER BY sort, id`
	rows, err := r.db.QueryContext(ctx, q, themeID)
	if err != nil {
		return nil, fmt.Errorf("查产业链环节失败: %w", err)
	}
	defer rows.Close()

	out := make([]model.Theme, 0, 8)
	for rows.Next() {
		var t model.Theme
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.Level, &t.ParentID,
			&t.Description, &t.Sort, &t.Status, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描产业链环节失败: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListVisibleMappings 查题材及其所有环节下【可对外展示】的归属映射。
//
// 方法名带 Visible、且过滤条件写死在 SQL 里（不接受 status 入参），是刻意的：
// 做成可传参的通用方法，漏传一次就是一次合规泄漏。
func (r *ThemeRepo) ListVisibleMappings(ctx context.Context, themeIDs []int64) ([]model.ThemeStockMapping, error) {
	if len(themeIDs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(themeIDs))
	for _, id := range themeIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, listVisibleMappingsSQL(len(themeIDs)), args...)
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

// FindVisibleMapping 查单条映射，用于依据浮层。同样写死过滤条件。
func (r *ThemeRepo) FindVisibleMapping(ctx context.Context, themeID int64, tsCode string) (*model.ThemeStockMapping, error) {
	rows, err := r.db.QueryContext(ctx, findVisibleMappingSQL(), themeID, themeID, tsCode)
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
		&m.ID, &m.ThemeID, &m.StockID, &m.TsCode,
		&m.SourceType, &m.SourceExcerpt, &m.SourceURL, &m.CollectedAt,
		&m.AuditStatus, &m.Status, &m.Sort)
	if err != nil {
		return m, fmt.Errorf("扫描归属映射失败: %w", err)
	}
	return m, nil
}

// ListStocksByID 按主键批量查个股，返回以 id 为键的 map。
func (r *ThemeRepo) ListStocksByID(ctx context.Context, ids []int64) (map[int64]model.Stock, error) {
	out := map[int64]model.Stock{}
	if len(ids) == 0 {
		return out, nil
	}
	q := `SELECT id, IFNULL(ts_code,''), IFNULL(symbol,''), IFNULL(name,''),
	             IFNULL(industry,''), IFNULL(cnspell,''), IFNULL(market,''),
	             IFNULL(exchange,''), IFNULL(list_status,'')
	        FROM addon_quant_base_stock
	       WHERE deleted_at IS NULL AND id IN (` + placeholders(len(ids)) + `)`
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("查个股失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s model.Stock
		if err := rows.Scan(&s.ID, &s.TsCode, &s.Symbol, &s.Name,
			&s.Industry, &s.CnSpell, &s.Market, &s.Exchange, &s.ListStatus); err != nil {
			return nil, fmt.Errorf("扫描个股失败: %w", err)
		}
		out[s.ID] = s
	}
	return out, rows.Err()
}

// SearchThemes 按名称搜索题材（只搜 level=1 的题材本身，不搜环节）。
func (r *ThemeRepo) SearchThemes(ctx context.Context, kw string, limit int) ([]model.Theme, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	const q = `SELECT id, name, IFNULL(code,''), IFNULL(level,0), IFNULL(parent_id,0),
	                  IFNULL(description,''), IFNULL(sort,0), IFNULL(status,1),
	                  IFNULL(updated_at, created_at)
	             FROM addon_quant_theme
	            WHERE deleted_at IS NULL AND IFNULL(status,1) = 1
	              AND IFNULL(level,0) <= 1 AND name LIKE ?
	            ORDER BY sort, id
	            LIMIT ?`
	rows, err := r.db.QueryContext(ctx, q, "%"+kw+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("搜索题材失败: %w", err)
	}
	defer rows.Close()

	out := make([]model.Theme, 0, limit)
	for rows.Next() {
		var t model.Theme
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.Level, &t.ParentID,
			&t.Description, &t.Sort, &t.Status, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描题材失败: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
