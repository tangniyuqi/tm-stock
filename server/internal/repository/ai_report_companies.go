package repository

import (
	"context"
	"database/sql"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var _ finalreport.CompanyReader = (*aiTaskTx)(nil)

// 公司集合只读资料库，不使用 ts.sort，不允许调用方选择证券或传入URL。
const reportCompaniesSQL = `SELECT ts.id,ts.theme_id,IFNULL(t.parent_id,0),ts.stock_id,
 IFNULL(ts.audit_by,0),IFNULL(t.name,''),IFNULL(s.name,''),IFNULL(s.symbol,''),IFNULL(s.ts_code,''),ts.ts_code,
 ts.source_type,ts.audit_status,ts.status,ts.source_excerpt,ts.source_url,ts.excerpt_trimmed,
 CAST(ts.collected_at AS CHAR),CAST(ts.audit_at AS CHAR)
 FROM addon_quant_theme_stock ts` + upstreamJoin + `
 WHERE (t.id=? OR t.parent_id=?) AND IFNULL(t.status,1)=1 AND` + mappingBaseWhere + `
 ORDER BY s.ts_code,ts.theme_id,ts.id LIMIT ? FOR SHARE`

func (t *aiTaskTx) ReportCompanies(ctx context.Context, themeID int64) ([]finalreport.CompanyRecord, error) {
	if themeID <= 0 {
		return nil, points.ErrInvalid
	}
	rows, err := t.tx.QueryContext(ctx, reportCompaniesSQL, themeID, themeID, finalreport.MaxCompanyMappings+1)
	if err != nil {
		return nil, pointsDBError("读取报告已审核公司", err)
	}
	defer rows.Close()
	out := make([]finalreport.CompanyRecord, 0)
	for rows.Next() {
		var r finalreport.CompanyRecord
		var collected string
		var reviewed sql.NullString
		if err := rows.Scan(&r.ID, &r.NodeID, &r.ParentID, &r.StockID, &r.ReviewerID, &r.NodeName, &r.Name, &r.Symbol, &r.TsCode, &r.MappingCode, &r.SourceType, &r.AuditStatus, &r.Status, &r.Excerpt, &r.URL, &r.Trimmed, &collected, &reviewed); err != nil {
			return nil, pointsDBError("扫描报告公司依据", err)
		}
		r.CollectedAt, err = parsePointsUTC(collected)
		if err != nil {
			return nil, err
		}
		if reviewed.Valid {
			r.ReviewedAt, err = parsePointsUTC(reviewed.String)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, pointsDBError("遍历报告公司依据", err)
	}
	if len(out) > finalreport.MaxCompanyMappings {
		return nil, finalreport.ErrRejected
	}
	return out, nil
}
