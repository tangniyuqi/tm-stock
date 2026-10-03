package initialize

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// addon_quant_theme_stock 是 tm-stock 的合规命门表（依据 NOT NULL + CHECK 非空，ADR-0003），
// 由 server/migrations 的 SQL 迁移管理。GVA 的 AutoMigrate 一旦注册 quant.ThemeStock{}，
// 就会静默改写这张表（去掉 NOT NULL、丢默认值、加回五个评价类列）或建出没有依据约束的旧形态。
// 这个测试用 AST 找出 initialize 包里所有 AutoMigrate 调用的参数，发现 ThemeStock 就失败。
// （要换一种写法绕过，比如先放进变量再传入——AST 同样会看到那个变量的类型字面量。）
func TestAutoMigrateNeverRegistersThemeStock(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned, calls := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", name, err)
		}
		scanned++
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "AutoMigrate" {
				return true
			}
			calls++
			for _, arg := range call.Args {
				ast.Inspect(arg, func(m ast.Node) bool {
					if id, ok := m.(*ast.Ident); ok && id.Name == "ThemeStock" {
						t.Errorf("%s: AutoMigrate 的参数里出现了 ThemeStock —— 这张表由 server/migrations 管理，不得交给 AutoMigrate", fset.Position(id.Pos()))
					}
					return true
				})
			}
			return true
		})
	}
	if scanned < 10 || calls == 0 {
		t.Fatalf("只扫描了 %d 个文件、%d 处 AutoMigrate 调用，扫描范围可能出了问题", scanned, calls)
	}
}
