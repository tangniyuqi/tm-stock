package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

const (
	ExtractionStageVersion = "closed-extraction-stage-v1"
	maxStageSnapshotBytes  = 1 << 20
	maxStageSentences      = 10000
	maxStageThemes         = 128
	maxStageAliases        = 32
)

// ExtractionStage 是一份快照的固定、纯校验策略，不是事实已核验或来源当前有权使用的证明。
// 构造前由可信调用方读取当前有效授权和任务范围内词汇；这里不做数据库/网络调用。
type ExtractionStage struct {
	inputDigest string
	policy      task.StagePolicy
}

func (s ExtractionStage) InputDigest() string      { return s.inputDigest }
func (s ExtractionStage) Policy() task.StagePolicy { return s.policy }

type extractionStageBinding struct {
	SnapshotID, SourceID                            int64
	Category, URL, BodyHash, LicenseTag, LicenseRef string
	LicenseExpires, FetchedAt                       string
	PromptVersion, System, Prompt, Schema           string
	MaxOutputTokens                                 int
}

func NewExtractionStage(snapshot sources.Snapshot, input guard.Vocabulary) (ExtractionStage, error) {
	if snapshot.ID <= 0 || len(snapshot.Body) > maxStageSnapshotBytes || len(snapshot.Sentences) > maxStageSentences || snapshot.Validate() != nil {
		return ExtractionStage{}, points.ErrInvalid
	}
	u, err := url.Parse(snapshot.URL)
	if err != nil || u == nil {
		return ExtractionStage{}, points.ErrInvalid
	}
	canonical, err := sources.ValidateURL(u.Hostname(), snapshot.URL)
	if err != nil || canonical != snapshot.URL {
		return ExtractionStage{}, points.ErrInvalid
	}
	vocab, err := stageVocabularyCopy(input, snapshot.SourceCategory)
	if err != nil {
		return ExtractionStage{}, err
	}
	prompt, err := extractionPrompt(snapshot, vocab)
	if err != nil {
		return ExtractionStage{}, points.ErrInvalid
	}
	binding := extractionStageBinding{
		SnapshotID: snapshot.ID, SourceID: snapshot.SourceID, Category: snapshot.SourceCategory, URL: snapshot.URL,
		BodyHash: snapshot.Hash, LicenseTag: snapshot.LicenseTag, LicenseRef: snapshot.LicenseRef,
		FetchedAt: snapshot.FetchedAt.UTC().Format(time.RFC3339Nano), PromptVersion: PromptVersion,
		System: extractionSystem, Prompt: prompt, Schema: extractionSchema, MaxOutputTokens: maxOutputTokens,
	}
	if snapshot.LicenseExpires != nil {
		binding.LicenseExpires = snapshot.LicenseExpires.UTC().Format(time.RFC3339Nano)
	}
	rawInput, err := json.Marshal(binding)
	if err != nil {
		return ExtractionStage{}, points.ErrInvalid
	}
	digest := task.StageDigest(rawInput)
	rawPolicy, err := json.Marshal(struct {
		Version, Guard, Input string
		Vocabulary            guard.Vocabulary
	}{ExtractionStageVersion, guard.PolicyDigest(), digest, vocab})
	if err != nil {
		return ExtractionStage{}, points.ErrInvalid
	}
	// 正文字符串不可变；句子索引由代码重建，不持有调用方的切片/词汇 map。
	snap := guard.NewSnapshot(snapshot.Body)
	policy := task.StagePolicy{Digest: task.StageDigest(rawPolicy), Check: func(ctx context.Context, in task.StageValidationInput) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if in.Stage != "extract" || in.InputDigest != digest {
			return task.ErrStagePolicy
		}
		payload := []byte(in.Payload)
		if task.CheckStageJSON(payload) != nil {
			return task.ErrStageRejected
		}
		report, err := guard.ValidateExtraction(payload, snap, vocab)
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil || report == nil || len(report.Accepted) == 0 || len(report.Rejected) != 0 {
			return task.ErrStageRejected
		}
		type claimKey struct{ Predicate, Entity, Theme, IDs string }
		seen := map[claimKey]bool{}
		for _, accepted := range report.Accepted {
			c := accepted.Claim
			key := claimKey{c.Predicate, guard.Skeleton(c.Entity), c.Theme, fmt.Sprint(c.SentenceIDs)}
			if accepted.Trimmed || seen[key] {
				return task.ErrStageRejected
			}
			seen[key] = true
		}
		return ctx.Err()
	}}
	return ExtractionStage{inputDigest: digest, policy: policy}, nil
}

func stageVocabularyCopy(input guard.Vocabulary, category string) (guard.Vocabulary, error) {
	out := guard.Vocabulary{Predicates: map[string]bool{}, Themes: map[string][]string{}}
	if len(input.Themes) == 0 || len(input.Themes) > maxStageThemes {
		return out, points.ErrInvalid
	}
	for predicate := range input.Predicates {
		if predicate != "DISCLOSES_BUSINESS" && predicate != "LISTED_IN_CATALOG" {
			return out, points.ErrInvalid
		}
	}
	predicate := "DISCLOSES_BUSINESS"
	if category == sources.CategoryOfficialList {
		predicate = "LISTED_IN_CATALOG"
	}
	if !input.Predicates[predicate] {
		return out, points.ErrInvalid
	}
	out.Predicates[predicate] = true
	for theme, aliases := range input.Themes {
		if !validStageTerm(theme) || len(aliases) > maxStageAliases {
			return out, points.ErrInvalid
		}
		copyAliases := make([]string, 0, len(aliases))
		seen := map[string]bool{theme: true}
		for _, alias := range aliases {
			if !validStageTerm(alias) {
				return out, points.ErrInvalid
			}
			if !seen[alias] {
				seen[alias] = true
				copyAliases = append(copyAliases, alias)
			}
		}
		sort.Strings(copyAliases)
		out.Themes[theme] = copyAliases
	}
	return out, nil
}

func validStageTerm(s string) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && n >= 2 && n <= 64 && guard.Normalize(s) == s && !strings.ContainsAny(s, "\r\n") && !guard.Blocked(s)
}
