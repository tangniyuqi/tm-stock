package repository

import (
	"strings"
	"testing"
)

// 本文件守护本项目最关键的一条合规约束：
// 任何查询 theme_stock_mapping 的 SQL 都必须同时带上
//   ① status = 'APPROVED'（只有审核通过的对外可见）
//   ② 证据三项非空（无依据不构成客观归属）
//
// 为什么值得单独写测试：这条过滤一旦被谁"顺手简化"掉，
// 后果是【未审核 / 无依据的个股归属直接对用户可见】——
// 那正是 ADR-0003 里"退化成品种选择"的情形，属公司级风险。
// 而它不会有任何报错，静默泄漏。
//
// 这些断言不需要数据库，因此在 CI 每次构建都会跑。

// requiredFilters 是所有映射查询都必须包含的片段。
var requiredFilters = []struct {
	name    string
	snippet string
}{
	{"审核状态过滤", "status = 'APPROVED'"},
	{"来源类型非空", "source_type"},
	{"原文摘录非空", "source_excerpt"},
	{"原文链接非空", "source_url"},
}

func TestMappingBaseWhereHasAllFilters(t *testing.T) {
	for _, f := range requiredFilters {
		if !strings.Contains(mappingBaseWhere, f.snippet) {
			t.Errorf("mappingBaseWhere 缺少「%s」（应含 %q）——这是合规过滤，不得移除",
				f.name, f.snippet)
		}
	}
	// 证据字段必须是「非空」判断，不是仅仅出现在列清单里。
	// 先剥掉所有空白再比对 —— 否则断言会依赖 SQL 里的对齐空格数，
	// 一次 gofmt 或手工对齐就会让测试假失败（初版就踩了这个）。
	compact := strings.Join(strings.Fields(mappingBaseWhere), "")
	for _, col := range []string{"source_type", "source_excerpt", "source_url"} {
		if !strings.Contains(compact, col+"<>''") {
			t.Errorf("%s 必须有 <> '' 非空判断，当前 WHERE:\n%s", col, mappingBaseWhere)
		}
	}
}

// 两个映射查询都必须走 mappingBaseWhere —— 不允许谁自己手写一套 WHERE。
func TestMappingQueriesUseBaseWhere(t *testing.T) {
	cases := map[string]string{
		"ListApprovedMappings": listApprovedMappingsSQL(),
		"FindApprovedMapping":  findApprovedMappingSQL(),
	}
	for name, sql := range cases {
		for _, f := range requiredFilters {
			if !strings.Contains(sql, f.snippet) {
				t.Errorf("%s 的 SQL 缺少「%s」——不得绕过 mappingBaseWhere 手写过滤条件",
					name, f.name)
			}
		}
	}
}

// 参数绑定底线：映射查询不得出现拼接进 SQL 的字面量条件。
// 这里用一个粗判据——SQL 里除了 'APPROVED' 与空串字面量之外，
// 不该出现别的单引号内容。
func TestMappingQueriesUseBoundParameters(t *testing.T) {
	for name, sql := range map[string]string{
		"ListApprovedMappings": listApprovedMappingsSQL(),
		"FindApprovedMapping":  findApprovedMappingSQL(),
	} {
		if !strings.Contains(sql, "theme_id = ?") {
			t.Errorf("%s 必须用参数绑定查 theme_id，不得拼接", name)
		}
		// 允许的字面量：'APPROVED' 和 ''（非空判断）。其余单引号内容视为可疑拼接。
		cleaned := strings.ReplaceAll(sql, "'APPROVED'", "")
		cleaned = strings.ReplaceAll(cleaned, "''", "")
		if strings.Contains(cleaned, "'") {
			t.Errorf("%s 含疑似拼接的字面量（残留单引号）:\n%s", name, sql)
		}
	}
}

// 列顺序必须与 scanMapping 的扫描顺序一致。
// 这条测试挡的是"加了一列却忘了改 Scan"——那种错误在运行时表现为
// 字段错位（比如把 status 扫进 SourceURL），非常难查。
func TestMappingColumnsMatchScanOrder(t *testing.T) {
	want := []string{
		"id", "theme_id", "chain_node_id", "stock_code",
		"source_type", "source_excerpt", "source_url", "collected_at",
		"status", "sort_order",
	}
	got := []string{}
	for _, part := range strings.Split(mappingColumns, ",") {
		if p := strings.TrimSpace(part); p != "" {
			got = append(got, p)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("列数不符：期望 %d 列，实际 %d 列（%v）\nscanMapping 也要同步改",
			len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 列不符：期望 %q，实际 %q —— scanMapping 的扫描顺序必须同步",
				i+1, want[i], got[i])
		}
	}
}
