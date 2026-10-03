package guard

// NumbersReport 是数字对账的结果。
type NumbersReport struct {
	// OK 为 true 表示 display 里的每个数字记号都有依据。
	OK bool
	// Ungrounded 是无依据的数字记号，按首次出现顺序、去重。
	Ungrounded []string
}

// numberTokens 提取数字记号：极大的数字串，内部可含 . 或 , 且两侧必须是数字
// （12.5、1,234、2025、300001）；句末的 . 不属于记号；中文数字不是记号，百分号与单位被忽略。
// 2025-03-31 拆成 2025、03、31。入参先做 Normalize（全角数字会被折成半角）。
func numberTokens(s string) []string {
	rs := []rune(Normalize(s))
	var out []string
	for i := 0; i < len(rs); {
		if !isDigitRune(rs[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(rs) {
			if isDigitRune(rs[j]) {
				j++
				continue
			}
			if (rs[j] == '.' || rs[j] == ',') && j+1 < len(rs) && isDigitRune(rs[j+1]) {
				j += 2
				continue
			}
			break
		}
		out = append(out, string(rs[i:j]))
		i = j
	}
	return out
}

// NumbersGrounded 数字对账（AC-E3）：display 里的每个数字记号，必须逐字等于 quote 或任一
// allowed（结构化字段，如证券代码、来源日期）里的某个"完整数字记号"，而不是它们的子串——
// "12.5" 不被 "112.58" 或 "12.50" 支撑。口径见 eval/README.md 第 4 节。
//
// 已知边界：只比对数字本身，不比对单位与方向（"下降 8.5%" 与 "增长 8.5%" 数字相同）；
// 这类语义错误靠"展示的文字只来自逐字引文与模板"这一结构约束，以及蕴含裁判与人工审核。
func NumbersGrounded(display, quote string, allowed []string) NumbersReport {
	have := map[string]bool{}
	for _, t := range numberTokens(quote) {
		have[t] = true
	}
	for _, a := range allowed {
		for _, t := range numberTokens(a) {
			have[t] = true
		}
	}
	var missing []string
	seen := map[string]bool{}
	for _, t := range numberTokens(display) {
		if have[t] || seen[t] {
			continue
		}
		seen[t] = true
		missing = append(missing, t)
	}
	return NumbersReport{OK: len(missing) == 0, Ungrounded: missing}
}
