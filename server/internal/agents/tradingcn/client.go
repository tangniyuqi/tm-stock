// Package tradingcn 适配固定版本的 TradingAgents-CN V2 异步 API，不适配 Chat Completions。
package tradingcn

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

const UpstreamCommit = "51060a7682cf51810b4bc3a704829438d18cd8dc"

var hostLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func validHost(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !hostLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// Config 只允许服务器批准的固定地址与模型，不能由会员请求指定。
type Config struct {
	Enabled         bool
	LocalEvaluation bool
	BaseURL         string
	ApprovedHost    string
	Bearer          string // 服务登录凭据，不是社区版软件激活码。
	ApprovalRef     string
	LicenseRef      string // 适用许可/用途的追溯引用，不是需要向作者购买的序列号。
	QuickModel      string
	DeepModel       string
	Timeout         time.Duration
	MaxSubmissions  int
	// SelectedAnalysts 可选：默认 news+fundamentals；缩减流程用于本地稳定性验收。
	SelectedAnalysts []string
	// AnalysisDate 可选 YYYY-MM-DD；空值由上游用“今天”。测试环境日期可能领先真实数据，需显式指定历史交易日。
	AnalysisDate string
}

func (c Config) Validate() error {
	if !c.Enabled {
		return agents.ErrDisabled
	}
	u, err := url.Parse(c.BaseURL)
	validAddress := false
	if err == nil {
		validAddress = u.Scheme == "https" && validHost(u.Hostname()) && net.ParseIP(u.Hostname()) == nil && (u.Host == c.ApprovedHost || u.Host == c.ApprovedHost+":443") && u.Hostname() == c.ApprovedHost
		if c.LocalEvaluation {
			ip := net.ParseIP(u.Hostname())
			port, portErr := strconv.Atoi(u.Port())
			validAddress = u.Scheme == "http" && ip != nil && ip.IsLoopback() && u.Hostname() == c.ApprovedHost && portErr == nil && port >= 1 && port <= 65535
		}
	}
	if !validAddress || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("Agent 地址须为精确批准域名的 HTTPS 根地址，不允许凭据、路径或跳转")
	}
	if !validText(c.ApprovalRef, 512) || !validText(c.LicenseRef, 512) || !validText(c.QuickModel, 128) || !validText(c.DeepModel, 128) || c.Timeout < time.Second || c.Timeout > 30*time.Second || c.MaxSubmissions < 1 || c.MaxSubmissions > 1000 || len(c.Bearer) < 32 || len(c.Bearer) > 8192 {
		return fmt.Errorf("Agent 缺少批准/许可引用、固定模型、有效时限、提交限额或服务器凭据")
	}
	for _, ch := range c.Bearer {
		if ch < 33 || ch > 126 {
			return fmt.Errorf("Agent 服务器凭据格式无效")
		}
	}
	seen := map[string]bool{}
	for _, analyst := range c.SelectedAnalysts {
		if (analyst != "news" && analyst != "fundamentals") || seen[analyst] {
			return fmt.Errorf("Agent 分析师须为不重复的已批准资料分析师")
		}
		seen[analyst] = true
	}
	if c.AnalysisDate != "" {
		date, err := time.Parse("2006-01-02", c.AnalysisDate)
		if err != nil || date.Year() < 1900 || date.Format("2006-01-02") != c.AnalysisDate {
			return fmt.Errorf("Agent 分析日期须为规范日历日期")
		}
	}
	return nil
}

func validText(v string, max int) bool {
	if v == "" || len(v) > max || !utf8.ValidString(v) || strings.TrimSpace(v) != v {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func (c Config) Profile() agents.Profile {
	// v2 绑定实际分析师及日期策略；旧摘要不得自动换配置重提，令牌轮换仍不改变身份。
	body, _ := json.Marshal([]any{"tradingcn-execution-v2", UpstreamCommit, c.LocalEvaluation, strings.TrimSuffix(c.BaseURL, "/"), c.ApprovedHost, c.ApprovalRef, c.LicenseRef, c.QuickModel, c.DeepModel, c.MaxSubmissions, c.analysts(), c.AnalysisDate})
	sum := sha256.Sum256(body)
	return agents.Profile{Digest: hex.EncodeToString(sum[:]), MaxSubmissions: c.MaxSubmissions}
}

func (c Config) analysts() []string {
	if len(c.SelectedAnalysts) == 0 {
		return []string{"news", "fundamentals"}
	}
	return slices.Clone(c.SelectedAnalysts)
}

type Client struct {
	cfg  Config
	http *http.Client
}

// New 不读取代理、Cookie 或登录状态，不跟随重定向，不自动刷新令牌。
func New(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.SelectedAnalysts = cfg.analysts()
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DisableCompression = true
	return &Client{cfg: cfg, http: &http.Client{Transport: tr, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Close() { c.http.CloseIdleConnections() }

// Error 仅有固定错误码。任何已进入传输层的提交失败都不能据此推断远端未执行。
type Error struct {
	Code       string
	HTTPStatus int
	Cause      error
}

func (e *Error) Error() string {
	return "TradingAgents-CN 调用失败：" + e.Code + "（HTTP " + strconv.Itoa(e.HTTPStatus) + "）"
}
func (e *Error) Unwrap() error { return e.Cause }

type envelope struct {
	Success *bool           `json:"success"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) call(ctx context.Context, method, path string, payload any, needsData bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, &Error{Code: "context_done", Cause: err}
	}
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, &Error{Code: "invalid_input"}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(c.cfg.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Code: "invalid_request"}
	}
	// 禁止 net/http 对 POST 在连接故障时自动重放。也不发送暗示上游支持幂等的请求头。
	req.GetBody = nil
	req.Header.Set("Authorization", "Bearer "+c.cfg.Bearer)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// 不保存 *url.Error 或驱动原文，避免地址、凭据进入错误链。
		var cause error
		if ctx.Err() != nil {
			cause = ctx.Err()
		}
		return nil, &Error{Code: "transport_uncertain", Cause: cause}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &Error{Code: "http_rejected", HTTPStatus: resp.StatusCode}
	}
	media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, &Error{Code: "invalid_content_type", HTTPStatus: resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, agents.MaxResultBytes+1))
	if err != nil || len(raw) > agents.MaxResultBytes || !utf8.Valid(raw) || validateJSON(raw) != nil {
		return nil, &Error{Code: "invalid_response", HTTPStatus: resp.StatusCode}
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Success == nil || !*env.Success || (needsData && !object(env.Data)) {
		return nil, &Error{Code: "invalid_envelope", HTTPStatus: resp.StatusCode}
	}
	return env.Data, nil
}

func object(raw []byte) bool { return len(bytes.TrimSpace(raw)) > 1 && bytes.TrimSpace(raw)[0] == '{' }

func (c *Client) Submit(ctx context.Context, symbol string) (agents.Acceptance, error) {
	if !(agents.Request{RequestID: "validate_input", Symbol: symbol}).Valid() {
		return agents.Acceptance{}, agents.ErrInvalid
	}
	// 不开放自由提问、工作流覆盖、用户上下文或任意分析师。即使关掉可选情绪/风险，仍不宣称上游输出合规。
	params := struct {
		Engine    string   `json:"engine"`
		Market    string   `json:"market_type"`
		Depth     string   `json:"research_depth"`
		Analysts  []string `json:"selected_analysts"`
		Sentiment bool     `json:"include_sentiment"`
		Risk      bool     `json:"include_risk"`
		Language  string   `json:"language"`
		Quick     string   `json:"quick_analysis_model"`
		Deep      string   `json:"deep_analysis_model"`
		Trace     bool     `json:"enable_trace"`
		Date      string   `json:"analysis_date,omitempty"`
	}{"v2", "A股", "快速", c.cfg.SelectedAnalysts, false, false, "zh-CN", c.cfg.QuickModel, c.cfg.DeepModel, false, c.cfg.AnalysisDate}
	if len(params.Analysts) == 0 {
		params.Analysts = []string{"news", "fundamentals"}
	}
	data, err := c.call(ctx, http.MethodPost, "/api/analysis/single", struct {
		Symbol     string `json:"symbol"`
		Parameters any    `json:"parameters"`
	}{symbol, params}, true)
	if err != nil {
		return agents.Acceptance{}, err
	}
	var v struct {
		ID    string `json:"task_id"`
		State string `json:"status"`
	}
	if json.Unmarshal(data, &v) != nil || !agents.ValidRemoteID(v.ID) || (v.State != "pending" && v.State != "queued" && v.State != "running" && v.State != "processing") {
		return agents.Acceptance{}, &Error{Code: "invalid_acceptance"}
	}
	return agents.Acceptance{RemoteID: v.ID}, nil
}

func (c *Client) Status(ctx context.Context, id string) (agents.Observation, error) {
	if !agents.ValidRemoteID(id) {
		return agents.Observation{}, agents.ErrInvalid
	}
	data, err := c.call(ctx, http.MethodGet, "/api/analysis/tasks/"+id+"/status", nil, true)
	if err != nil {
		return agents.Observation{}, err
	}
	var v struct {
		ID    string `json:"task_id"`
		State string `json:"status"`
	}
	if json.Unmarshal(data, &v) != nil || v.ID != id {
		return agents.Observation{}, &Error{Code: "identity_mismatch"}
	}
	states := map[string]string{"pending": agents.Accepted, "queued": agents.Accepted, "running": agents.Running, "processing": agents.Running, "suspended": agents.Suspended, "completed": agents.Completed, "failed": agents.Failed, "cancelled": agents.Cancelled}
	state, ok := states[v.State]
	if !ok {
		return agents.Observation{}, &Error{Code: "unknown_state"}
	}
	return agents.Observation{RemoteID: id, State: state}, nil
}

func (c *Client) Result(ctx context.Context, id string) (agents.Result, error) {
	if !agents.ValidRemoteID(id) {
		return agents.Result{}, agents.ErrInvalid
	}
	data, err := c.call(ctx, http.MethodGet, "/api/analysis/tasks/"+id+"/result", nil, true)
	if err != nil {
		return agents.Result{}, err
	}
	var v struct {
		ID      string                     `json:"analysis_id"`
		Reports map[string]json.RawMessage `json:"reports"`
	}
	// 上游 analysis_id 是“代码_日期_时间”，不是任务 UUID；结果身份由请求 URL 与任务 ID 绑定。
	if json.Unmarshal(data, &v) != nil || v.ID == "" || len(v.Reports) == 0 {
		return agents.Result{}, &Error{Code: "invalid_result"}
	}
	// 逐字段替换已知服务器凭据；不能直接替换 JSON 字节，否则引号可能破坏 JSON。
	var decoded any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if d.Decode(&decoded) != nil {
		return agents.Result{}, &Error{Code: "invalid_result"}
	}
	redactJSON(&decoded, c.cfg.Bearer)
	safe, err := json.Marshal(decoded)
	if err != nil || len(safe) > agents.MaxResultBytes {
		return agents.Result{}, &Error{Code: "invalid_result"}
	}
	return agents.Result{RemoteID: id, Payload: safe}, nil
}

func (c *Client) Cancel(ctx context.Context, id string) error {
	if !agents.ValidRemoteID(id) {
		return agents.ErrInvalid
	}
	_, err := c.call(ctx, http.MethodPost, "/api/analysis/tasks/"+id+"/cancel", nil, false)
	return err // 仅表示取消请求被接受；必须另查状态，不等于工具/模型已经停止或退费。
}

func redactJSON(v *any, secret string) {
	switch x := (*v).(type) {
	case string:
		*v = strings.ReplaceAll(x, secret, "[已隐藏]")
	case []any:
		for i := range x {
			redactJSON(&x[i], secret)
		}
	case map[string]any:
		clean := make(map[string]any, len(x))
		for k, child := range x {
			redactJSON(&child, secret)
			clean[strings.ReplaceAll(k, secret, "[已隐藏]")] = child
		}
		*v = clean
	}
}

// validateJSON 拒绝重复键、多个 JSON 值、无效 UTF-8 与过深嵌套，不让后写覆盖改变任务身份。
func validateJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return agents.ErrInvalid
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		open, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch open {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return agents.ErrInvalid
				}
				seen[key] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return agents.ErrInvalid
		}
		_, err = d.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return agents.ErrInvalid
	}
	return nil
}
