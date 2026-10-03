package guard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 夹具运行器：读取 eval/redteam（必须被拦）、eval/benign（必须放行）与本包 testdata（自有用例），
// 逐条断言。格式与口径见 eval/README.md。夹具含违规措辞，所以不放在 server/ 的 .go 或 .json 里。

var reportPath = flag.String("report", "", "把夹具评测汇总写到该 JSON 文件（可选；相对路径相对本包目录）")

const (
	evalRoot     = "../../../../eval"
	testdataRoot = "testdata"

	// 允许标记为 known_gap 的样本占全部夹具的最大比例：防止用它藏漏洞。
	maxKnownGapRatio = 0.10
)

// 数量下限（起点值，只增不减）。删用例会让测试失败，新增用例直接放行。
var (
	minRedteam = map[string]int{"check_text": 100, "screen_quote": 25, "validate_extraction": 35, "co_occur": 25, "numbers_grounded": 25}
	minBenign  = map[string]int{"check_text": 50, "screen_quote": 10, "validate_extraction": 10, "co_occur": 15, "numbers_grounded": 15}
)

type expectation struct {
	Blocked    *bool              `json:"blocked"`
	RuleAny    []string           `json:"rule_any"`
	Action     string             `json:"action"`
	Text       *string            `json:"text"`
	OK         *bool              `json:"ok"`
	Reason     string             `json:"reason"`
	Ungrounded []string           `json:"ungrounded"`
	Accepted   *[]int             `json:"accepted"`
	Rejected   *map[string]string `json:"rejected"`
	Trimmed    []int              `json:"trimmed"`
	Malformed  bool               `json:"malformed"`
}

type fixture struct {
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	Channel   string      `json:"channel"`
	Tags      []string    `json:"tags"`
	Note      string      `json:"note"`
	KnownGap  bool        `json:"known_gap"`
	GapReason string      `json:"gap_reason"`
	Text      string      `json:"text"`
	Quote     string      `json:"quote"`
	Display   string      `json:"display"`
	Body      string      `json:"body"`
	Raw       string      `json:"raw"`
	Names     []string    `json:"names"`
	Anchors   []string    `json:"anchors"`
	Allowed   []string    `json:"allowed"`
	Expect    expectation `json:"expect"`

	file string // 来源文件（不参与序列化）
	line int
}

var idPattern = regexp.MustCompile(`^B?O?(CT|SQ|VE|CO|NG)-\d{3,}$`)

var kindPrefix = map[string]string{"check_text": "CT", "screen_quote": "SQ", "validate_extraction": "VE", "co_occur": "CO", "numbers_grounded": "NG"}

// loadFixtureDir 读取目录下全部 .jsonl；目录不存在直接失败（不是跳过：跳过会让整套红队集悄悄失效）。
func loadFixtureDir(t testing.TB, dir string) []fixture {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("夹具目录不存在：%s（%v）。夹具缺失必须失败，不能跳过。", dir, err)
	}
	sort.Strings(files)
	var out []fixture
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 1<<20), 8<<20)
		for n := 1; sc.Scan(); n++ {
			line := sc.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				t.Fatalf("%s:%d 出现空行（JSONL 不允许空行）", f, n)
			}
			dec := json.NewDecoder(bytes.NewReader(line))
			dec.DisallowUnknownFields() // 拼错键名会让断言悄悄失效，必须当场失败
			var fx fixture
			if err := dec.Decode(&fx); err != nil {
				t.Fatalf("%s:%d 解析失败：%v", f, n, err)
			}
			if _, err := dec.Token(); err == nil {
				t.Fatalf("%s:%d 一行里有多个 JSON 值", f, n)
			}
			fx.file, fx.line = filepath.Base(f), n
			out = append(out, fx)
		}
		if err := sc.Err(); err != nil {
			t.Fatalf("%s 读取失败：%v", f, err)
		}
	}
	return out
}

func loadVocab(t testing.TB) Vocabulary {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(evalRoot, "vocab.json"))
	if err != nil {
		t.Fatalf("读不到 eval/vocab.json：%v", err)
	}
	var raw struct {
		Predicates []string            `json:"predicates"`
		Themes     map[string][]string `json:"themes"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("eval/vocab.json 解析失败：%v", err)
	}
	v := Vocabulary{Predicates: map[string]bool{}, Themes: raw.Themes}
	for _, p := range raw.Predicates {
		v.Predicates[p] = true
	}
	if len(v.Predicates) == 0 || len(v.Themes) == 0 {
		t.Fatal("eval/vocab.json 的谓词或题材为空")
	}
	return v
}

// validateFixture 检查夹具本身是否良构：字段齐全、断言键与 kind 匹配。返回错误描述；空表示良构。
func validateFixture(f fixture, allowKnownGap bool) string {
	where := fmt.Sprintf("%s:%d(%s)", f.file, f.line, f.ID)
	prefix, ok := kindPrefix[f.Kind]
	if !ok {
		return where + " kind 不合法：" + f.Kind
	}
	if m := idPattern.FindStringSubmatch(f.ID); m == nil || m[1] != prefix {
		return where + " id 格式或前缀与 kind 不符（应为 " + prefix + "-三位序号，对照集前加 B）"
	}
	switch f.Channel {
	case "model_output", "source_body", "quote":
	default:
		return where + " channel 不合法：" + f.Channel
	}
	if len(f.Tags) == 0 {
		return where + " tags 不能为空"
	}
	if f.KnownGap && (!allowKnownGap || strings.TrimSpace(f.GapReason) == "") {
		return where + " known_gap 只允许出现在红队集，且必须有 gap_reason"
	}
	if !f.KnownGap && f.GapReason != "" {
		return where + " 没有 known_gap 却有 gap_reason"
	}
	e := f.Expect
	switch f.Kind {
	case "check_text":
		if e.Blocked == nil {
			return where + " 缺少 expect.blocked"
		}
		for _, r := range e.RuleAny {
			if r != "word" && r != "pat" {
				return where + " rule_any 只能是 word 或 pat：" + r
			}
		}
		if !*e.Blocked && len(e.RuleAny) > 0 {
			return where + " blocked=false 时不应有 rule_any"
		}
	case "screen_quote":
		if f.Quote == "" {
			return where + " 缺少 quote"
		}
		switch e.Action {
		case "pass", "human":
			if e.Text != nil {
				return where + " 只有 trim 才有 expect.text"
			}
		case "trim":
			if e.Text == nil || *e.Text == "" {
				return where + " trim 必须给出 expect.text"
			}
		default:
			return where + " expect.action 不合法：" + e.Action
		}
	case "co_occur":
		if f.Quote == "" || e.OK == nil {
			return where + " 缺少 quote 或 expect.ok"
		}
		valid := map[string]bool{"": true, string(CoOK): true, string(CoUnusableAlias): true, string(CoNoEntity): true,
			string(CoNoAnchor): true, string(CoNotSameSentence): true, string(CoNegatedAnchor): true}
		if !valid[e.Reason] {
			return where + " expect.reason 不合法：" + e.Reason
		}
		if *e.OK != (e.Reason == "" || e.Reason == string(CoOK)) {
			return where + " expect.ok 与 expect.reason 自相矛盾"
		}
	case "numbers_grounded":
		if f.Display == "" || e.OK == nil {
			return where + " 缺少 display 或 expect.ok"
		}
		if *e.OK && len(e.Ungrounded) > 0 {
			return where + " ok=true 时不应有 ungrounded"
		}
	case "validate_extraction":
		if f.Body == "" {
			return where + " 缺少 body"
		}
		if e.Malformed {
			if e.Accepted != nil || e.Rejected != nil || e.Trimmed != nil {
				return where + " malformed=true 时不应再有 accepted/rejected/trimmed"
			}
		} else if e.Accepted == nil {
			return where + " 缺少 expect.accepted（或 malformed）"
		}
		if e.Rejected != nil {
			for k := range *e.Rejected {
				if _, err := strconv.Atoi(k); err != nil {
					return where + " rejected 的键必须是 claim 下标：" + k
				}
			}
		}
	}
	return ""
}

// runFixture 执行一条夹具，返回是否符合断言及说明。
func runFixture(f fixture, vocab Vocabulary) (ok bool, detail string) {
	e := f.Expect
	switch f.Kind {
	case "check_text":
		vs := CheckText(f.Text)
		blocked := len(vs) > 0
		if blocked != *e.Blocked {
			return false, fmt.Sprintf("blocked=%v，期望 %v；命中=%v", blocked, *e.Blocked, ruleList(vs))
		}
		if blocked && len(e.RuleAny) > 0 {
			for _, v := range vs {
				for _, want := range e.RuleAny {
					if v.Category() == want {
						return true, ""
					}
				}
			}
			return false, fmt.Sprintf("命中的规则大类不在 rule_any=%v 内：%v", e.RuleAny, ruleList(vs))
		}
	case "screen_quote":
		got := ScreenQuote(f.Quote)
		if got.Action.String() != e.Action {
			return false, fmt.Sprintf("action=%s，期望 %s（text=%q）", got.Action, e.Action, got.Text)
		}
		if e.Action == "trim" && got.Text != *e.Text {
			return false, fmt.Sprintf("trim 文本=%q，期望 %q", got.Text, *e.Text)
		}
	case "co_occur":
		got := CoOccur(f.Quote, f.Names, f.Anchors)
		if (got == CoOK) != *e.OK {
			return false, fmt.Sprintf("结果=%s，期望 ok=%v", got, *e.OK)
		}
		if e.Reason != "" && string(got) != e.Reason {
			return false, fmt.Sprintf("原因=%s，期望 %s", got, e.Reason)
		}
	case "numbers_grounded":
		got := NumbersGrounded(f.Display, f.Quote, f.Allowed)
		if got.OK != *e.OK {
			return false, fmt.Sprintf("ok=%v，期望 %v；无依据=%v", got.OK, *e.OK, got.Ungrounded)
		}
		if e.Ungrounded != nil && !reflect.DeepEqual(got.Ungrounded, e.Ungrounded) {
			return false, fmt.Sprintf("无依据=%v，期望 %v", got.Ungrounded, e.Ungrounded)
		}
	case "validate_extraction":
		snap := NewSnapshot(f.Body)
		rep, err := ValidateExtraction([]byte(f.Raw), snap, vocab)
		// 与断言无关的通用不变量：无论夹具期望什么，被接受的引文都必须是快照子串、无红线命中。
		if rep != nil {
			for _, a := range rep.Accepted {
				if msg := acceptedInvariants(snap, vocab, a); msg != "" {
					return false, fmt.Sprintf("被接受的 claim %d 违反通用不变量：%s", a.Index, msg)
				}
			}
		}
		if e.Malformed {
			if !errors.Is(err, ErrMalformed) {
				return false, fmt.Sprintf("期望整体畸形，实际 err=%v rep=%+v", err, rep)
			}
			return true, ""
		}
		if err != nil {
			return false, fmt.Sprintf("不应整体报错：%v", err)
		}
		gotAcc := []int{}
		var gotTrim []int
		for _, a := range rep.Accepted {
			gotAcc = append(gotAcc, a.Index)
			if a.Trimmed {
				gotTrim = append(gotTrim, a.Index)
			}
		}
		if !reflect.DeepEqual(gotAcc, *e.Accepted) {
			return false, fmt.Sprintf("accepted=%v，期望 %v；rejected=%v", gotAcc, *e.Accepted, rejectList(rep))
		}
		wantRej := map[string]string{}
		if e.Rejected != nil {
			wantRej = *e.Rejected
		}
		gotRej := map[string]string{}
		for _, r := range rep.Rejected {
			gotRej[strconv.Itoa(r.Index)] = string(r.Reason)
		}
		if !reflect.DeepEqual(gotRej, wantRej) {
			return false, fmt.Sprintf("rejected=%v，期望 %v", gotRej, wantRej)
		}
		if e.Trimmed != nil && !reflect.DeepEqual(gotTrim, e.Trimmed) && !(len(gotTrim) == 0 && len(e.Trimmed) == 0) {
			return false, fmt.Sprintf("trimmed=%v，期望 %v", gotTrim, e.Trimmed)
		}
	}
	return true, ""
}

func ruleList(vs []Violation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Rule)
	}
	return out
}

func rejectList(rep *Report) map[int]RejectReason {
	m := map[int]RejectReason{}
	for _, r := range rep.Rejected {
		m[r.Index] = r.Reason
	}
	return m
}

// dirSet 是一个夹具目录及其语义：allowGap 表示是否允许 known_gap。
type dirSet struct {
	name     string
	path     string
	allowGap bool
	floor    map[string]int
}

func fixtureDirs() []dirSet {
	return []dirSet{
		{"redteam", filepath.Join(evalRoot, "redteam"), true, minRedteam},
		{"benign", filepath.Join(evalRoot, "benign"), false, minBenign},
		{"testdata", testdataRoot, true, nil},
	}
}

// 汇总结果，供 TestEvalReport 使用。
type summaryRow struct {
	Dir, Kind                    string
	Total, Passed, Failed, Gaps  int
	GapsStillOpen, GapsClosed    int
	FailedIDs, GapIDs, ClosedIDs []string
}

func runDir(t *testing.T, d dirSet, vocab Vocabulary) []summaryRow {
	fixtures := loadFixtureDir(t, d.path)
	rows := map[string]*summaryRow{}
	row := func(kind string) *summaryRow {
		if rows[kind] == nil {
			rows[kind] = &summaryRow{Dir: d.name, Kind: kind}
		}
		return rows[kind]
	}
	for _, f := range fixtures {
		r := row(f.Kind)
		r.Total++
		ok, detail := runFixture(f, vocab)
		switch {
		case ok && f.KnownGap:
			r.GapsClosed++
			r.ClosedIDs = append(r.ClosedIDs, f.ID)
			r.Passed++
			t.Logf("[缺口已封闭] %s：可以去掉 known_gap 标记（原因：%s）", f.ID, f.GapReason)
		case ok:
			r.Passed++
		case f.KnownGap:
			r.Gaps++
			r.GapsStillOpen++
			r.GapIDs = append(r.GapIDs, f.ID)
			t.Logf("[已知缺口] %s：%s —— %s", f.ID, f.GapReason, detail)
		default:
			r.Failed++
			r.FailedIDs = append(r.FailedIDs, f.ID)
			t.Errorf("%s:%d %s [%s] 不符合断言：%s\n    note: %s", f.file, f.line, f.ID, f.Kind, detail, f.Note)
		}
	}
	var out []summaryRow
	for _, k := range []string{"check_text", "screen_quote", "validate_extraction", "co_occur", "numbers_grounded"} {
		if rows[k] != nil {
			out = append(out, *rows[k])
		}
	}
	return out
}

func TestFixturesWellFormed(t *testing.T) {
	seen := map[string]string{}
	totalAll, gaps := 0, 0
	for _, d := range fixtureDirs() {
		fixtures := loadFixtureDir(t, d.path)
		counts := map[string]int{}
		for _, f := range fixtures {
			if msg := validateFixture(f, d.allowGap); msg != "" {
				t.Error(msg)
			}
			if prev, dup := seen[f.ID]; dup {
				t.Errorf("id %s 重复：%s 与 %s:%d", f.ID, prev, f.file, f.line)
			}
			seen[f.ID] = fmt.Sprintf("%s:%d", f.file, f.line)
			counts[f.Kind]++
			if d.name != "testdata" {
				totalAll++
				if f.KnownGap {
					gaps++
				}
			}
		}
		for kind, floor := range d.floor {
			if counts[kind] < floor {
				t.Errorf("%s/%s 夹具只有 %d 条，低于下限 %d（夹具只增不减）", d.name, kind, counts[kind], floor)
			}
		}
	}
	if totalAll == 0 {
		t.Fatal("没有读到任何夹具")
	}
	if ratio := float64(gaps) / float64(totalAll); ratio > maxKnownGapRatio {
		t.Errorf("known_gap 占比 %.1f%% 超过上限 %.0f%%（%d/%d）", ratio*100, maxKnownGapRatio*100, gaps, totalAll)
	}
}

func TestRedTeam(t *testing.T)  { runDir(t, fixtureDirs()[0], loadVocab(t)) }
func TestBenign(t *testing.T)   { runDir(t, fixtureDirs()[1], loadVocab(t)) }
func TestOwnCases(t *testing.T) { runDir(t, fixtureDirs()[2], loadVocab(t)) }

// TestEveryPatternHasCoverage：每条句式规则在夹具里都必须至少有一条正例触发它，没有测试的规则不允许存在。
// 词表词的覆盖由 TestWordsResistInsertion 保证（它对每个词做了多种变形的正例）。
func TestEveryPatternHasCoverage(t *testing.T) {
	hit := map[string]bool{}
	for _, d := range fixtureDirs() {
		for _, f := range loadFixtureDir(t, d.path) {
			var texts []string
			switch f.Kind {
			case "check_text":
				texts = []string{f.Text}
			case "screen_quote":
				texts = []string{f.Quote}
			case "validate_extraction":
				texts = []string{f.Body}
			}
			for _, tx := range texts {
				for _, v := range CheckText(tx) {
					hit[v.Rule] = true
				}
			}
		}
	}
	var missing []string
	for _, n := range RuleNames() {
		if strings.HasPrefix(n, "pat:") && !hit[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		t.Errorf("以下句式规则没有任何夹具正例：%v（请在 testdata/own_cases.jsonl 或 eval/redteam 里补充）", missing)
	}
}

// TestEvalReport：指定 -report=路径 时，把各目录各 kind 的通过数、失败数、已知缺口写成 JSON，
// 供手动触发的评测作业上传。不指定时跳过（它不是断言，而是报告生成器）。
func TestEvalReport(t *testing.T) {
	if *reportPath == "" {
		t.Skip("未指定 -report，跳过报告生成")
	}
	vocab := loadVocab(t)
	var all []summaryRow
	failed := false
	for _, d := range fixtureDirs() {
		rows := runDir(t, d, vocab)
		for _, r := range rows {
			if r.Failed > 0 {
				failed = true
			}
		}
		all = append(all, rows...)
	}
	data, err := json.MarshalIndent(map[string]any{"rows": all, "rules": len(RuleNames())}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*reportPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("写报告失败：%v", err)
	}
	t.Logf("报告已写入 %s", *reportPath)
	if failed {
		t.Error("存在不符合断言的夹具，详见上方输出")
	}
}
