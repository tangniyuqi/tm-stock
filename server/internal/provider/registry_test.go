package provider_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

const registryJSON = `[
  {"id":"m-b","vendor":"厂商乙","name":"模型乙","filing_no":"备案-002","enabled_for_c":true,"caps":["json_object","json_schema"],"temp_min":0.01,"temp_max":1},
  {"id":"m-a","vendor":"厂商甲","name":"模型甲","filing_no":"备案-001","enabled_for_c":true,"caps":["json_object","tools","stream"],"temp_min":0,"temp_max":2},
  {"id":"m-c","vendor":"厂商丙","name":"模型丙","filing_no":"","enabled_for_c":false,"caps":[],"temp_min":0,"temp_max":0}
]`

// quoted 把字符串编码成 JSON 字符串字面量，用于在测试里安全地嵌入不可见字符。
func quoted(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	return string(b)
}

func Test注册表_加载与查询(t *testing.T) {
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	inputs := []struct {
		name string
		text string
	}{
		{"顶层是数组", registryJSON},
		{"顶层是带models的对象", `{"models":` + registryJSON + `}`},
		{"文件带BOM", bom + registryJSON},
		{"前后有空白", "\n  " + registryJSON + "  \n"},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			reg, err := provider.LoadRegistry(strings.NewReader(in.text))
			if err != nil {
				t.Fatalf("加载失败: %v", err)
			}

			a, ok := reg.Lookup("m-a")
			if !ok || a.Vendor != "厂商甲" || a.Name != "模型甲" || a.FilingNo != "备案-001" || !a.EnabledForC {
				t.Fatalf("Lookup(m-a) = %+v, %v", a, ok)
			}
			if a.TempMin != 0 || a.TempMax != 2 {
				t.Fatalf("m-a 的温度范围 = [%v, %v]，期望 [0, 2]", a.TempMin, a.TempMax)
			}
			if got, want := a.Caps(), provider.CapJSONObject|provider.CapTools|provider.CapStream; got != want {
				t.Fatalf("m-a 的能力 = %v，期望 %v", got, want)
			}
			b, _ := reg.Lookup("m-b")
			if got, want := b.Caps(), provider.CapJSONObject|provider.CapJSONSchema; got != want || b.TempMin != 0.01 || b.TempMax != 1 {
				t.Fatalf("m-b 的能力 = %v（期望 %v），温度范围 = [%v, %v]", got, want, b.TempMin, b.TempMax)
			}
			c, ok := reg.Lookup("m-c")
			if !ok || c.Caps() != 0 || c.EnabledForC {
				t.Fatalf("Lookup(m-c) = %+v, %v", c, ok)
			}
			if _, ok := reg.Lookup("不存在"); ok {
				t.Fatal("不存在的 id 不应被找到")
			}

			forC := reg.ForC()
			if len(forC) != 2 || forC[0].ID != "m-a" || forC[1].ID != "m-b" {
				t.Fatalf("ForC() = %+v，期望只含启用于 C 端的 m-a、m-b，且按 id 升序", forC)
			}
			for _, m := range forC {
				if strings.TrimSpace(m.FilingNo) == "" {
					t.Fatalf("启用于 C 端的模型 %s 没有备案号", m.ID)
				}
			}
		})
	}
}

func Test注册表_校验失败的各种情形(t *testing.T) {
	filing := func(value string) string {
		return fmt.Sprintf(`[{"id":"a","enabled_for_c":true,"filing_no":%s}]`, quoted(t, value))
	}
	cases := []struct {
		name   string
		in     string
		wantIs error // 为 nil 表示只要求失败，不要求特定原因
	}{
		{"id重复", `[{"id":"a"},{"id":"a"}]`, provider.ErrDuplicateModelID},
		{"未知能力", `[{"id":"a","caps":["vision"]}]`, provider.ErrUnknownCap},
		{"能力名区分大小写", `[{"id":"a","caps":["JSON_OBJECT"]}]`, provider.ErrUnknownCap},
		{"启用于C端却备案号为空串", `[{"id":"a","enabled_for_c":true,"filing_no":""}]`, provider.ErrMissingFilingNo},
		{"启用于C端却没有备案号字段", `[{"id":"a","enabled_for_c":true}]`, provider.ErrMissingFilingNo},
		{"备案号仅空格与制表换行", `[{"id":"a","enabled_for_c":true,"filing_no":"   \t\n"}]`, provider.ErrMissingFilingNo},
		{"备案号仅全角空格", filing(string(rune(0x3000))), provider.ErrMissingFilingNo},
		{"备案号仅零宽空格", filing(string(rune(0x200B))), provider.ErrMissingFilingNo},
		{"备案号仅BOM字符", filing(string(rune(0xFEFF))), provider.ErrMissingFilingNo},
		{"备案号是各种不可见字符的混合", filing(" " + string(rune(0x200B)) + string(rune(0x3000)) + string(rune(0xA0)) + string(rune(0x2060))), provider.ErrMissingFilingNo},
		{"id为空串", `[{"id":""}]`, provider.ErrInvalidModel},
		{"id仅空白", `[{"id":"  "}]`, provider.ErrInvalidModel},
		{"id首尾有空白", `[{"id":" a "}]`, provider.ErrInvalidModel},
		{"缺少id", `[{"vendor":"x"}]`, provider.ErrInvalidModel},
		{"温度范围上下限颠倒", `[{"id":"a","temp_min":1,"temp_max":0.5}]`, provider.ErrInvalidModel},
		{"温度下限为负", `[{"id":"a","temp_min":-1,"temp_max":1}]`, provider.ErrInvalidModel},
		{"只登记了温度下限", `[{"id":"a","temp_min":0.5}]`, provider.ErrInvalidModel},
		{"条目里有未知字段", `[{"id":"a","extra":1}]`, nil},
		{"包装对象里有未知字段", `{"models":[],"extra":1}`, nil},
		{"JSON语法错误", `[{"id":"a"`, nil},
		{"字段类型错误", `[{"id":1}]`, nil},
		{"enabled_for_c类型错误", `[{"id":"a","enabled_for_c":"yes"}]`, nil},
		{"包装对象里models不是数组", `{"models":{}}`, nil},
		{"JSON之后还有第二个值", `[] []`, nil},
		{"JSON之后有垃圾内容", `[] x`, nil},
		{"空输入", ``, nil},
		{"仅空白", " \n\t ", nil},
		{"顶层是字符串", `"abc"`, nil},
		{"顶层是数字", `42`, nil},
		{"顶层是null", `null`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg, err := provider.LoadRegistry(strings.NewReader(c.in))
			if err == nil {
				t.Fatalf("期望加载失败，却得到注册表 %+v", reg)
			}
			if reg != nil {
				t.Fatal("加载失败时不应返回注册表")
			}
			if c.wantIs != nil && !errors.Is(err, c.wantIs) {
				t.Fatalf("错误 = %v，期望属于 %v", err, c.wantIs)
			}
		})
	}

	t.Run("读取输入失败", func(t *testing.T) {
		if _, err := provider.LoadRegistry(iotest.ErrReader(errors.New("读取失败"))); err == nil {
			t.Fatal("读取失败应返回错误")
		}
	})
	t.Run("输入为nil", func(t *testing.T) {
		if _, err := provider.LoadRegistry(nil); err == nil {
			t.Fatal("nil 输入应返回错误")
		}
	})
	t.Run("输入超过1MiB", func(t *testing.T) {
		if _, err := provider.LoadRegistry(strings.NewReader(strings.Repeat(" ", 1<<20+1))); err == nil {
			t.Fatal("超大输入应返回错误")
		}
	})
}

func Test注册表_合法但边界的输入(t *testing.T) {
	t.Run("不启用于C端时允许没有备案号", func(t *testing.T) {
		reg, err := provider.LoadRegistry(strings.NewReader(`[{"id":"a","enabled_for_c":false}]`))
		if err != nil {
			t.Fatalf("加载失败: %v", err)
		}
		if _, ok := reg.Lookup("a"); !ok || len(reg.ForC()) != 0 {
			t.Fatal("a 应可查到，且不在 C 端清单里")
		}
	})

	t.Run("备案号两侧有空白但有实质内容时通过", func(t *testing.T) {
		in := fmt.Sprintf(`[{"id":"a","enabled_for_c":true,"filing_no":%s}]`, quoted(t, " 备案-9 "+string(rune(0x200B))))
		if _, err := provider.LoadRegistry(strings.NewReader(in)); err != nil {
			t.Fatalf("加载失败: %v", err)
		}
	})

	t.Run("能力名重复不影响结果", func(t *testing.T) {
		reg, err := provider.LoadRegistry(strings.NewReader(`[{"id":"a","caps":["tools","tools"]}]`))
		if err != nil {
			t.Fatalf("加载失败: %v", err)
		}
		m, _ := reg.Lookup("a")
		if m.Caps() != provider.CapTools {
			t.Fatalf("能力 = %v，期望 tools", m.Caps())
		}
	})

	t.Run("空数组与空对象都是合法的空注册表", func(t *testing.T) {
		for _, in := range []string{`[]`, `{}`, `{"models":[]}`} {
			reg, err := provider.LoadRegistry(strings.NewReader(in))
			if err != nil {
				t.Fatalf("%s 加载失败: %v", in, err)
			}
			if forC := reg.ForC(); forC == nil || len(forC) != 0 {
				t.Fatalf("%s 的 ForC() = %v，期望非 nil 的空切片", in, forC)
			}
		}
	})

	t.Run("Model.Caps忽略不认识的能力名", func(t *testing.T) {
		m := provider.Model{CapList: []string{"tools", "bogus"}}
		if m.Caps() != provider.CapTools {
			t.Fatalf("Caps() = %v，期望只有 tools", m.Caps())
		}
	})
}

func Test注册表_返回的是副本(t *testing.T) {
	reg, err := provider.LoadRegistry(strings.NewReader(registryJSON))
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}

	m, _ := reg.Lookup("m-a")
	m.CapList[0] = "stream"
	m.FilingNo = "被改了"
	again, _ := reg.Lookup("m-a")
	if again.CapList[0] != "json_object" || again.FilingNo != "备案-001" {
		t.Fatalf("修改 Lookup 的返回值不应影响注册表: %+v", again)
	}

	forC := reg.ForC()
	forC[0].CapList[0] = "stream"
	forC[0] = provider.Model{}
	fresh := reg.ForC()
	if fresh[0].ID != "m-a" || fresh[0].CapList[0] != "json_object" {
		t.Fatalf("修改 ForC 的返回值不应影响注册表: %+v", fresh[0])
	}
}

func Test注册表_nil接收者安全(t *testing.T) {
	var reg *provider.Registry
	if _, ok := reg.Lookup("a"); ok {
		t.Fatal("nil 注册表的 Lookup 应返回 false")
	}
	if forC := reg.ForC(); forC == nil || len(forC) != 0 {
		t.Fatalf("nil 注册表的 ForC() = %v，期望非 nil 的空切片", forC)
	}
}
