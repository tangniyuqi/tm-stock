package quant

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
)

// 题材-股票关联的"依据"校验与审核规则。
//
// 为什么放在服务层：库里的 NOT NULL + CHECK 只能拦住"空"，拦不住"见链接"这类占位、
// 拦不住把评价性措辞写进摘录、也拦不住有人改了依据却保留"已通过"。这些必须在写入口处拦住。
// 本文件的函数都是纯函数（不碰数据库），单测可以直接穷举边界。

const (
	minExcerptRunes = 8    // 摘录至少这么长才可能是一句有意义的原文
	maxExcerptRunes = 1000 // 与列定义 varchar(1000) 一致
	maxURLBytes     = 512  // 与列定义 varchar(512) 一致
	maxRejectRunes  = 250  // 与列定义 varchar(250) 一致
)

// complianceWordsTxt 是 scripts/compliance-forbidden-words.txt 的副本（go:embed 不能嵌入包目录之外的文件）。
// 由 scripts/sync-guard-words.sh 同步，--check 在门禁里比对，词表只增不减。
//
//go:embed compliance_words.txt
var complianceWordsTxt string

var complianceWords = parseComplianceWords(complianceWordsTxt)

func parseComplianceWords(txt string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n") {
		w := strings.TrimSpace(line)
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		if k := compactForMatch(w); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// compactForMatch 去掉空白、控制与零宽字符、标点符号并转小写，让"龙 头""龙*头""龙​头"这类插字写法也能被词表命中。
// 这是网关层的轻量版；server/internal/ai/guard 里有更完整的实现（含繁简折叠与句式规则），L0 改造后写入口会迁过去。
func compactForMatch(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsSpace(r), unicode.IsControl(r), unicode.IsPunct(r), unicode.IsSymbol(r), unicode.Is(unicode.Cf, r):
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// forbiddenWordIn 返回文本命中的第一个合规禁用词；没有则返回空串。
func forbiddenWordIn(text string) string {
	compact := compactForMatch(text)
	for _, w := range complianceWords {
		if strings.Contains(compact, w) {
			return w
		}
	}
	return ""
}

// 占位写法：这些不是原文摘录，只是"看别处"的指路牌。
// 指路类短语用"开头匹配且整体很短"来判定，避免误伤真实原文里偶然出现的"参见""详见"；
// 单字或英文缩写类占位只做整体相等判断（"无人机……"这样的真实摘录不能被"无"误伤）。
var (
	excerptPlaceholderPrefixes = []string{
		"见链接", "详见链接", "见原文", "详见原文", "见公告", "详见公告", "见年报", "详见年报", "见附件", "详见附件",
		"见招股书", "详见招股书", "见上", "同上", "参见", "请见", "请参见", "如题",
	}
	excerptPlaceholderExact = []string{"略", "无", "暂无", "待补充", "待补", "tbd", "todo", "na"}
)

func isPlaceholderExcerpt(excerpt string) bool {
	k := compactForMatch(excerpt)
	if k == "" {
		return true
	}
	for _, p := range excerptPlaceholderExact {
		if k == p {
			return true
		}
	}
	for _, p := range excerptPlaceholderPrefixes {
		if strings.HasPrefix(k, p) && utf8.RuneCountInString(k) <= utf8.RuneCountInString(p)+6 {
			return true
		}
	}
	return false
}

// themeStockEvidenceError 是校验失败的原因，用于直接回显给后台操作员。
type themeStockEvidenceError struct{ msg string }

func (e *themeStockEvidenceError) Error() string { return e.msg }

func evidenceErr(format string, a ...any) error {
	return &themeStockEvidenceError{msg: fmt.Sprintf(format, a...)}
}

// validateThemeStockEvidence 校验并规整（去首尾空白）依据四项。通过返回 nil。
// now 由调用方传入，便于测试"采集时点不得晚于现在"。
func validateThemeStockEvidence(ts *quant.ThemeStock, now time.Time) error {
	if ts.SourceType == nil || *ts.SourceType < quant.ThemeStockSourceAnnouncement || *ts.SourceType > quant.ThemeStockSourceInteractive {
		return evidenceErr("依据类型必须是 1公告 2年报 3招股书 4官方产业目录 5互动易问答 之一")
	}

	ts.SourceExcerpt = strings.TrimSpace(ts.SourceExcerpt)
	n := utf8.RuneCountInString(ts.SourceExcerpt)
	switch {
	case ts.SourceExcerpt == "":
		return evidenceErr("原文摘录不能为空：每条归属都必须有可溯源的依据，无依据禁止入库")
	case n > maxExcerptRunes:
		return evidenceErr("原文摘录过长（%d 字，上限 %d 字）", n, maxExcerptRunes)
	case isPlaceholderExcerpt(ts.SourceExcerpt):
		return evidenceErr("原文摘录不能是“见链接”“详见公告”之类的占位，请填写原文中支撑该归属的那一句话")
	case n < minExcerptRunes:
		return evidenceErr("原文摘录过短（%d 字，至少 %d 字）：请摘录能说明归属依据的完整原文", n, minExcerptRunes)
	}
	if w := forbiddenWordIn(ts.SourceExcerpt); w != "" {
		return evidenceErr("原文摘录含合规禁用词“%s”：用户可见内容不得出现价值评价、收益承诺、买卖时机类措辞。请改摘录同一出处中不含评价的客观句子", w)
	}

	ts.SourceUrl = strings.TrimSpace(ts.SourceUrl)
	if ts.SourceUrl == "" {
		return evidenceErr("原文链接不能为空")
	}
	if len(ts.SourceUrl) > maxURLBytes {
		return evidenceErr("原文链接过长（上限 %d 字节）", maxURLBytes)
	}
	u, err := url.Parse(ts.SourceUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(ts.SourceUrl, " \t\r\n") {
		return evidenceErr("原文链接必须是完整的 http(s) 地址")
	}
	if u.User != nil {
		return evidenceErr("原文链接不得包含账号口令（形如 user:pass@host）")
	}

	switch {
	case ts.CollectedAt == nil || ts.CollectedAt.IsZero():
		return evidenceErr("采集时点不能为空")
	case ts.CollectedAt.After(now.Add(5 * time.Minute)):
		return evidenceErr("采集时点不能晚于当前时间")
	case ts.CollectedAt.Year() < 2000:
		return evidenceErr("采集时点不合理（早于 2000 年）")
	}
	return nil
}

// IsThemeStockEvidenceError 判断错误是否是依据校验失败（可直接回显给操作员）。
func IsThemeStockEvidenceError(err error) bool {
	var e *themeStockEvidenceError
	return errors.As(err, &e)
}

// evidenceOrKeyChanged 判断这次更新是否动了"关联键或依据"。动了就等于换了一条主张，必须重新审核。
func evidenceOrKeyChanged(old, upd *quant.ThemeStock) bool {
	if !int32PtrEqual(old.ThemeId, upd.ThemeId) || !int64PtrEqual(old.StockId, upd.StockId) {
		return true
	}
	if !int8PtrEqual(old.SourceType, upd.SourceType) {
		return true
	}
	if old.SourceExcerpt != upd.SourceExcerpt || old.SourceUrl != upd.SourceUrl {
		return true
	}
	switch {
	case old.CollectedAt == nil && upd.CollectedAt == nil:
		return false
	case old.CollectedAt == nil || upd.CollectedAt == nil:
		return true
	}
	return !old.CollectedAt.Equal(*upd.CollectedAt)
}

func int32PtrEqual(a, b *int32) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func int64PtrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func int8PtrEqual(a, b *int8) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// themeStockAuditPlan 是一次更新里审核字段的最终取值。Reset 表示因依据变动而被打回草稿。
type themeStockAuditPlan struct {
	Status       int8
	By           *uint
	At           *time.Time
	RejectReason *string
	Reset        bool // 依据变动，审核被强制重置为草稿
	Changed      bool // 审核字段需要写库
}

// planThemeStockAudit 根据"依据是否变动"和"请求的审核状态"决定审核字段怎么写。
//
//   - 依据或关联键变了：一律打回草稿并清空审核人，请求里的审核状态被忽略（避免"改完依据顺手自审通过"）。
//   - 依据没变、审核状态没变：审核字段不动。
//   - 改为已通过：记录审核人与时间（前置条件由调用方保证：依据已通过 validateThemeStockEvidence）。
//   - 改为已驳回：必须写驳回原因。
//   - 改为草稿/待审：清空审核人、时间与驳回原因。
func planThemeStockAudit(old, upd *quant.ThemeStock, requested int8, operator uint, now time.Time) (themeStockAuditPlan, error) {
	if requested < quant.ThemeStockAuditDraft || requested > quant.ThemeStockAuditRejected {
		return themeStockAuditPlan{}, evidenceErr("审核状态必须是 0草稿 1待审 2已通过 3已驳回 之一")
	}
	if evidenceOrKeyChanged(old, upd) {
		return themeStockAuditPlan{Status: quant.ThemeStockAuditDraft, Reset: old.AuditStatus != quant.ThemeStockAuditDraft, Changed: true}, nil
	}
	if requested == old.AuditStatus {
		return themeStockAuditPlan{Status: old.AuditStatus}, nil
	}
	switch requested {
	case quant.ThemeStockAuditPassed:
		by, at := operator, now
		return themeStockAuditPlan{Status: requested, By: &by, At: &at, Changed: true}, nil
	case quant.ThemeStockAuditRejected:
		reason := ""
		if upd.RejectReason != nil {
			reason = strings.TrimSpace(*upd.RejectReason)
		}
		if reason == "" {
			return themeStockAuditPlan{}, evidenceErr("驳回必须填写驳回原因")
		}
		if utf8.RuneCountInString(reason) > maxRejectRunes {
			return themeStockAuditPlan{}, evidenceErr("驳回原因过长（上限 %d 字）", maxRejectRunes)
		}
		by, at := operator, now
		return themeStockAuditPlan{Status: requested, By: &by, At: &at, RejectReason: &reason, Changed: true}, nil
	default: // 草稿、待审
		return themeStockAuditPlan{Status: requested, Changed: true}, nil
	}
}
