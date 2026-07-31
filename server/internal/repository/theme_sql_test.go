package repository

import (
	"strings"
	"testing"
)

// 本文件守护本项目最关键的一组合规约束：
// 任何查询 addon_quant_theme_stock 的 SQL 都必须同时带上
//   ① deleted_at IS NULL     现有表全是软删除，漏了会带出已删除数据
//   ② audit_status = 2       未审核的归属不得对外（ADR-0007）
//   ③ status = 1             已停用的不得对外
//   ④ 证据三项非空            无依据不构成客观归属（ADR-0003）
// 且 JOIN 上游两表时同样要过滤它们的 deleted_at。
//
// 为什么值得单独写测试：这些过滤一旦被谁"顺手简化"掉，
// 后果是【未审核 / 无依据 / 已删除的个股归属直接对用户可见】——
// 那正是 ADR-0003 里"退化成品种选择"的情形，属公司级风险。
// 而它不会有任何报错，静默泄漏。
//
// 这些断言不需要数据库，因此在 CI 每次构建都会跑。

// requiredFilters 是所有映射查询都必须包含的片段（比对前会剥掉空白）。
var requiredFilters = []struct {
	name    string
	snippet string
}{
	{"软删除过滤", "ts.deleted_atISNULL"},
	{"审核状态过滤", "ts.audit_status=2"},
	{"启用状态过滤", "ts.status=1"},
	{"来源类型有效", "ts.source_type>0"},
	{"原文摘录非空", "ts.source_excerpt<>''"},
	{"原文链接非空", "ts.source_url<>''"},
}

// compact 剥掉所有空白再比对。
// 直接比对原串会依赖 SQL 里的缩进与换行，一次格式调整就让测试假失败——
// 初版在别处踩过这个，这里从一开始就规避。
func compact(s string) string { return strings.Join(strings.Fields(s), "") }

func TestMappingBaseWhereHasAllFilters(t *testing.T) {
	got := compact(mappingBaseWhere)
	for _, f := range requiredFilters {
		if !strings.Contains(got, f.snippet) {
			t.Errorf("mappingBaseWhere 缺少「%s」（应含 %q）——这是合规过滤，不得移除\n当前:%s",
				f.name, f.snippet, mappingBaseWhere)
		}
	}
}

// JOIN 上游两表时必须过滤它们各自的 deleted_at，
// 否则已删除的题材或股票仍会被带出来。
func TestUpstreamJoinFiltersSoftDelete(t *testing.T) {
	got := compact(upstreamJoin)
	// 分项断言而不是拼一整条长串：剥空白后表别名会和表名粘连
	// （addon_quant_base_stock s ON → ...stocksON...），
	// 长串断言既难写对也难读懂——初版就在这里写错过。
	checks := []struct {
		name string
		want string
	}{
		{"关联股票表", "JOINaddon_quant_base_stock"},
		{"股票表软删除过滤", "s.deleted_atISNULL"},
		{"股票表关联条件", "s.id=ts.stock_id"},
		{"关联题材表", "JOINaddon_quant_theme"},
		{"题材表软删除过滤", "t.deleted_atISNULL"},
		{"题材表关联条件", "t.id=ts.theme_id"},
	}
	for _, c := range checks {
		if !strings.Contains(got, c.want) {
			t.Errorf("upstreamJoin 缺少「%s」（应含 %q）\n当前:%s", c.name, c.want, upstreamJoin)
		}
	}
}

// 两个映射查询都必须走 mappingBaseWhere 与 upstreamJoin——
// 不允许谁自己手写一套过滤条件。
func TestMappingQueriesUseSharedFilters(t *testing.T) {
	cases := map[string]string{
		"ListVisibleMappings": listVisibleMappingsSQL(3),
		"FindVisibleMapping":  findVisibleMappingSQL(),
	}
	for name, sql := range cases {
		got := compact(sql)
		for _, f := range requiredFilters {
			if !strings.Contains(got, f.snippet) {
				t.Errorf("%s 缺少「%s」——不得绕过 mappingBaseWhere 手写过滤条件", name, f.name)
			}
		}
		if !strings.Contains(got, "s.deleted_atISNULL") || !strings.Contains(got, "t.deleted_atISNULL") {
			t.Errorf("%s 未过滤上游表软删除", name)
		}
	}
}

// 参数绑定底线：不得把值拼进 SQL。
func TestMappingQueriesUseBoundParameters(t *testing.T) {
	for name, sql := range map[string]string{
		"ListVisibleMappings": listVisibleMappingsSQL(2),
		"FindVisibleMapping":  findVisibleMappingSQL(),
	} {
		if !strings.Contains(sql, "?") {
			t.Errorf("%s 未使用参数绑定", name)
		}
		// 允许的字面量只有 ''（非空判断）。其余单引号内容视为可疑拼接。
		if strings.Contains(strings.ReplaceAll(sql, "''", ""), "'") {
			t.Errorf("%s 含疑似拼接的字面量（残留单引号）:\n%s", name, sql)
		}
	}
}

// IN 占位符数量必须与传入的题材 id 数一致，否则要么报错要么少查。
func TestPlaceholdersCount(t *testing.T) {
	for n, want := range map[int]string{1: "?", 2: "?,?", 5: "?,?,?,?,?"} {
		if got := placeholders(n); got != want {
			t.Errorf("placeholders(%d) = %q，期望 %q", n, got, want)
		}
	}
	// 0 或负数要退化成 1 个占位符，避免生成 IN () 这种语法错误
	for _, n := range []int{0, -1} {
		if got := placeholders(n); got != "?" {
			t.Errorf("placeholders(%d) = %q，应退化为 %q 以免 IN () 语法错误", n, got, "?")
		}
	}
	if got := strings.Count(listVisibleMappingsSQL(4), "?"); got != 4 {
		t.Errorf("listVisibleMappingsSQL(4) 应有 4 个占位符，实际 %d", got)
	}
}

// 列顺序必须与 scanMapping 的扫描顺序一致。
// 这条挡的是"加了一列却忘了改 Scan"——那种错误在运行时表现为字段错位
// （比如把 audit_status 扫进 SourceURL），非常难查。
func TestMappingColumnsMatchScanOrder(t *testing.T) {
	want := []string{
		"ts.id", "ts.theme_id", "ts.stock_id", "ts.ts_code",
		"ts.source_type", "ts.source_excerpt", "ts.source_url", "ts.collected_at",
		"ts.audit_status", "ts.status", "ts.sort",
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

// 上游两表由现有系统维护，本项目只读。
// 这条挡的是"顺手在 repository 里写个 UPDATE/INSERT 改上游表"。
func TestNoWritesToUpstreamTables(t *testing.T) {
	all := listVisibleMappingsSQL(1) + findVisibleMappingSQL() + mappingBaseWhere + upstreamJoin
	upper := strings.ToUpper(all)
	for _, verb := range []string{"INSERT ", "UPDATE ", "DELETE ", "ALTER ", "DROP "} {
		if strings.Contains(upper, verb) {
			t.Errorf("查询 SQL 里出现写操作 %q —— 上游两表由现有系统维护，本项目只读（ADR-0007）", verb)
		}
	}
}
