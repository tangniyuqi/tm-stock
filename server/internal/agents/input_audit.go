package agents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"slices"
	"strings"
)

const InputAuditVersion = "tm-stock-agent-inputs-v1"
const MaxInputAuditBytes = 2 << 20

var inputNodes = []string{"bear_researcher_v2", "bull_researcher_v2", "fundamentals_analyst_v2", "index_analyst_v2", "market_analyst_v2", "neutral_analyst_v2", "news_analyst_v2", "research_manager_v2", "risk_manager_v2", "risky_analyst_v2", "safe_analyst_v2", "sector_analyst_v2", "social_analyst_v2", "trader_v2"}

// 固定上游研究经理声明的唯一可选委派，不替代十四个必需节点。
const delegatedInputNode = "chip_distribution_analyst_v2"

// 完整请求只在原隔离结果内保存；不得序列化到会员DTO、日志或下一模型的线索中。
type InputAudit struct {
	Schema         string      `json:"schema"`
	Scope          string      `json:"scope"`
	UpstreamCommit string      `json:"upstream_commit"`
	Provider       string      `json:"provider"`
	FixtureJSON    string      `json:"fixture_json"`
	FixtureDigest  string      `json:"fixture_digest"`
	Calls          []InputCall `json:"calls"`
}
type InputCall struct {
	Seq           int    `json:"seq"`
	CallID        string `json:"call_id"`
	Node          string `json:"node"`
	Model         string `json:"model"`
	ResponseModel string `json:"response_model"`
	RequestJSON   string `json:"request_json"`
	RequestDigest string `json:"request_digest"`
	PromptDigest  string `json:"prompt_digest"`
	State         string `json:"state"`
	Status        int    `json:"status"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
}

func (InputAudit) String() string     { return "上游完整输入存证（正文已隐藏）" }
func (a InputAudit) GoString() string { return a.String() }
func (InputCall) String() string      { return "上游调用输入（正文已隐藏）" }
func (a InputCall) GoString() string  { return a.String() }

func inputDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func inputShape(raw json.RawMessage, typ reflect.Type) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	if typ.Kind() == reflect.Struct {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil || len(obj) != typ.NumField() {
			return false
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			v, ok := obj[f.Tag.Get("json")]
			if !ok || !inputShape(v, f.Type) {
				return false
			}
		}
	} else if typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.Struct {
		var rows []json.RawMessage
		if json.Unmarshal(raw, &rows) != nil {
			return false
		}
		for _, r := range rows {
			if !inputShape(r, typ.Elem()) {
				return false
			}
		}
	}
	return true
}
func inputUnique(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 32 {
			return false
		}
		t, e := d.Token()
		if e != nil {
			return false
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				s, ok := key.(string)
				if e != nil || !ok || seen[s] {
					return false
				}
				seen[s] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, e := d.Token()
			return e == nil && end == json.Delim('}')
		case json.Delim('['):
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, e := d.Token()
			return e == nil && end == json.Delim(']')
		default:
			return true
		}
	}
	if !value(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
func inputJSON(raw []byte, out any) error {
	if !inputUnique(raw) || !inputShape(raw, reflect.TypeOf(out).Elem()) {
		return ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(out) != nil || dec.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	return nil
}

// 仅声明这次隔离试跑的合成资料，不能冒充项目真实候选资料或第三方许可。
type InputFixture struct {
	Scope   string      `json:"scope"`
	Subject string      `json:"subject"`
	Notice  string      `json:"notice"`
	Facts   []InputFact `json:"facts"`
}
type InputFact struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func (a *InputAudit) Fixture() (InputFixture, error) {
	var f InputFixture
	if a == nil || len(a.FixtureJSON) > 8192 || inputJSON([]byte(a.FixtureJSON), &f) != nil || inputDigest([]byte(a.FixtureJSON)) != a.FixtureDigest || f.Scope != "synthetic_local_fixture" || f.Subject != "TMEVAL（集成测试主体，非真实证券）" || f.Notice != "仅用于接线测试，无真实行情、财报、新闻或投资用途。不得编造缺失数字。" || len(f.Facts) != 1 || f.Facts[0].ID != "fixture-1" || f.Facts[0].Text != "测试设备用于处理样例数据。" {
		return InputFixture{}, ErrInvalid
	}
	return f, nil
}

func ParseInputAudit(raw []byte) (*InputAudit, error) {
	var a InputAudit
	if len(raw) == 0 || len(raw) > MaxInputAuditBytes || inputJSON(raw, &a) != nil || a.Schema != InputAuditVersion || a.Scope != "synthetic-fixture-only" || a.UpstreamCommit != "51060a7682cf51810b4bc3a704829438d18cd8dc" || (a.Provider != "deepseek" && a.Provider != "qwen-token-plan") || len(a.Calls) < len(inputNodes) || len(a.Calls) > 128 {
		return nil, ErrInvalid
	}
	if _, err := a.Fixture(); err != nil {
		return nil, err
	}
	nodes := map[string]bool{}
	ids := map[string]bool{}
	for i, c := range a.Calls {
		var request struct {
			Model     string          `json:"model"`
			Messages  json.RawMessage `json:"messages"`
			MaxTokens int             `json:"max_tokens"`
		}
		model := "deepseek-flash"
		if a.Provider == "qwen-token-plan" {
			model = "qwen3.7-plus"
		}
		if c.Seq != i+1 || len(c.CallID) != 32 || strings.Trim(c.CallID, "0123456789abcdef") != "" || ids[c.CallID] || (!slices.Contains(inputNodes, c.Node) && c.Node != delegatedInputNode) || c.Model != model || c.ResponseModel != c.Model || c.State != "confirmed" || c.Status != 200 || c.InputTokens < 0 || c.OutputTokens < 0 || len(c.RequestJSON) > 512<<10 || !inputUnique([]byte(c.RequestJSON)) || inputDigest([]byte(c.RequestJSON)) != c.RequestDigest || json.Unmarshal([]byte(c.RequestJSON), &request) != nil || request.Model != c.Model || request.MaxTokens < 1 || request.MaxTokens > 2048 || len(request.Messages) == 0 || request.Messages[0] != '[' || inputDigest(request.Messages) != c.PromptDigest {
			return nil, ErrInvalid
		}
		var messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(request.Messages, &messages) != nil || len(messages) < 1 || len(messages) > 128 {
			return nil, ErrInvalid
		}
		for _, m := range messages {
			var text string
			if !slices.Contains([]string{"system", "user", "assistant", "tool"}, m.Role) || (bytes.Equal(m.Content, []byte("null")) && m.Role != "assistant") || (!bytes.Equal(m.Content, []byte("null")) && json.Unmarshal(m.Content, &text) != nil) {
				return nil, ErrInvalid
			}
		}
		ids[c.CallID] = true
		nodes[c.Node] = true
	}
	for _, required := range inputNodes {
		if !nodes[required] {
			return nil, ErrInvalid
		}
	}
	return &a, nil
}

// 新版结果把输入审计和研究线索分开；原结果摘要仍绑定整份隔离正文。
func InputAuditPayload(result Result) (json.RawMessage, *InputAudit, error) {
	var obj map[string]json.RawMessage
	if !result.Valid() || !inputUnique(result.Payload) || json.Unmarshal(result.Payload, &obj) != nil {
		return nil, nil, ErrInvalid
	}
	raw, found := obj["input_audit"]
	for key := range obj {
		if (strings.EqualFold(key, "input_audit") || strings.EqualFold(key, "inputAudit")) && key != "input_audit" {
			return nil, nil, ErrInvalid
		}
	}
	if !found {
		return bytes.Clone(result.Payload), nil, nil
	}
	var envelope struct {
		ID      string          `json:"analysis_id"`
		Reports json.RawMessage `json:"reports"`
		Scope   string          `json:"evaluation_scope"`
		Audit   json.RawMessage `json:"input_audit"`
	}
	// RawMessage不逐字节按slice检查；外层键仍是精确闭集。
	if len(obj) != 4 || obj["analysis_id"] == nil || obj["reports"] == nil || obj["evaluation_scope"] == nil || json.Unmarshal(result.Payload, &envelope) != nil || envelope.ID != result.RemoteID || envelope.Scope != "synthetic_fixture_only" {
		return nil, nil, ErrInvalid
	}
	a, err := ParseInputAudit(raw)
	if err != nil {
		return nil, nil, err
	}
	var reports map[string]string
	if json.Unmarshal(envelope.Reports, &reports) != nil || len(reports) == 0 {
		return nil, nil, ErrInvalid
	}
	projected, err := json.Marshal(struct {
		ID      string          `json:"analysis_id"`
		Reports json.RawMessage `json:"reports"`
		Scope   string          `json:"evaluation_scope"`
	}{envelope.ID, envelope.Reports, envelope.Scope})
	if err != nil {
		return nil, nil, ErrInvalid
	}
	return projected, a, nil
}
