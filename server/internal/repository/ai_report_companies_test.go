package repository

import (
	"context"
	"strings"
	"testing"
)

func TestAIReportCompaniesQueryBoundsAndVisibility(t *testing.T) {
	for _, part := range []string{mappingBaseWhere, upstreamJoin, "FOR SHARE", "ORDER BY s.ts_code,ts.theme_id,ts.id", "LIMIT ?", "IFNULL(t.status,1)=1", "ts.audit_by", "ts.audit_at", "ts.excerpt_trimmed"} {
		if !strings.Contains(reportCompaniesSQL, part) {
			t.Fatal("公司读取缺少必要约束", part)
		}
	}
	if strings.Contains(reportCompaniesSQL, "ts.sort") {
		t.Fatal("报告公司排序受人工排序影响")
	}
	var tx aiTaskTx
	for _, id := range []int64{0, -1} {
		if _, err := tx.ReportCompanies(context.Background(), id); err == nil {
			t.Fatal("坏题材访问数据库")
		}
	}
}
