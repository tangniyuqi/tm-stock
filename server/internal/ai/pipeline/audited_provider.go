package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type AuditedProvider struct {
	base           provider.Provider
	store          RunStore
	model          string
	secret         string
	maxPromptBytes int
}

func NewAuditedProvider(base provider.Provider, store RunStore, model, secret string, maxPromptBytes int) (*AuditedProvider, error) {
	if base == nil || store == nil || model == "" || maxPromptBytes < 1 {
		return nil, ErrInvalidRun
	}
	return &AuditedProvider{base: base, store: store, model: model, secret: secret, maxPromptBytes: maxPromptBytes}, nil
}

// Generate 审计起点落库成功后才调用模型；完成审计失败时不把输出交给候选队列。
func (p *AuditedProvider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	id := RunID(ctx)
	if id <= 0 || req.Attempt < 1 || req.Attempt > 2 || (req.Model != "" && req.Model != p.model) || req.Format != provider.FormatJSONSchema || !req.Strict || req.MaxTokens < 1 || req.MaxTokens > 4096 {
		return provider.Response{}, ErrInvalidRun
	}
	if len(req.Prompt)+len(req.System) > p.maxPromptBytes {
		return provider.Response{}, &provider.Error{Kind: provider.ContextLen, Code: "l0_prompt_limit"}
	}
	req.Model = p.model
	redact := func(data []byte) []byte {
		data = provider.Redact(data)
		if p.secret != "" {
			data = bytes.ReplaceAll(data, []byte(p.secret), []byte("<已隐藏>"))
		}
		return data
	}
	loggedRequest := req
	loggedRequest.Prompt = string(redact([]byte(req.Prompt)))
	loggedRequest.System = string(redact([]byte(req.System)))
	raw, err := json.Marshal(loggedRequest)
	if err != nil {
		return provider.Response{}, ErrInvalidRun
	}
	if err := p.store.AppendCallEvent(ctx, CallEvent{RunID: id, Attempt: req.Attempt, Phase: "start", Payload: redact(raw)}); err != nil {
		return provider.Response{}, fmt.Errorf("调用前审计失败: %w", ErrAuditUnavailable)
	}
	start := time.Now()
	response, callErr := p.base.Generate(ctx, req)
	var payload struct {
		Model          string         `json:"model"`
		OK             bool           `json:"ok"`
		ErrorKind      string         `json:"error_kind,omitempty"`
		Usage          provider.Usage `json:"usage"`
		UsageEstimated bool           `json:"usage_estimated"`
		LatencyMS      int64          `json:"latency_ms"`
		RequestID      string         `json:"request_id,omitempty"`
		Raw            []byte         `json:"raw,omitempty"`
		Text           string         `json:"text,omitempty"`
	}
	payload.Model, payload.OK, payload.Usage, payload.UsageEstimated = response.Model, callErr == nil, response.Usage, response.UsageEstimated
	payload.LatencyMS, payload.RequestID, payload.Raw = time.Since(start).Milliseconds(), response.RequestID, redact(response.Raw)
	if len(response.Raw) == 0 {
		payload.Text = response.Text
	}
	if callErr != nil {
		kind, _ := provider.KindOf(callErr)
		payload.ErrorKind = kind.String()
		var pe *provider.Error
		if errors.As(callErr, &pe) {
			payload.Usage = pe.Usage
			payload.RequestID = pe.RequestID
			payload.Text = pe.Detail
		}
	}
	payload.Text = string(redact([]byte(payload.Text)))
	raw, err = json.Marshal(payload)
	if err != nil {
		return provider.Response{}, ErrAuditUnavailable
	}
	// 客户端取消后仍给审计最多 5 秒，不把供应商结果留成未追踪的成功。
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := p.store.AppendCallEvent(finishCtx, CallEvent{RunID: id, Attempt: req.Attempt, Phase: "finish", Payload: redact(raw)}); err != nil {
		return provider.Response{}, ErrAuditUnavailable
	}
	if callErr == nil && response.Model != p.model {
		return provider.Response{}, &provider.Error{Kind: provider.Malformed, Code: "unregistered_response_model"}
	}
	return response, callErr
}
