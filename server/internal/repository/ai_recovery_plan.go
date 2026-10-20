package repository

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// ThemeForRecovery 按任务固化的题材编号精确读取，不接受计划文件的自报题材或模糊匹配。
func (c *AIExtractionCatalog) ThemeForRecovery(ctx context.Context, id int64) (string, error) {
	if c == nil || c.db == nil || id <= 0 {
		return "", points.ErrInvalid
	}
	var name string
	if err := c.db.QueryRowContext(ctx, `SELECT name FROM addon_quant_theme WHERE id=? AND deleted_at IS NULL AND IFNULL(status,1)=1 AND name IS NOT NULL AND name<>''`, id).Scan(&name); err != nil {
		return "", pointsDBError("读取恢复题材", err)
	}
	return guard.Normalize(name), nil
}
