package pipeline

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const stageFact = `{"claims":[{"predicate":"DISCLOSES_BUSINESS","entity":"星河科技","theme":"算力","sentence_ids":[1]}]}`

//go:embed testdata/stage_quote_rejections.jsonl
var stageQuoteRejections string

func stageVocabulary() guard.Vocabulary {
	return guard.Vocabulary{Predicates: map[string]bool{"DISCLOSES_BUSINESS": true, "LISTED_IN_CATALOG": true}, Themes: map[string][]string{"算力": {"智算", "算力"}}}
}
func extractionStageFixture(t *testing.T) (ExtractionStage, sources.Snapshot, guard.Vocabulary) {
	t.Helper()
	snap := testSnapshot(t, sources.CategoryAnnualReport, "星河科技主营算力服务。另一段是明确的测试资料。")
	vocab := stageVocabulary()
	stage, err := NewExtractionStage(snap, vocab)
	if err != nil {
		t.Fatal(err)
	}
	return stage, snap, vocab
}
func checkStage(stage ExtractionStage, payload string) error {
	return stage.Policy().Check(context.Background(), task.StageValidationInput{Stage: "extract", InputDigest: stage.InputDigest(), Payload: payload})
}

func TestExtractionStageClosedValidation(t *testing.T) {
	s, _, _ := extractionStageFixture(t)
	if len(s.InputDigest()) != 64 || !s.Policy().Valid() || checkStage(s, stageFact) != nil {
		t.Fatal("固定测试资料的正例未通过")
	}
	for name, raw := range map[string]string{
		"空数组":    `{"claims":[]}`,
		"未知字段":   strings.TrimSuffix(stageFact, "}") + `,"recommendation":"测试禁止字段"}`,
		"未知条目字段": strings.Replace(stageFact, `"predicate":`, `"reason":"测试多余字段","predicate":`, 1),
		"大小写字段":  strings.Replace(stageFact, `"entity":`, `"Entity":`, 1),
		"重复键":    strings.Replace(stageFact, `"entity":`, `"entity":"另一测试公司","entity":`, 1),
		"重复大小写键": strings.Replace(stageFact, `"entity":`, `"ENTITY":"另一测试公司","entity":`, 1),
		"输出原文引文": strings.Replace(stageFact, `"predicate":`, `"quote":"模型编造测试文字","predicate":`, 1),
		"编号越界":   strings.Replace(stageFact, `[1]`, `[9999]`, 1),
		"编号不升序":  strings.Replace(stageFact, `[1]`, `[2,1]`, 1),
		"无共同出处":  strings.Replace(stageFact, `[1]`, `[2]`, 1),
		"未知实体":   strings.Replace(stageFact, "星河科技", "未披露测试公司", 1),
		"未知题材":   strings.Replace(stageFact, "算力", "测试目录外题材", 1),
		"非法谓词":   strings.Replace(stageFact, "DISCLOSES_BUSINESS", "RECOMMEND", 1),
		"目录谓词错位": strings.Replace(stageFact, "DISCLOSES_BUSINESS", "LISTED_IN_CATALOG", 1),
		"重复条目":   strings.Replace(stageFact, `}]}`, `},{"predicate":"DISCLOSES_BUSINESS","entity":"星河科技","theme":"算力","sentence_ids":[1]}]}`, 1),
		"尾随JSON": stageFact + `{}`,
		"原始研究报告": `{"reports":{"research":"仅用于测试的研究原文"},"verified":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkStage(s, raw); !errors.Is(err, task.ErrStageRejected) {
				t.Fatal("不安全/畸形响应未整体拒绝", err)
			}
		})
	}
	for _, field := range []string{"predicate", "entity", "theme", "sentence_ids"} {
		// 缺字段不能由 encoding/json 的零值补成可用候选。
		raw := strings.Replace(stageFact, `"`+field+`":`, `"missing_`+field+`":`, 1)
		if err := checkStage(s, raw); !errors.Is(err, task.ErrStageRejected) {
			t.Fatal(field, err)
		}
	}
}

func TestExtractionStageNoPartialOrTrimmedAcceptance(t *testing.T) {
	// 带违规措辞的反例只在测试 JSONL 中，只有测试二进制嵌入，不随产品代码发布。
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(stageQuoteRejections), "\n") {
		var fixture struct{ Name, Body, Purpose string }
		if err := json.Unmarshal([]byte(line), &fixture); err != nil || fixture.Purpose != "仅用于拒绝测试，非真实公司披露" || seen[fixture.Name] {
			t.Fatal("拒绝测试夹具损坏或重复", err)
		}
		seen[fixture.Name] = true
		snap := testSnapshot(t, sources.CategoryAnnouncement, fixture.Body)
		if fixture.Name == "需要节选" {
			report, err := guard.ValidateExtraction([]byte(stageFact), guard.NewSnapshot(snap.Body), stageVocabulary())
			if err != nil || len(report.Accepted) != 1 || !report.Accepted[0].Trimmed || len(report.Rejected) != 0 {
				t.Fatal("节选负例不是有效的部分通过基线", err)
			}
		}
		s, err := NewExtractionStage(snap, stageVocabulary())
		if err != nil {
			t.Fatal(err)
		}
		if err := checkStage(s, stageFact); !errors.Is(err, task.ErrStageRejected) {
			t.Fatal("仅用于反例的测试资料未拒绝", err)
		}
	}
	for _, name := range []string{"否定关系", "需要节选", "规则命中", "分句无共现"} {
		if !seen[name] {
			t.Fatal("拒绝夹具漏项", name)
		}
	}
	s, _, _ := extractionStageFixture(t)
	raw := strings.Replace(stageFact, `}]}`, `},{"predicate":"DISCLOSES_BUSINESS","entity":"未披露测试公司","theme":"算力","sentence_ids":[1]}]}`, 1)
	if err := checkStage(s, raw); !errors.Is(err, task.ErrStageRejected) {
		t.Fatal("部分失败误推进整个原始响应", err)
	}
	catalog := testSnapshot(t, sources.CategoryOfficialList, "星河科技列入算力服务目录。")
	s, err := NewExtractionStage(catalog, stageVocabulary())
	if err != nil || checkStage(s, strings.Replace(stageFact, "DISCLOSES_BUSINESS", "LISTED_IN_CATALOG", 1)) != nil {
		t.Fatal("目录事实正例未通过", err)
	}
	if err := checkStage(s, stageFact); !errors.Is(err, task.ErrStageRejected) {
		t.Fatal("来源类别/谓词不一致", err)
	}
}

func TestExtractionStageDigestAndImmutableInputs(t *testing.T) {
	s, snap, vocab := extractionStageFixture(t)
	again, err := NewExtractionStage(snap, vocab)
	if err != nil || again.InputDigest() != s.InputDigest() || again.Policy().Digest != s.Policy().Digest {
		t.Fatal("固定输入摘要漂移", err)
	}
	vocab.Themes["算力"] = []string{"算力", "智算", "智算"}
	again, err = NewExtractionStage(snap, vocab)
	if err != nil || again.InputDigest() != s.InputDigest() || again.Policy().Digest != s.Policy().Digest {
		t.Fatal("等价别名顺序/重复造成策略漂移", err)
	}
	vocab.Themes["算力"] = []string{"算力", "数据中心"}
	changed, err := NewExtractionStage(snap, vocab)
	if err != nil || changed.InputDigest() != s.InputDigest() || changed.Policy().Digest == s.Policy().Digest {
		t.Fatal("别名改变未隔离旧检查点", err)
	}
	for _, change := range []func(*sources.Snapshot){
		func(s *sources.Snapshot) { s.ID++ },
		func(s *sources.Snapshot) { s.SourceID++ },
		func(s *sources.Snapshot) { s.URL += "?edition=2" },
		func(s *sources.Snapshot) { s.LicenseRef += "-other" },
		func(s *sources.Snapshot) { s.FetchedAt = s.FetchedAt.Add(time.Millisecond) },
	} {
		other := snap
		change(&other)
		changed, err := NewExtractionStage(other, stageVocabulary())
		if err != nil || changed.InputDigest() == s.InputDigest() || changed.Policy().Digest == s.Policy().Digest {
			t.Fatal("快照身份/授权副本改变未绑定摘要", err)
		}
	}
	vocab.Predicates["DISCLOSES_BUSINESS"] = false
	vocab.Themes["算力"][0] = "调用方改变测试锚词"
	delete(vocab.Themes, "算力")
	snap.Body = "调用方改变测试正文"
	snap.Sentences[0].Text = "调用方改变测试句子"
	if err := checkStage(s, stageFact); err != nil {
		t.Fatal("调用方修改别名/句子污染固定策略", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := checkStage(s, stageFact); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestExtractionStageInvalidInputsAndContext(t *testing.T) {
	s, snap, vocab := extractionStageFixture(t)
	for _, change := range []func(*sources.Snapshot){
		func(s *sources.Snapshot) { s.ID = 0 },
		func(s *sources.Snapshot) { s.URL = "://" },
		func(s *sources.Snapshot) { s.Hash = "wrong" },
		func(s *sources.Snapshot) { s.URL = "http://exchange.example/docs/1" },
		func(s *sources.Snapshot) { s.URL += "?token=fake-stage-only" },
		func(s *sources.Snapshot) { s.Sentences = nil },
		func(s *sources.Snapshot) { s.Sentences = make([]sources.Sentence, maxStageSentences+1) },
		func(s *sources.Snapshot) { s.LicenseRef = "" },
		func(s *sources.Snapshot) { s.Body = strings.Repeat("测", maxStageSnapshotBytes) },
	} {
		other := snap
		change(&other)
		if _, err := NewExtractionStage(other, vocab); !errors.Is(err, points.ErrInvalid) {
			t.Fatal("无效快照未拒绝", err)
		}
	}
	for _, v := range []guard.Vocabulary{
		{},
		{Predicates: map[string]bool{"RECOMMEND": true}, Themes: vocab.Themes},
		{Predicates: map[string]bool{"LISTED_IN_CATALOG": true}, Themes: vocab.Themes},
		{Predicates: vocab.Predicates, Themes: map[string][]string{" 算力": {"算力"}}},
		{Predicates: vocab.Predicates, Themes: map[string][]string{"算力": {"单"}}},
		{Predicates: vocab.Predicates, Themes: map[string][]string{"算力": {strings.Repeat("测", 65)}}},
		{Predicates: vocab.Predicates, Themes: map[string][]string{"算力": make([]string, maxStageAliases+1)}},
	} {
		if _, err := NewExtractionStage(snap, v); !errors.Is(err, points.ErrInvalid) {
			t.Fatal("无效词汇未拒绝", err)
		}
	}
	for _, in := range []task.StageValidationInput{
		{Stage: "judge", InputDigest: s.InputDigest(), Payload: stageFact},
		{Stage: "extract", InputDigest: strings.Repeat("a", 64), Payload: stageFact},
	} {
		if err := s.Policy().Check(context.Background(), in); !errors.Is(err, task.ErrStagePolicy) {
			t.Fatal("输入绑定未生效", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Policy().Check(ctx, task.StageValidationInput{Stage: "extract", InputDigest: s.InputDigest(), Payload: stageFact}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if (ExtractionStage{}).Policy().Valid() {
		t.Fatal("零值策略可用")
	}
	large := stageVocabulary()
	large.Themes = map[string][]string{}
	for i := 0; i <= maxStageThemes; i++ {
		large.Themes[fmt.Sprintf("测试题材%03d", i)] = nil
	}
	if _, err := NewExtractionStage(snap, large); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("题材容量未限制", err)
	}
	if err := checkStage(s, `{"claims":"`+strings.Repeat("a", task.MaxStageResultBytes)+`"}`); !errors.Is(err, task.ErrStageRejected) {
		t.Fatal("超大结果未拒绝", err)
	}
}

func TestExtractionStageAuthorizationCopyAndTimezoneDigest(t *testing.T) {
	_, snap, vocab := extractionStageFixture(t)
	expires := snap.FetchedAt.Add(time.Hour)
	snap.LicenseExpires = &expires
	s, err := NewExtractionStage(snap, vocab)
	if err != nil {
		t.Fatal(err)
	}
	snap.FetchedAt = snap.FetchedAt.In(time.FixedZone("测试时区", 8*3600))
	expires = expires.In(time.FixedZone("测试时区", -5*3600))
	again, err := NewExtractionStage(snap, vocab)
	if err != nil || again.InputDigest() != s.InputDigest() || again.Policy().Digest != s.Policy().Digest {
		t.Fatal("等价 UTC 时点导致摘要漂移", err)
	}
	expires = expires.Add(time.Millisecond)
	changed, err := NewExtractionStage(snap, vocab)
	if err != nil || changed.InputDigest() == s.InputDigest() || changed.Policy().Digest == s.Policy().Digest {
		t.Fatal("授权到期副本改变未绑定摘要", err)
	}
	if err := checkStage(s, stageFact); err != nil {
		t.Fatal("调用方修改授权指针污染策略", err)
	}
}
