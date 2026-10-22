package provider

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// NewDirect 为受限判定创建独立、无代理/重定向/自动重试的连接；HTTP仅可显式本地评估。
// 构造不发请求，不启用业务功能；真实审批、来源与硬预算仍由上层控制器核验。
func NewDirect(cfg Config, allowLoopbackHTTP bool) (*OpenAICompat, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u == nil {
		return nil, fmt.Errorf("判定端点无效")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if !allowLoopbackHTTP || u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.Port() == "" {
			return nil, fmt.Errorf("判定端点必须为HTTPS或显式回环评估")
		}
	}
	tr := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: maxAttemptTimeout,
		ForceAttemptHTTP2: true, MaxIdleConns: 8, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	c, err := New(cfg, WithTransport(tr), WithMaxResponseBytes(1<<20))
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal([]any{"direct-single-call-v1", c.name, c.endpoint, c.model, c.timeout, c.maxBody, allowLoopbackHTTP, c.jsonFallback, c.reasoning})
	if err != nil {
		return nil, fmt.Errorf("判定端点配置无效")
	}
	h := sha256.Sum256(b)
	c.directProfile = hex.EncodeToString(h[:])
	return c, nil
}

// DirectProfileDigest 只标识端点/模型/时限/传输策略；不含密钥，也不冒充远端预算或质量证明。
func (c *OpenAICompat) DirectProfileDigest() string {
	if c == nil {
		return ""
	}
	return c.directProfile
}

// MatchesDirect 拒绝借一个固定端点身份覆盖成另一模型；审批与预算必须绑定实际请求模型。
func (c *OpenAICompat) MatchesDirect(profile, model string) bool {
	return c != nil && c.directProfile != "" && c.directProfile == profile && model != "" && c.model == model
}
