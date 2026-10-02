package render

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// AC-G5：输出 schema 与 DTO 中不存在 rating、score、rank、target、leader、relevance、tier、
// direction、confidence 类字段。这里用 AST 扫描结构体字段名与 json 标签（不是 grep：注释里提到这些词不应误报，
// 而改个别名、换个大小写写法也躲不过子串匹配），在 CI 的 go test 里运行。
//
// 范围：server/internal/ai 下全部非测试 Go 文件、server/internal/dto 下以 ai 开头的 DTO 文件，
// 以及这两处的 *.schema.json。旧有的快讯检索 DTO 不在范围内（相关度打分在那里是合法的检索指标，
// 见 scripts/check-architecture.sh 规则 7 的说明）。

var forbiddenFieldStems = []string{"rating", "score", "rank", "target", "leader", "relevance", "tier", "direction", "confidence"}

func forbiddenStemIn(name string) string {
	low := strings.ToLower(name)
	for _, s := range forbiddenFieldStems {
		if strings.Contains(low, s) {
			return s
		}
	}
	return ""
}

// forbiddenFieldHits 解析一份 Go 源码，返回含禁用词干的结构体字段名与 json 标签名。
func forbiddenFieldHits(filename, src string) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		for _, fld := range st.Fields.List {
			for _, nm := range fld.Names {
				if s := forbiddenStemIn(nm.Name); s != "" {
					hits = append(hits, filename+": 字段 "+nm.Name+"（含 "+s+"）")
				}
			}
			if fld.Tag != nil {
				tag := reflect.StructTag(strings.Trim(fld.Tag.Value, "`"))
				for _, key := range []string{"json", "yaml", "db", "gorm"} {
					name, _, _ := strings.Cut(tag.Get(key), ",")
					if s := forbiddenStemIn(name); s != "" {
						hits = append(hits, filename+": "+key+" 标签 "+name+"（含 "+s+"）")
					}
				}
			}
		}
		return true
	})
	return hits, nil
}

// schemaKeyHits 递归检查 JSON Schema（或任意 JSON）里的键名。
func schemaKeyHits(filename string, data []byte) ([]string, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	var hits []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, child := range t {
				if s := forbiddenStemIn(k); s != "" {
					hits = append(hits, filename+": 键 "+k+"（含 "+s+"）")
				}
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(v)
	sort.Strings(hits)
	return hits, nil
}

func TestForbiddenFieldScannerDetects(t *testing.T) {
	positive := []string{
		"package x\ntype A struct { Rating string }",
		"package x\ntype A struct { ScoreValue int }",
		"package x\ntype A struct { Foo int `json:\"leaderFlag\"` }",
		"package x\ntype A struct { Foo int `json:\"tier,omitempty\"` }",
		"package x\ntype A struct { MarketDirection string }",
		"package x\ntype A struct { TargetPrice float64 }",
		"package x\ntype A struct { Conf int `json:\"confidence\"` }",
		"package x\ntype A struct { Foo int `yaml:\"rank\"` }",
		"package x\nvar v = struct { RelevanceScore int }{}",
		"package x\ntype A struct { rankInternal int }",
	}
	for _, src := range positive {
		hits, err := forbiddenFieldHits("syn.go", src)
		if err != nil || len(hits) == 0 {
			t.Errorf("扫描器应当命中：%q（hits=%v err=%v）", src, hits, err)
		}
	}
	negative := []string{
		"package x\n// Rating 只在注释里提到\ntype A struct { Name string `json:\"name\"` }",
		"package x\ntype A struct { Pct float64 `json:\"pct\"` }",
		"package x\nconst Rank = 1\nfunc Score() int { return 1 }", // 只有结构体字段与标签才受约束
	}
	for _, src := range negative {
		hits, err := forbiddenFieldHits("syn.go", src)
		if err != nil || len(hits) != 0 {
			t.Errorf("扫描器不应命中：%q（hits=%v err=%v）", src, hits, err)
		}
	}
	if _, err := forbiddenFieldHits("bad.go", "package x\ntype A struct {"); err == nil {
		t.Error("语法错误应返回错误，而不是悄悄当作无命中")
	}

	if hits, err := schemaKeyHits("s.json", []byte(`{"properties":{"claims":{"items":{"properties":{"confidence":{"type":"number"}}}}}}`)); err != nil || len(hits) != 1 {
		t.Errorf("Schema 扫描器应命中 confidence：%v %v", hits, err)
	}
	if hits, err := schemaKeyHits("s.json", []byte(`{"properties":{"predicate":{"type":"string"},"entity":{"type":"string"}}}`)); err != nil || len(hits) != 0 {
		t.Errorf("Schema 扫描器不应命中：%v %v", hits, err)
	}
	if _, err := schemaKeyHits("s.json", []byte(`{bad`)); err == nil {
		t.Error("非法 JSON 应返回错误")
	}
}

func TestAIStructsHaveNoForbiddenFields(t *testing.T) {
	roots := []string{"..", filepath.Join("..", "..", "dto")}
	scannedGo, scannedSchema := 0, 0
	var hits []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			base := d.Name()
			isDTORoot := strings.HasSuffix(filepath.ToSlash(root), "dto")
			if isDTORoot && !strings.HasPrefix(base, "ai") {
				return nil // dto 目录下只扫以 ai 开头的 AI DTO 文件
			}
			switch {
			case strings.HasSuffix(base, ".go") && !strings.HasSuffix(base, "_test.go"):
				src, rerr := os.ReadFile(path)
				if rerr != nil {
					return rerr
				}
				h, perr := forbiddenFieldHits(filepath.ToSlash(path), string(src))
				if perr != nil {
					return perr
				}
				hits = append(hits, h...)
				scannedGo++
			case strings.HasSuffix(base, ".schema.json"):
				data, rerr := os.ReadFile(path)
				if rerr != nil {
					return rerr
				}
				h, perr := schemaKeyHits(filepath.ToSlash(path), data)
				if perr != nil {
					return perr
				}
				hits = append(hits, h...)
				scannedSchema++
			}
			return nil
		})
		if err != nil {
			t.Fatalf("扫描 %s 失败：%v", root, err)
		}
	}
	// 防止扫描范围悄悄变空（路径写错、目录改名）而让守卫形同虚设：guard 与 render 两个包至少有若干源文件。
	if scannedGo < 8 {
		t.Fatalf("只扫描到 %d 个 Go 文件，扫描范围可能出了问题", scannedGo)
	}
	t.Logf("已扫描 %d 个 Go 文件、%d 个 schema 文件", scannedGo, scannedSchema)
	if len(hits) > 0 {
		t.Errorf("AI 输出结构与 DTO 里出现了禁用字段（rating、score、rank、target、leader、relevance、tier、direction、confidence 类，AC-G5）。\n"+
			"正确做法是删掉该字段，不是改名保留：\n  %s", strings.Join(hits, "\n  "))
	}
}
