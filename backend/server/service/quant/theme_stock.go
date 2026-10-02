package quant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type ThemeStockService struct{}

// publicThemeStockMax 是 C 端单次最多返回的关联条数。
const publicThemeStockMax = 100

// 对 C 端可见的关联必须同时满足：未软删（GORM 自动加）、审核已通过、启用中、依据非空。
// 三个条件漏任意一个都是"静默泄漏"——不会报错，只会把不该露的数据露出去（见 server/migrations 里的说明）。
// 依据非空在库里已有 CHECK，这里再写一遍是纵深防御：库约束被削弱时，读路径仍然拦得住。
const publicThemeStockWhere = "addon_quant_theme_stock.audit_status = ? AND addon_quant_theme_stock.status = ? " +
	"AND addon_quant_theme_stock.source_excerpt <> '' AND addon_quant_theme_stock.source_url <> ''"

// syncThemeStockCount 重新统计题材下"C 端可见"的股票数量（审核已通过、启用、未删除）并更新到 addon_quant_theme.stock_count。
// 口径必须与 C 端列表一致：草稿、待审、已驳回、停用的关联不计入，否则题材上显示的数量会比点进去看到的多。
func syncThemeStockCount(tx *gorm.DB, themeIDs []int32) error {
	if len(themeIDs) == 0 {
		return nil
	}
	var rows []struct {
		ThemeId int32
		Count   int64
	}
	if err := tx.Model(&quant.ThemeStock{}).
		Select("theme_id, COUNT(*) AS count").
		Where("theme_id IN ? AND audit_status = ? AND status = ?", themeIDs, quant.ThemeStockAuditPassed, 1).
		Group("theme_id").
		Scan(&rows).Error; err != nil {
		return err
	}
	countMap := make(map[int32]int64, len(rows))
	for _, row := range rows {
		countMap[row.ThemeId] = row.Count
	}
	for _, themeID := range themeIDs {
		count := int32(countMap[themeID])
		if err := tx.Model(&quant.Theme{}).Where("id = ?", themeID).Update("stock_count", count).Error; err != nil {
			return err
		}
	}
	return nil
}

// translateThemeStockDBError 把常见的库约束错误翻成操作员看得懂的话；其余原样返回。
func translateThemeStockDBError(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case 1062:
			return evidenceErr("该题材下已存在这只股票的有效关联（同一题材、同一股票只能有一条）")
		case 3819:
			return evidenceErr("依据不合规：依据类型、原文摘录、原文链接都不能为空")
		}
	}
	return err
}

// prepareThemeStockForSave 校验关联键与依据，并用股票表里的代码回填 ts_code（不信任请求值：权威是 stock_id）。
func prepareThemeStockForSave(tx *gorm.DB, ts *quant.ThemeStock, now time.Time) error {
	if ts.ThemeId == nil || *ts.ThemeId <= 0 {
		return evidenceErr("题材 ID 不能为空")
	}
	if ts.StockId == nil || *ts.StockId <= 0 {
		return evidenceErr("股票 ID 不能为空")
	}
	var themeCount int64
	if err := tx.Model(&quant.Theme{}).Where("id = ?", *ts.ThemeId).Count(&themeCount).Error; err != nil {
		return err
	}
	if themeCount == 0 {
		return evidenceErr("题材不存在或已删除（ID=%d）", *ts.ThemeId)
	}
	var stock quant.BaseStock
	if err := tx.Where("id = ?", *ts.StockId).First(&stock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return evidenceErr("股票不存在或已删除（ID=%d）", *ts.StockId)
		}
		return err
	}
	if stock.TsCode == nil || strings.TrimSpace(*stock.TsCode) == "" {
		return evidenceErr("该股票缺少 TS 代码，无法建立关联（ID=%d）", *ts.StockId)
	}
	ts.TsCode = strings.TrimSpace(*stock.TsCode)
	return validateThemeStockEvidence(ts, now)
}

// CreateThemeStock 创建题材股票关联。
// 新建一律是草稿：请求里带的审核状态、审核人、驳回原因都会被忽略——"通过"只能经 UpdateThemeStock 的审核流转产生。
func (themeStockService *ThemeStockService) CreateThemeStock(ctx context.Context, themeStock *quant.ThemeStock) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := prepareThemeStockForSave(tx, themeStock, time.Now()); err != nil {
			return err
		}
		themeStock.AuditStatus = quant.ThemeStockAuditDraft
		themeStock.AuditBy, themeStock.AuditAt, themeStock.RejectReason = nil, nil, nil
		if err := tx.Create(themeStock).Error; err != nil {
			return translateThemeStockDBError(err)
		}
		return syncThemeStockCount(tx, []int32{*themeStock.ThemeId})
	})
	return err
}

// DeleteThemeStock 删除题材股票记录
func (themeStockService *ThemeStockService) DeleteThemeStock(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ts quant.ThemeStock
		if err := tx.Where("id = ?", id).First(&ts).Error; err != nil {
			return err
		}
		if err := tx.Model(&quant.ThemeStock{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.ThemeStock{}, "id = ?", id).Error; err != nil {
			return err
		}
		if ts.ThemeId != nil {
			return syncThemeStockCount(tx, []int32{*ts.ThemeId})
		}
		return nil
	})
	return err
}

// DeleteThemeStockByIds 批量删除题材股票记录
func (themeStockService *ThemeStockService) DeleteThemeStockByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var themeIDs []int32
		if err := tx.Model(&quant.ThemeStock{}).Where("id IN ?", ids).Distinct().Pluck("theme_id", &themeIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&quant.ThemeStock{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.ThemeStock{}).Error; err != nil {
			return err
		}
		return syncThemeStockCount(tx, themeIDs)
	})
	return err
}

// UpdateThemeStock 更新题材股票关联，并承担审核流转。以整对象提交，请求里的 UpdatedBy 是操作人。
//
// 返回的 auditReset 为 true 表示：这次修改动了关联键或依据，原来的审核结论作废，已被打回草稿，需要重新审核。
// 规则见 planThemeStockAudit。依据文本类字段留空表示"不改"；备注、排序、状态仅在提交时才更新。
func (themeStockService *ThemeStockService) UpdateThemeStock(ctx context.Context, req quant.ThemeStock) (auditReset bool, err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old quant.ThemeStock
		if err := tx.Where("id = ?", req.ID).First(&old).Error; err != nil {
			return err
		}

		upd := old // 以旧记录为底，逐项叠加请求里提交的值
		if req.ThemeId != nil {
			upd.ThemeId = req.ThemeId
		}
		if req.StockId != nil {
			upd.StockId = req.StockId
		}
		if req.SourceType != nil {
			upd.SourceType = req.SourceType
		}
		if strings.TrimSpace(req.SourceExcerpt) != "" {
			upd.SourceExcerpt = req.SourceExcerpt
		}
		if strings.TrimSpace(req.SourceUrl) != "" {
			upd.SourceUrl = req.SourceUrl
		}
		if req.CollectedAt != nil {
			upd.CollectedAt = req.CollectedAt
		}
		upd.RejectReason = req.RejectReason

		now := time.Now()
		// 先规整依据文本再比较，避免首尾空白造成"依据变了"的误判
		upd.SourceExcerpt = strings.TrimSpace(upd.SourceExcerpt)
		upd.SourceUrl = strings.TrimSpace(upd.SourceUrl)

		plan, planErr := planThemeStockAudit(&old, &upd, req.AuditStatus, req.UpdatedBy, now)
		if planErr != nil {
			return planErr
		}
		auditReset = plan.Reset

		// 动了关联键或依据，或要把状态改成"已通过"：依据必须是干净的
		if plan.Changed && (evidenceOrKeyChanged(&old, &upd) || plan.Status == quant.ThemeStockAuditPassed) {
			if err := prepareThemeStockForSave(tx, &upd, now); err != nil {
				return err
			}
		}

		updates := map[string]any{
			"theme_id":       upd.ThemeId,
			"stock_id":       upd.StockId,
			"ts_code":        upd.TsCode,
			"source_type":    upd.SourceType,
			"source_excerpt": upd.SourceExcerpt,
			"source_url":     upd.SourceUrl,
			"collected_at":   upd.CollectedAt,
			"updated_by":     req.UpdatedBy,
		}
		if req.Remark != nil {
			updates["remark"] = req.Remark
		}
		if req.Sort != nil {
			updates["sort"] = req.Sort
		}
		if req.Status != nil {
			updates["status"] = req.Status
		}
		if plan.Changed {
			updates["audit_status"] = plan.Status
			updates["audit_by"] = plan.By
			updates["audit_at"] = plan.At
			updates["reject_reason"] = plan.RejectReason
		}
		if err := tx.Model(&quant.ThemeStock{}).Where("id = ?", old.ID).Updates(updates).Error; err != nil {
			return translateThemeStockDBError(err)
		}

		// 关联键、审核状态、启用状态都会影响题材上的可见数量：新旧题材一起重新统计
		themeIDs := make([]int32, 0, 2)
		if old.ThemeId != nil {
			themeIDs = append(themeIDs, *old.ThemeId)
		}
		if upd.ThemeId != nil && (old.ThemeId == nil || *upd.ThemeId != *old.ThemeId) {
			themeIDs = append(themeIDs, *upd.ThemeId)
		}
		return syncThemeStockCount(tx, themeIDs)
	})
	return auditReset, err
}

// GetThemeStock 根据id获取题材股票记录（后台）
func (themeStockService *ThemeStockService) GetThemeStock(ctx context.Context, id string) (themeStock quant.ThemeStock, err error) {
	err = global.GVA_DB.WithContext(ctx).Preload("Theme").Preload("Stock").Where("id = ?", id).First(&themeStock).Error
	return
}

// escapeLike 转义 LIKE 通配符，避免搜索词里的 % 与 _ 被当成通配；配合 SQL 里的 ESCAPE '!' 使用。
// 选 ! 做转义符是因为它在 MySQL 与 SQLite 的字符串字面量里都不需要再转义（反斜杠两边的写法不一样）。
func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

// GetThemeStockInfoList 分页获取题材股票记录（仅后台：返回全部审核状态的记录，C 端不得调用）
func (themeStockService *ThemeStockService) GetThemeStockInfoList(ctx context.Context, info quantReq.ThemeStockSearch) (list []quant.ThemeStock, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db，并加载题材与股票名称/代码
	db := global.GVA_DB.WithContext(ctx).Model(&quant.ThemeStock{}).Preload("Theme")
	// 按涨跌幅排序时需联表查询股票实时涨跌幅
	if info.OrderKey == "change_pct" {
		db = db.Joins("Stock")
	} else {
		db = db.Preload("Stock")
	}
	var themeStocks []quant.ThemeStock
	// 精确或模糊搜索字段（字段名加表名前缀，避免联表排序时列名歧义）
	if info.ID != nil {
		db = db.Where("addon_quant_theme_stock.id = ?", *info.ID)
	}
	if info.ThemeId != nil {
		db = db.Where("addon_quant_theme_stock.theme_id = ?", *info.ThemeId)
	}
	if info.StockId != nil {
		db = db.Where("addon_quant_theme_stock.stock_id = ?", *info.StockId)
	}
	if info.SourceType != nil {
		db = db.Where("addon_quant_theme_stock.source_type = ?", *info.SourceType)
	}
	if info.SourceExcerpt != nil && strings.TrimSpace(*info.SourceExcerpt) != "" {
		db = db.Where("addon_quant_theme_stock.source_excerpt LIKE ? ESCAPE '!'", "%"+escapeLike(strings.TrimSpace(*info.SourceExcerpt))+"%")
	}
	if info.AuditStatus != nil {
		db = db.Where("addon_quant_theme_stock.audit_status = ?", *info.AuditStatus)
	}
	if info.Status != nil {
		db = db.Where("addon_quant_theme_stock.status = ?", *info.Status)
	}
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("addon_quant_theme_stock.created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}
	// 支持前端传入 sort 作为筛选项（非排序）
	if info.Sort != nil {
		db = db.Where("addon_quant_theme_stock.sort = ?", *info.Sort)
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	// 默认排序：sort 倒序、id 倒序（只用客观字段）
	defaultOrder := "addon_quant_theme_stock.sort DESC, addon_quant_theme_stock.id DESC"
	// 可排序字段白名单（防止注入非法字段）
	orderFieldMap := map[string]string{
		"id":           "addon_quant_theme_stock.id",
		"sort":         "addon_quant_theme_stock.sort",
		"collected_at": "addon_quant_theme_stock.collected_at",
		"audit_status": "addon_quant_theme_stock.audit_status",
		"change_pct":   "Stock.change_pct",
	}
	orderStr := defaultOrder
	if info.OrderKey != "" {
		if field, ok := orderFieldMap[info.OrderKey]; ok {
			orderDirect := "ASC"
			if info.Desc {
				orderDirect = "DESC"
			}
			orderStr = field + " " + orderDirect + ", addon_quant_theme_stock.id DESC"
		}
	}

	err = db.Order(orderStr).Find(&themeStocks).Error
	return themeStocks, total, err
}

// ListPublicThemeStocks 返回题材下"C 端可见"的关联：审核已通过、启用中、未删除、依据非空，
// 且题材与股票本身也未被删除。排序只用客观字段（股票代码），不使用任何人工权重。
//
// 这是 C 端读取题材股票的唯一入口。后台列表 GetThemeStockInfoList 会返回草稿与已驳回的记录，
// 绝不能直接拿给 C 端用。
func (themeStockService *ThemeStockService) ListPublicThemeStocks(ctx context.Context, themeID int32, limit int) ([]quant.ThemeStock, error) {
	if limit <= 0 || limit > publicThemeStockMax {
		limit = publicThemeStockMax
	}
	var list []quant.ThemeStock
	err := global.GVA_DB.WithContext(ctx).Model(&quant.ThemeStock{}).
		InnerJoins("Theme").InnerJoins("Stock").
		Where("addon_quant_theme_stock.theme_id = ?", themeID).
		Where(publicThemeStockWhere, quant.ThemeStockAuditPassed, 1).
		Order("Stock.ts_code ASC, addon_quant_theme_stock.id ASC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("查询题材股票失败: %w", err)
	}
	return list, nil
}

func (themeStockService *ThemeStockService) GetThemeStockPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
