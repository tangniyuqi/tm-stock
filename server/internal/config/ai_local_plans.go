package config

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

// 本地计划只批准合成资料验收，不是生产定价、模型备案或资料许可证明。
// 不保存DSN、密钥、原文或完整快照；正文须从同一隔离库重新取得并核验。
type AILocalPlans struct {
	Version     int           `json:"version"`
	InstanceID  string        `json:"instance_id"`
	Revision    string        `json:"revision"`
	Scope       string        `json:"scope"`
	IssuedAtMS  int64         `json:"issued_at_ms"`
	ExpiresAtMS int64         `json:"expires_at_ms"`
	Members     []int64       `json:"members"`
	Plans       []AILocalPlan `json:"plans"`
}

type AILocalPlan struct {
	AnalysisType       int               `json:"analysis_type"`
	SubjectID          int64             `json:"subject_id"`
	SubjectName        string            `json:"subject_name"`
	ActorID            int64             `json:"actor_id"`
	Symbol             string            `json:"symbol"`
	AgentProfileDigest string            `json:"agent_profile_digest"`
	AgentCalls         int64             `json:"agent_calls"`
	AgentTokens        int64             `json:"agent_tokens"`
	AgentPages         int64             `json:"agent_pages"`
	ModelName          string            `json:"model_name"`
	ModelProfileDigest string            `json:"model_profile_digest"`
	FilingNo           string            `json:"filing_no"`
	ModelVersion       string            `json:"model_version"`
	MaxInputTokens     int64             `json:"max_input_tokens"`
	SelectionMaxTokens int               `json:"selection_max_tokens"`
	CheckMaxTokens     int               `json:"check_max_tokens"`
	PolicyRef          string            `json:"policy_ref"`
	MinQuotes          int               `json:"min_quotes"`
	AllowedDomains     []string          `json:"allowed_domains"`
	ApprovalRef        string            `json:"approval_ref"`
	QualityRef         string            `json:"quality_ref"`
	HardBudgetRef      string            `json:"hard_budget_ref"`
	PriceRef           string            `json:"price_ref"`
	SourceRef          string            `json:"source_ref"`
	PriceVersion       string            `json:"price_version"`
	PricePoints        int64             `json:"price_points"`
	Budget             task.Budget       `json:"budget"`
	Materials          []AILocalMaterial `json:"materials"`
}

type AILocalMaterial struct {
	SnapshotID int64  `json:"snapshot_id"`
	Digest     string `json:"digest"`
	LicenseRef string `json:"license_ref"`
}

func (AILocalPlans) String() string     { return "本地合成资料计划（内容已隐藏）" }
func (p AILocalPlans) GoString() string { return p.String() }

// AILocalPlanSource 每次重新读取并比较规范摘要；不允许运行中静默更换计划。
type AILocalPlanSource struct {
	path    string
	runtime AILocalRuntime
	digest  string
}

func NewAILocalPlanSource(path string, runtime *AILocalRuntime) (*AILocalPlanSource, error) {
	if runtime == nil || runtime.validate() != nil {
		return nil, invalidLocalRuntime()
	}
	s := &AILocalPlanSource{path: path, runtime: *runtime}
	p, err := s.read()
	if err != nil {
		return nil, err
	}
	s.digest, err = localPlansDigest(p)
	if err != nil {
		return nil, invalidLocalRuntime()
	}
	return s, nil
}

func localPlansDigest(p *AILocalPlans) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return task.StageDigest(b), nil
}

func (s *AILocalPlanSource) Current(ctx context.Context) (*AILocalPlans, error) {
	if s == nil || ctx == nil {
		return nil, invalidLocalRuntime()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := s.read()
	if err != nil {
		return nil, err
	}
	digest, err := localPlansDigest(p)
	if err != nil || digest != s.digest {
		return nil, invalidLocalRuntime()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *AILocalPlanSource) read() (*AILocalPlans, error) {
	b, err := readAILocalFile(s.path, 128<<10)
	if err != nil || task.CheckStageJSON(b) != nil || !exactLocalPlanJSON(b, reflect.TypeOf(AILocalPlans{})) {
		return nil, invalidLocalRuntime()
	}
	var p AILocalPlans
	if json.Unmarshal(b, &p) != nil || p.validate(&s.runtime, time.Now().UnixMilli()) != nil {
		return nil, invalidLocalRuntime()
	}
	return &p, nil
}

// 所有层级精确键且全部必填，包括显式零值；不接受大小写别名、null或未知字段。
func exactLocalPlanJSON(raw json.RawMessage, typ reflect.Type) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	switch typ.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || len(fields) != typ.NumField() {
			return false
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			value, ok := fields[name]
			if !ok || !exactLocalPlanJSON(value, field.Type) {
				return false
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil {
			return false
		}
		for _, value := range values {
			if !exactLocalPlanJSON(value, typ.Elem()) {
				return false
			}
		}
	}
	return true
}

var localRef = regexp.MustCompile(`^local-eval:[A-Za-z0-9_.:-]{1,110}$`)
var localRevision = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)

func (p *AILocalPlans) validate(runtime *AILocalRuntime, now int64) error {
	if runtime.validate() != nil || p.Version != 1 || p.InstanceID != runtime.InstanceID || !localRevision.MatchString(p.Revision) || p.Scope != "synthetic-local-only" || p.IssuedAtMS <= 0 || p.IssuedAtMS > now || p.ExpiresAtMS <= now || p.ExpiresAtMS <= p.IssuedAtMS || p.ExpiresAtMS-p.IssuedAtMS > 86400000 || len(p.Members) < 1 || len(p.Members) > 16 || len(p.Plans) < 1 || len(p.Plans) > 16 {
		return invalidLocalRuntime()
	}
	for i, id := range p.Members {
		if id <= 0 || id > 9007199254740991 || i > 0 && id <= p.Members[i-1] {
			return invalidLocalRuntime()
		}
	}
	seen := map[[2]int64]bool{}
	for _, v := range p.Plans {
		key := [2]int64{int64(v.AnalysisType), v.SubjectID}
		if seen[key] || v.AnalysisType != 1 && v.AnalysisType != 2 || v.SubjectID <= 0 || v.ActorID <= 0 || v.Budget.Validate() != nil || v.SelectionMaxTokens <= 0 || v.CheckMaxTokens <= 0 || len(v.Materials) < 1 || len(v.Materials) > 32 || len(v.AllowedDomains) < 1 || len(v.AllowedDomains) > 32 {
			return invalidLocalRuntime()
		}
		// 与原模型输入上限同量级，避免不可信整数先在容量算式中溢出。
		if v.MaxInputTokens < 1 || v.MaxInputTokens > 1<<22 || v.SelectionMaxTokens > 1<<22 || v.CheckMaxTokens > 1<<22 {
			return invalidLocalRuntime()
		}
		seen[key] = true
		for _, ref := range []string{v.PolicyRef, v.ApprovalRef, v.QualityRef, v.HardBudgetRef, v.PriceRef, v.SourceRef} {
			if !localRef.MatchString(ref) {
				return invalidLocalRuntime()
			}
		}
		if !strings.HasPrefix(v.SubjectName, "仅测试:") || !strings.HasPrefix(v.FilingNo, "仅测试:") || !strings.HasPrefix(v.ModelVersion, "local-eval:") {
			return invalidLocalRuntime()
		}
		for i, m := range v.Materials {
			if m.SnapshotID <= 0 || i > 0 && m.SnapshotID <= v.Materials[i-1].SnapshotID || !localRef.MatchString(m.LicenseRef) {
				return invalidLocalRuntime()
			}
		}
		if !slices.IsSorted(v.AllowedDomains) {
			return invalidLocalRuntime()
		}
		for i, d := range v.AllowedDomains {
			if !strings.HasSuffix(d, ".example.test") || i > 0 && d == v.AllowedDomains[i-1] {
				return invalidLocalRuntime()
			}
		}
	}
	return nil
}
