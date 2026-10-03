package quant

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
)

// 本文件的测试文本里需要"违规词"时，一律从词表副本里取：源码里不写违规字面量，
// 否则合规词门禁会把测试文件本身当成违规（词表只增不减，取第一个词即可）。
func firstBannedWord(t *testing.T) string {
	t.Helper()
	if len(complianceWords) == 0 {
		t.Fatal("词表副本为空：go:embed 的 compliance_words.txt 没读到内容")
	}
	return complianceWords[0]
}

func i8(v int8) *int8           { return &v }
func i32(v int32) *int32        { return &v }
func i64(v int64) *int64        { return &v }
func tp(v time.Time) *time.Time { return &v }

var evidenceNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

func goodThemeStock() quant.ThemeStock {
	return quant.ThemeStock{
		ThemeId:       i32(100010),
		StockId:       i64(1),
		SourceType:    i8(quant.ThemeStockSourceAnnualReport),
		SourceExcerpt: "公司光源模组业务收入占当期营业收入的比例为34.2%。",
		SourceUrl:     "https://www.example.com/annual/2025.pdf",
		CollectedAt:   tp(evidenceNow.Add(-2 * time.Hour)),
	}
}

func TestValidateThemeStockEvidenceAccepts(t *testing.T) {
	ts := goodThemeStock()
	ts.SourceExcerpt = "  \t" + ts.SourceExcerpt + "\n "
	ts.SourceUrl = " " + ts.SourceUrl + " "
	if err := validateThemeStockEvidence(&ts, evidenceNow); err != nil {
		t.Fatalf("合格依据被拒绝：%v", err)
	}
	if ts.SourceExcerpt != strings.TrimSpace(ts.SourceExcerpt) || strings.HasPrefix(ts.SourceUrl, " ") || strings.HasSuffix(ts.SourceUrl, " ") {
		t.Errorf("通过校验后摘录与链接应已去掉首尾空白：%q %q", ts.SourceExcerpt, ts.SourceUrl)
	}
	// 以"无"开头的真实摘录不能被占位规则误伤
	ts = goodThemeStock()
	ts.SourceExcerpt = "无人机整机研发与制造业务收入占比持续提升。"
	if err := validateThemeStockEvidence(&ts, evidenceNow); err != nil {
		t.Errorf("以\"无\"开头的真实摘录被误判为占位：%v", err)
	}
	// http 链接也允许（部分官方站点只有 http）
	ts = goodThemeStock()
	ts.SourceUrl = "http://www.example.com/a"
	if err := validateThemeStockEvidence(&ts, evidenceNow); err != nil {
		t.Errorf("http 链接被拒绝：%v", err)
	}
	// 五种依据类型都合法
	for typ := int8(1); typ <= 5; typ++ {
		ts = goodThemeStock()
		ts.SourceType = i8(typ)
		if err := validateThemeStockEvidence(&ts, evidenceNow); err != nil {
			t.Errorf("依据类型 %d 被拒绝：%v", typ, err)
		}
	}
}

func TestValidateThemeStockEvidenceRejects(t *testing.T) {
	banned := firstBannedWord(t)
	longExcerpt := strings.Repeat("字", maxExcerptRunes+1)
	longURL := "https://www.example.com/" + strings.Repeat("a", maxURLBytes)
	cases := []struct {
		name   string
		mutate func(*quant.ThemeStock)
		want   string // 报错里应出现的片段
	}{
		{"依据类型为空", func(ts *quant.ThemeStock) { ts.SourceType = nil }, "依据类型"},
		{"依据类型为 0", func(ts *quant.ThemeStock) { ts.SourceType = i8(0) }, "依据类型"},
		{"依据类型为 6", func(ts *quant.ThemeStock) { ts.SourceType = i8(6) }, "依据类型"},

		{"摘录为空", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "" }, "不能为空"},
		{"摘录只有空白", func(ts *quant.ThemeStock) { ts.SourceExcerpt = " \t\n " }, "不能为空"},
		{"摘录是见链接", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "见链接" }, "占位"},
		{"摘录是详见公告", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "详见公告" }, "占位"},
		{"摘录是详见公告加章节", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "详见公告第三节" }, "占位"},
		{"摘录带标点的占位", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "【见 链接】" }, "占位"},
		{"摘录是同上", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "同上。" }, "占位"},
		{"摘录是单字无", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "无" }, "占位"},
		{"摘录是 N/A", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "N/A" }, "占位"},
		{"摘录过短", func(ts *quant.ThemeStock) { ts.SourceExcerpt = "光源模组业务" }, "过短"},
		{"摘录过长", func(ts *quant.ThemeStock) { ts.SourceExcerpt = longExcerpt }, "过长"},
		{"摘录含禁用词", func(ts *quant.ThemeStock) {
			ts.SourceExcerpt = "公司被市场称为" + banned + "，主营光源模组。"
		}, banned},
		{"摘录含被空格拆开的禁用词", func(ts *quant.ThemeStock) {
			rs := []rune(banned)
			ts.SourceExcerpt = "公司被市场称为" + string(rs[:1]) + " " + string(rs[1:]) + "，主营光源模组。"
		}, banned},
		{"摘录含被零宽字符拆开的禁用词", func(ts *quant.ThemeStock) {
			rs := []rune(banned)
			ts.SourceExcerpt = "公司被市场称为" + string(rs[:1]) + "​" + string(rs[1:]) + "，主营光源模组。"
		}, banned},
		{"摘录含被标点拆开的禁用词", func(ts *quant.ThemeStock) {
			rs := []rune(banned)
			ts.SourceExcerpt = "公司被市场称为" + string(rs[:1]) + "*" + string(rs[1:]) + "，主营光源模组。"
		}, banned},

		{"链接为空", func(ts *quant.ThemeStock) { ts.SourceUrl = "" }, "链接不能为空"},
		{"链接没有协议", func(ts *quant.ThemeStock) { ts.SourceUrl = "www.example.com/a" }, "http(s)"},
		{"链接是 ftp", func(ts *quant.ThemeStock) { ts.SourceUrl = "ftp://example.com/a" }, "http(s)"},
		{"链接没有主机", func(ts *quant.ThemeStock) { ts.SourceUrl = "https:///a" }, "http(s)"},
		{"链接含空格", func(ts *quant.ThemeStock) { ts.SourceUrl = "https://example.com/a b" }, "http(s)"},
		{"链接含账号口令", func(ts *quant.ThemeStock) { ts.SourceUrl = "https://user:pass@example.com/a" }, "账号口令"},
		{"链接过长", func(ts *quant.ThemeStock) { ts.SourceUrl = longURL }, "过长"},

		{"采集时点为空", func(ts *quant.ThemeStock) { ts.CollectedAt = nil }, "采集时点不能为空"},
		{"采集时点为零值", func(ts *quant.ThemeStock) { ts.CollectedAt = tp(time.Time{}) }, "采集时点不能为空"},
		{"采集时点晚于现在", func(ts *quant.ThemeStock) { ts.CollectedAt = tp(evidenceNow.Add(time.Hour)) }, "晚于当前时间"},
		{"采集时点早于 2000 年", func(ts *quant.ThemeStock) { ts.CollectedAt = tp(time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)) }, "不合理"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ts := goodThemeStock()
			c.mutate(&ts)
			err := validateThemeStockEvidence(&ts, evidenceNow)
			if err == nil {
				t.Fatalf("应当被拒绝，却通过了：%+v", ts)
			}
			if !IsThemeStockEvidenceError(err) {
				t.Errorf("应是依据校验错误（可直接回显给操作员），实际 %T", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("报错应含 %q，实际：%v", c.want, err)
			}
		})
	}
	// 采集时点与现在相差在容忍窗口内（5 分钟）不算"晚于现在"
	ts := goodThemeStock()
	ts.CollectedAt = tp(evidenceNow.Add(4 * time.Minute))
	if err := validateThemeStockEvidence(&ts, evidenceNow); err != nil {
		t.Errorf("时钟漂移容忍窗口内被拒绝：%v", err)
	}
}

func TestIsPlaceholderExcerpt(t *testing.T) {
	for in, want := range map[string]bool{
		"":         true,
		"见链接":      true,
		"详见公告":     true,
		"详见公告第三节":  true,
		"参见公司公告全文": true,
		"  同上  ":   true,
		"TBD":      true,
		"na":       true,
		"无":        true,
		"无人机整机研发与制造业务收入占比持续提升。":        false,
		"公司在公告中披露：详见本公司于同日披露的另一份公告。":   false, // "详见"在句中，不是开头
		"年报显示参见附注十二，公司光源模组业务收入占比34.2%": false,
		"nanotech 业务收入占比 12.5%":        false, // 英文缩写类占位只做整体相等，不做前缀匹配
	} {
		if got := isPlaceholderExcerpt(in); got != want {
			t.Errorf("isPlaceholderExcerpt(%q) = %v，期望 %v", in, got, want)
		}
	}
}

func TestComplianceWordsEmbedded(t *testing.T) {
	if len(complianceWords) < 30 {
		t.Fatalf("词表副本只有 %d 个词（基线 30，只增不减）", len(complianceWords))
	}
	got := parseComplianceWords("# 注释\r\n甲 词\r\n\r\n  乙*词  \r\n")
	if len(got) != 2 || got[0] != "甲词" || got[1] != "乙词" {
		t.Errorf("parseComplianceWords 应去掉注释与空行并压缩空白与标点：%q", got)
	}
}

// 词表副本与真源（scripts/compliance-forbidden-words.txt）必须一致。副本落后，这里的校验就会比
// 用户可见文案的合规门禁更松。修复：bash scripts/sync-guard-words.sh
func TestComplianceWordsMatchSource(t *testing.T) {
	src, err := os.ReadFile("../../../../scripts/compliance-forbidden-words.txt")
	if err != nil {
		t.Fatalf("读不到词表真源：%v", err)
	}
	norm := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	if norm(string(src)) != norm(complianceWordsTxt) {
		t.Fatal("service/quant/compliance_words.txt 与 scripts/compliance-forbidden-words.txt 不一致，" +
			"请在仓库根运行 bash scripts/sync-guard-words.sh（词表只增不减，禁止为过门禁删词）")
	}
}

func TestEvidenceOrKeyChanged(t *testing.T) {
	base := goodThemeStock()
	same := base
	if evidenceOrKeyChanged(&base, &same) {
		t.Fatal("完全相同不应判为变动")
	}
	clone := func() quant.ThemeStock { c := base; return c }
	cases := map[string]func(*quant.ThemeStock){
		"题材变了":    func(c *quant.ThemeStock) { c.ThemeId = i32(*base.ThemeId + 1) },
		"股票变了":    func(c *quant.ThemeStock) { c.StockId = i64(*base.StockId + 1) },
		"依据类型变了":  func(c *quant.ThemeStock) { c.SourceType = i8(quant.ThemeStockSourceProspectus) },
		"摘录变了":    func(c *quant.ThemeStock) { c.SourceExcerpt += "。" },
		"链接变了":    func(c *quant.ThemeStock) { c.SourceUrl += "?x=1" },
		"采集时点变了":  func(c *quant.ThemeStock) { c.CollectedAt = tp(base.CollectedAt.Add(time.Second)) },
		"采集时点被清空": func(c *quant.ThemeStock) { c.CollectedAt = nil },
		"题材被清空":   func(c *quant.ThemeStock) { c.ThemeId = nil },
		"依据类型被清空": func(c *quant.ThemeStock) { c.SourceType = nil },
		"股票被清空":   func(c *quant.ThemeStock) { c.StockId = nil },
	}
	for name, mut := range cases {
		c := clone()
		mut(&c)
		if !evidenceOrKeyChanged(&base, &c) {
			t.Errorf("%s：应判为变动", name)
		}
	}
	// 备注、排序、状态不属于依据
	c := clone()
	c.Sort, c.Status, c.Remark = i32(9), i8(0), nil
	if evidenceOrKeyChanged(&base, &c) {
		t.Error("备注、排序、状态的变化不应判为依据变动")
	}
	// 同一时刻不同 Location 的时间不算变动
	c = clone()
	c.CollectedAt = tp(base.CollectedAt.In(time.FixedZone("X", 8*3600)))
	if evidenceOrKeyChanged(&base, &c) {
		t.Error("同一时刻（不同时区表示）不应判为变动")
	}
}

func TestPlanThemeStockAudit(t *testing.T) {
	now := evidenceNow
	old := goodThemeStock()
	reasonText := "  摘录与原文不符  "
	emptyReason := "   "
	tooLong := strings.Repeat("因", maxRejectRunes+1)

	t.Run("审核状态不合法", func(t *testing.T) {
		for _, bad := range []int8{-1, 4, 9} {
			upd := old
			if _, err := planThemeStockAudit(&old, &upd, bad, 7, now); err == nil || !IsThemeStockEvidenceError(err) {
				t.Errorf("状态 %d 应被拒绝：%v", bad, err)
			}
		}
	})

	t.Run("依据没变且状态没变：审核字段不动", func(t *testing.T) {
		o := old
		o.AuditStatus = quant.ThemeStockAuditPassed
		upd := o
		p, err := planThemeStockAudit(&o, &upd, quant.ThemeStockAuditPassed, 7, now)
		if err != nil || p.Changed || p.Reset || p.Status != quant.ThemeStockAuditPassed {
			t.Fatalf("得到 %+v err=%v", p, err)
		}
	})

	t.Run("草稿改为已通过：记录审核人与时间", func(t *testing.T) {
		upd := old
		p, err := planThemeStockAudit(&old, &upd, quant.ThemeStockAuditPassed, 7, now)
		if err != nil || !p.Changed || p.Status != quant.ThemeStockAuditPassed || p.By == nil || *p.By != 7 || p.At == nil || !p.At.Equal(now) || p.RejectReason != nil {
			t.Fatalf("得到 %+v err=%v", p, err)
		}
	})

	t.Run("驳回必须写原因，且会去掉首尾空白", func(t *testing.T) {
		upd := old
		upd.RejectReason = &emptyReason
		if _, err := planThemeStockAudit(&old, &upd, quant.ThemeStockAuditRejected, 7, now); err == nil {
			t.Error("没有驳回原因应被拒绝")
		}
		upd.RejectReason = nil
		if _, err := planThemeStockAudit(&old, &upd, quant.ThemeStockAuditRejected, 7, now); err == nil {
			t.Error("驳回原因为 nil 应被拒绝")
		}
		upd.RejectReason = &tooLong
		if _, err := planThemeStockAudit(&old, &upd, quant.ThemeStockAuditRejected, 7, now); err == nil {
			t.Error("驳回原因过长应被拒绝")
		}
		upd.RejectReason = &reasonText
		p, err := planThemeStockAudit(&old, &upd, quant.ThemeStockAuditRejected, 7, now)
		if err != nil || p.RejectReason == nil || *p.RejectReason != "摘录与原文不符" || p.By == nil || *p.By != 7 {
			t.Fatalf("得到 %+v err=%v", p, err)
		}
	})

	t.Run("改为草稿或待审：清空审核人时间与驳回原因", func(t *testing.T) {
		o := old
		o.AuditStatus = quant.ThemeStockAuditPassed
		for _, target := range []int8{quant.ThemeStockAuditDraft, quant.ThemeStockAuditPending} {
			upd := o
			p, err := planThemeStockAudit(&o, &upd, target, 7, now)
			if err != nil || !p.Changed || p.Status != target || p.By != nil || p.At != nil || p.RejectReason != nil || p.Reset {
				t.Errorf("目标 %d：得到 %+v err=%v", target, p, err)
			}
		}
	})

	t.Run("依据变了：无论请求什么审核状态都打回草稿（不能改完顺手自审通过）", func(t *testing.T) {
		o := old
		o.AuditStatus = quant.ThemeStockAuditPassed
		for _, requested := range []int8{quant.ThemeStockAuditPassed, quant.ThemeStockAuditPending, quant.ThemeStockAuditRejected} {
			upd := o
			upd.SourceExcerpt = "公司光源模组业务收入占当期营业收入的比例为35.0%。"
			p, err := planThemeStockAudit(&o, &upd, requested, 7, now)
			if err != nil || p.Status != quant.ThemeStockAuditDraft || !p.Reset || !p.Changed || p.By != nil || p.At != nil || p.RejectReason != nil {
				t.Errorf("请求 %d：得到 %+v err=%v", requested, p, err)
			}
		}
		// 草稿状态下改依据：状态本来就是草稿，不算"被重置"，但仍要写库保证审核字段是干净的
		upd := old
		upd.SourceUrl += "?v=2"
		p, err := planThemeStockAudit(&old, &upd, quant.ThemeStockAuditPassed, 7, now)
		if err != nil || p.Status != quant.ThemeStockAuditDraft || p.Reset {
			t.Errorf("草稿改依据：得到 %+v err=%v", p, err)
		}
	})

	t.Run("已驳回的记录改了依据：打回草稿并清掉驳回原因", func(t *testing.T) {
		o := old
		o.AuditStatus = quant.ThemeStockAuditRejected
		o.RejectReason = &reasonText
		upd := o
		upd.SourceExcerpt = "公司光源模组业务收入占当期营业收入的比例为35.0%。"
		p, err := planThemeStockAudit(&o, &upd, quant.ThemeStockAuditRejected, 7, now)
		if err != nil || p.Status != quant.ThemeStockAuditDraft || !p.Reset || p.RejectReason != nil {
			t.Fatalf("得到 %+v err=%v", p, err)
		}
	})
}
