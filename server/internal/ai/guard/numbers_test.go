package guard

import (
	"reflect"
	"testing"
)

func TestNumberTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"没有数字", nil},
		{"营业收入12.5亿元，同比增长8.5%。", []string{"12.5", "8.5"}},
		{"合同金额1,234万元", []string{"1,234"}},
		{"2025-03-31披露", []string{"2025", "03", "31"}},
		{"证券代码300001。", []string{"300001"}},
		{"句末小数点。共计12.", []string{"12"}},
		{"全角数字１２．５亿", []string{"12.5"}},
		{"三成与十二亿不是数字记号", nil},
		{"A300001B", []string{"300001"}},
		{"1.2.3", []string{"1.2.3"}},
		{"12,5与12.5", []string{"12,5", "12.5"}},
	}
	for _, c := range cases {
		if got := numberTokens(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("numberTokens(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestNumbersGroundedTable(t *testing.T) {
	cases := []struct {
		name    string
		display string
		quote   string
		allowed []string
		want    NumbersReport
	}{
		{"数字全部出现在引文里", "营业收入12.5亿元", "公司2025年营业收入12.5亿元。", nil, NumbersReport{OK: true}},
		{"没有数字", "公司主营算力服务", "公司主营算力服务。", nil, NumbersReport{OK: true}},
		{"改了数", "营业收入12.8亿元", "营业收入12.5亿元。", nil, NumbersReport{Ungrounded: []string{"12.8"}}},
		{"小数位变化：12.5 不被 12.50 支撑", "营业收入12.5亿元", "营业收入12.50亿元。", nil, NumbersReport{Ungrounded: []string{"12.5"}}},
		{"小数点丢失：12.5 与 125 不同", "营业收入12.5亿元", "营业收入125亿元。", nil, NumbersReport{Ungrounded: []string{"12.5"}}},
		{"子串不算：12.5 不被 112.58 支撑", "营业收入12.5亿元", "营业收入112.58亿元。", nil, NumbersReport{Ungrounded: []string{"12.5"}}},
		{"千分位必须逐字一致", "合同金额1234万元", "合同金额1,234万元。", nil, NumbersReport{Ungrounded: []string{"1234"}}},
		{"allowed 提供依据", "证券代码300001", "公司主营算力服务。", []string{"300001"}, NumbersReport{OK: true}},
		{"allowed 里的数字也按完整记号比对", "证券代码300001", "公司主营算力服务。", []string{"1300001"}, NumbersReport{Ungrounded: []string{"300001"}}},
		{"日期拆成记号逐个对账", "2025年3月披露", "公告于2025年3月披露。", nil, NumbersReport{OK: true}},
		{"部分有依据部分没有：只报没有的，按首次出现顺序并去重", "增长8.5%，增长9.1%，再增长9.1%", "同比增长8.5%。", nil, NumbersReport{Ungrounded: []string{"9.1"}}},
		{"全角数字先归一", "营业收入１２．５亿元", "营业收入12.5亿元。", nil, NumbersReport{OK: true}},
		{"百分号与单位被忽略（已知边界：只比数字）", "营业收入12.5%", "营业收入12.5亿元。", nil, NumbersReport{OK: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NumbersGrounded(c.display, c.quote, c.allowed)
			if got.OK != c.want.OK || !reflect.DeepEqual(got.Ungrounded, c.want.Ungrounded) {
				t.Fatalf("NumbersGrounded(%q) = %+v，期望 %+v", c.display, got, c.want)
			}
		})
	}
}
