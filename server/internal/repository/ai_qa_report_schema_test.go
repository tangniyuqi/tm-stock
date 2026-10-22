package repository

import (
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/migrations"
)

// 报告追问复用同一问答存储，但只允许已关联任务的 report_qa 消息和 report: 批准前缀。
func TestQASchemaAllowsReportQuestionsWithReportApproval(t *testing.T) {
	for _, needle := range []string{"m.skill='report_qa'", "m.task_id BETWEEN 1 AND 9007199254740991", "'$.ScopeApproval'", "'report:'"} {
		if !strings.Contains(migrations.QASchema, needle) {
			t.Fatal("问答触发器缺少报告追问约束", needle)
		}
	}
	if strings.Contains(migrations.QASchema, "m.skill='report_qa' AND m.created_at") {
		t.Fatal("报告追问不能绕过任务编号约束")
	}
}
