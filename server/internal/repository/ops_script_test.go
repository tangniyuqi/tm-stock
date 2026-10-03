package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 运维脚本 scripts/ops/theme-stock-audit.sql 是"上线前只读核查"，会在上线库（或其副本）上执行。
// 它承诺只读：不建表、不改表、不写数据。这条承诺不能靠"大家记得别往里加写语句"，而要机检——
// 这个静态守卫不需要数据库，随 go test ./... 在 CI 里运行。

const auditScriptName = "theme-stock-audit.sql"

// 顶层语句只允许这几个动词。
var auditAllowedTopLevel = map[string]bool{
	"SET": true, "SELECT": true, "SHOW": true, "PREPARE": true, "EXECUTE": true, "DEALLOCATE": true,
}

// 任何字符串字面量（脚本里的动态 SQL 都以字符串形式出现）都不能以这些动词开头。
var auditWriteVerbs = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true, "DROP": true, "ALTER": true,
	"CREATE": true, "TRUNCATE": true, "RENAME": true, "GRANT": true, "REVOKE": true, "LOAD": true,
	"CALL": true, "LOCK": true, "UNLOCK": true, "FLUSH": true, "KILL": true, "OPTIMIZE": true,
	"ANALYZE": true, "REPAIR": true, "START": true, "BEGIN": true, "COMMIT": true, "ROLLBACK": true,
	"SAVEPOINT": true, "HANDLER": true, "IMPORT": true, "INSTALL": true, "UNINSTALL": true,
	"SHUTDOWN": true, "RESET": true, "PURGE": true, "CHANGE": true, "XA": true,
}

// 无论出现在代码还是字符串里都不允许的片段：落盘、加锁、拖慢、读服务器文件。
var auditForbiddenFragments = []string{
	"INTO OUTFILE", "INTO DUMPFILE", "FOR UPDATE", "LOCK IN SHARE MODE",
	"SLEEP(", "GET_LOCK(", "BENCHMARK(", "LOAD_FILE(",
}

// splitSQL 把脚本切成顶层语句（已去掉 -- 行注释），并收集所有单引号字符串字面量（” 还原为 '）。
// 引号内的分号、-- 都不算语句结尾或注释。
func splitSQL(sql string) (stmts []string, literals []string) {
	var cur, lit strings.Builder
	inSingle, inBacktick := false, false
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}
	rs := []rune(sql)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case inSingle:
			cur.WriteRune(c)
			if c == '\'' {
				if i+1 < len(rs) && rs[i+1] == '\'' { // '' 是转义的单引号
					cur.WriteRune(rs[i+1])
					lit.WriteRune('\'')
					i++
					continue
				}
				inSingle = false
				literals = append(literals, lit.String())
				lit.Reset()
				continue
			}
			lit.WriteRune(c)
		case inBacktick:
			cur.WriteRune(c)
			if c == '`' {
				inBacktick = false
			}
		case c == '\'':
			inSingle = true
			cur.WriteRune(c)
		case c == '`':
			inBacktick = true
			cur.WriteRune(c)
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-' && (i+2 >= len(rs) || rs[i+2] == ' ' || rs[i+2] == '\n' || rs[i+2] == '\r' || rs[i+2] == '\t'):
			for i < len(rs) && rs[i] != '\n' { // 跳到行尾
				i++
			}
			cur.WriteRune('\n')
		case c == ';':
			flush()
		default:
			cur.WriteRune(c)
		}
	}
	flush()
	return stmts, literals
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexFunc(s, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '(' }); i >= 0 {
		s = s[:i]
	}
	return strings.ToUpper(s)
}

// auditReadOnlyViolations 返回脚本里违反"只读"承诺的地方；空切片表示通过。
func auditReadOnlyViolations(sql string) []string {
	var v []string
	stmts, literals := splitSQL(sql)
	for _, s := range stmts {
		w := firstWord(s)
		if !auditAllowedTopLevel[w] {
			v = append(v, "顶层语句不是只读动词："+truncateForReport(s))
			continue
		}
		if w == "SET" {
			rest := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), s[:len("SET")])))
			if !strings.HasPrefix(rest, "NAMES") && !strings.HasPrefix(rest, "@TM_") {
				v = append(v, "SET 只允许 SET NAMES 与会话用户变量 @tm_*："+truncateForReport(s))
			}
		}
		up := strings.ToUpper(s)
		for _, f := range auditForbiddenFragments {
			if strings.Contains(up, f) {
				v = append(v, "出现禁止的片段 "+f+"："+truncateForReport(s))
			}
		}
	}
	for _, lit := range literals {
		if auditWriteVerbs[firstWord(lit)] {
			v = append(v, "字符串字面量（动态 SQL）以写操作动词开头："+truncateForReport(lit))
		}
		up := strings.ToUpper(lit)
		for _, f := range auditForbiddenFragments {
			if strings.Contains(up, f) {
				v = append(v, "字符串字面量里出现禁止的片段 "+f+"："+truncateForReport(lit))
			}
		}
	}
	return v
}

func truncateForReport(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return s
}

func TestAuditScriptIsReadOnly(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "ops", auditScriptName))
	if err != nil {
		t.Fatalf("读运维脚本失败: %v", err)
	}
	sql := strings.ReplaceAll(string(b), "\r\n", "\n")
	if v := auditReadOnlyViolations(sql); len(v) > 0 {
		t.Fatalf("%s 不再是只读脚本：\n  %s", auditScriptName, strings.Join(v, "\n  "))
	}
	// 体检：确认检查器真的读到了内容，而不是因为解析出错而"空过"
	stmts, literals := splitSQL(sql)
	if len(stmts) < 40 || len(literals) < 20 {
		t.Fatalf("只解析出 %d 条语句、%d 个字面量，解析可能出了问题", len(stmts), len(literals))
	}
	body := strings.ToUpper(strings.Join(stmts, "\n"))
	prep, exec, dealloc := strings.Count(body, "PREPARE TM_STMT FROM"), strings.Count(body, "EXECUTE TM_STMT"), strings.Count(body, "DEALLOCATE PREPARE TM_STMT")
	if prep == 0 || prep != exec || exec != dealloc {
		t.Errorf("PREPARE/EXECUTE/DEALLOCATE 不成套：prepare=%d execute=%d deallocate=%d", prep, exec, dealloc)
	}
}

// 守卫自己也要被检验：给它喂一批写操作的写法，必须全部被抓住；再喂几种合法写法，不能误报。
func TestAuditReadOnlyCheckerCatchesWrites(t *testing.T) {
	bad := map[string]string{
		"直接 UPDATE":        "UPDATE t SET a = 1;",
		"夹在后面的 DELETE":     "SET @tm_x = 1; DELETE FROM t;",
		"动态 SQL 里的 INSERT": "SET @tm_s = 'INSERT INTO t VALUES (1)'; PREPARE tm_stmt FROM @tm_s;",
		"IF 分支里的小写 drop":   "SET @tm_s = IF(1, 'SELECT 1', 'drop table t');",
		"SET GLOBAL":       "SET GLOBAL max_connections = 1;",
		"SET 非 tm 前缀变量":    "SET @other = 1;",
		"INTO OUTFILE":     "SELECT * FROM t INTO OUTFILE '/tmp/x';",
		"SLEEP":            "SELECT SLEEP(10);",
		"CREATE":           "CREATE TABLE x (a int);",
		"FOR UPDATE":       "SELECT 1 FOR UPDATE;",
		"注释之后的 TRUNCATE":   "-- 注释里的 DROP 不算\nSELECT 1; TRUNCATE t;",
		"引号里带分号的 UPDATE":   "SET @tm_s = 'SELECT 1; UPDATE t SET a = 1'; UPDATE t SET a = 1;",
		"字面量里的 LOCK":       "SET @tm_s = ' lock tables t write';",
		"字面量里的 FOR UPDATE": "SET @tm_s = 'SELECT * FROM t FOR UPDATE';",
	}
	for name, sql := range bad {
		if len(auditReadOnlyViolations(sql)) == 0 {
			t.Errorf("未抓住写操作：%s\n  %s", name, sql)
		}
	}
	good := map[string]string{
		"注释里提到 UPDATE":      "-- UPDATE、DROP 只是注释\nSELECT 1;",
		"预处理一条 SELECT":      "SET @tm_a = 'SELECT 1'; PREPARE tm_stmt FROM @tm_a; EXECUTE tm_stmt; DEALLOCATE PREPARE tm_stmt;",
		"SHOW CREATE TABLE": "SET @tm_a = 'SHOW CREATE TABLE `t`'; PREPARE tm_stmt FROM @tm_a;",
		"字面量里有转义引号":         "SET @tm_a = IF(1, 'SELECT ''x'' AS c', 'DO 0');",
		"SET NAMES":         "SET NAMES utf8mb4;",
		"引号里的 -- 不是注释":      "SET @tm_a = 'a -- b'; SELECT 1;",
	}
	for name, sql := range good {
		if v := auditReadOnlyViolations(sql); len(v) != 0 {
			t.Errorf("误报：%s\n  %s\n  %v", name, sql, v)
		}
	}
}
