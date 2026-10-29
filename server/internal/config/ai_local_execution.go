package config

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// 仅指向操作者明确提供的本地协议服务；文件不包含服务认证凭据。
type AILocalExecution struct {
	Version         int                   `json:"version"`
	InstanceID      string                `json:"instance_id"`
	Revision        string                `json:"revision"`
	Scope           string                `json:"scope"`
	IssuedAtMS      int64                 `json:"issued_at_ms"`
	ExpiresAtMS     int64                 `json:"expires_at_ms"`
	Model           AILocalExecutionModel `json:"model"`
	Agent           AILocalExecutionAgent `json:"agent"`
	LeaseMS         int64                 `json:"lease_ms"`
	HeartbeatMS     int64                 `json:"heartbeat_ms"`
	WorkerTimeoutMS int64                 `json:"worker_timeout_ms"`
	AgentPollMS     int64                 `json:"agent_poll_ms"`
}
type AILocalExecutionModel struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	TimeoutMS int    `json:"timeout_ms"`
}
type AILocalExecutionAgent struct {
	BaseURL        string `json:"base_url"`
	ApprovalRef    string `json:"approval_ref"`
	LicenseRef     string `json:"license_ref"`
	QuickModel     string `json:"quick_model"`
	DeepModel      string `json:"deep_model"`
	TimeoutMS      int    `json:"timeout_ms"`
	MaxSubmissions int    `json:"max_submissions"`
}

func (AILocalExecution) String() string     { return "本地执行配置（端点与身份已隐藏）" }
func (c AILocalExecution) GoString() string { return c.String() }
func (c AILocalExecution) ModelConfig() provider.Config {
	return provider.Config{Name: c.Model.Name, BaseURL: c.Model.BaseURL, Model: c.Model.Model, TimeoutMS: c.Model.TimeoutMS}
}
func (c AILocalExecution) AgentConfig(bearer string) tradingcn.Config {
	return tradingcn.Config{Enabled: true, LocalEvaluation: true, BaseURL: c.Agent.BaseURL, ApprovedHost: "127.0.0.1", Bearer: bearer, ApprovalRef: c.Agent.ApprovalRef, LicenseRef: c.Agent.LicenseRef, QuickModel: c.Agent.QuickModel, DeepModel: c.Agent.DeepModel, Timeout: time.Duration(c.Agent.TimeoutMS) * time.Millisecond, MaxSubmissions: c.Agent.MaxSubmissions}
}
func (c AILocalExecution) RecoveryOptions() task.RecoveryOptions {
	return task.RecoveryOptions{Lease: time.Duration(c.LeaseMS) * time.Millisecond, Heartbeat: time.Duration(c.HeartbeatMS) * time.Millisecond, Timeout: time.Duration(c.WorkerTimeoutMS) * time.Millisecond}
}
func localExecutionURL(raw string, model bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1024 || port > 65535 || u.Host != "127.0.0.1:"+strconv.Itoa(port) {
		return false
	}
	return u.Path == "" || u.Path == "/" || model && u.Path == "/v1"
}

type AILocalExecutionSource struct {
	path    string
	runtime AILocalRuntime
	digest  string
}

func NewAILocalExecutionSource(path string, runtime *AILocalRuntime) (*AILocalExecutionSource, error) {
	if runtime == nil || runtime.validate() != nil {
		return nil, invalidLocalRuntime()
	}
	s := &AILocalExecutionSource{path: path, runtime: *runtime}
	c, err := s.read()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, invalidLocalRuntime()
	}
	s.digest = task.StageDigest(b)
	return s, nil
}
func (s *AILocalExecutionSource) read() (*AILocalExecution, error) {
	if s == nil {
		return nil, invalidLocalRuntime()
	}
	b, err := readAILocalFile(s.path, 8192)
	if err != nil || task.CheckStageJSON(b) != nil || !exactLocalPlanJSON(b, reflect.TypeOf(AILocalExecution{})) {
		return nil, invalidLocalRuntime()
	}
	var c AILocalExecution
	if json.Unmarshal(b, &c) != nil {
		return nil, invalidLocalRuntime()
	}
	now := time.Now().UnixMilli()
	if c.Version != 1 || c.InstanceID != s.runtime.InstanceID || c.Scope != "synthetic-loopback-only" || c.Revision == "" || len(c.Revision) > 64 || c.IssuedAtMS <= 0 || c.IssuedAtMS > now || c.ExpiresAtMS <= now || c.ExpiresAtMS-c.IssuedAtMS > 86400000 || !localExecutionURL(c.Model.BaseURL, true) || !localExecutionURL(c.Agent.BaseURL, false) || c.Model.TimeoutMS < 100 || c.Model.TimeoutMS > 30000 || c.Agent.TimeoutMS < 1000 || c.Agent.TimeoutMS > 30000 || c.LeaseMS < 60000 || c.LeaseMS > 600000 || c.WorkerTimeoutMS < 1000 || c.WorkerTimeoutMS > 1800000 || c.HeartbeatMS < 50 || c.HeartbeatMS > 60000 || c.AgentPollMS < 50 || c.AgentPollMS > 5000 || c.RecoveryOptions().Validate() != nil || !strings.HasPrefix(c.Agent.ApprovalRef, "local-eval:") || !strings.HasPrefix(c.Agent.LicenseRef, "local-eval:") {
		return nil, invalidLocalRuntime()
	}
	// 仅校验无密钥结构；该占位参数不用于构造实际Client或发出请求。
	if c.AgentConfig(strings.Repeat("x", 32)).Validate() != nil {
		return nil, invalidLocalRuntime()
	}
	if _, err := provider.NewDirect(c.ModelConfig(), true); err != nil {
		return nil, invalidLocalRuntime()
	}
	return &c, nil
}
func (s *AILocalExecutionSource) Current(ctx context.Context) (*AILocalExecution, error) {
	if ctx == nil {
		return nil, invalidLocalRuntime()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, err := s.read()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(c)
	if err != nil || task.StageDigest(b) != s.digest {
		return nil, invalidLocalRuntime()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c, nil
}
