package provider

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDirectProfileAndTransport(t *testing.T) {
	cfg := Config{Name: "test-only", BaseURL: "https://model.example.test/v1", Model: "test-only-model", APIKey: "fake-only-value", TimeoutMS: 1000}
	a, err := NewDirect(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg.APIKey = "fake-only-rotated"
	b, err := NewDirect(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.DirectProfileDigest() == "" || a.DirectProfileDigest() != b.DirectProfileDigest() {
		t.Fatal("凭据轮换改变非秘密身份摘要")
	}
	if !a.MatchesDirect(a.DirectProfileDigest(), cfg.Model) || a.MatchesDirect(a.DirectProfileDigest(), "other-model") || a.MatchesDirect("bad", cfg.Model) {
		t.Fatal("直连模型/身份匹配错误")
	}
	tr, ok := a.client.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || tr.TLSClientConfig == nil || a.maxBody != 16<<10 || a.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("直连边界不完整")
	}
	for _, change := range []func(*Config){func(c *Config) { c.BaseURL = "https://other.example.test/v1" }, func(c *Config) { c.Model = "other-model" }, func(c *Config) { c.TimeoutMS = 2000 }} {
		x := cfg
		change(&x)
		other, err := NewDirect(x, false)
		if err != nil || other.DirectProfileDigest() == a.DirectProfileDigest() {
			t.Fatal("端点/模型/时限变化不失效", err)
		}
	}
	for _, url := range []string{"", ":bad", "http://model.example.test/v1", "http://127.0.0.1/v1", "http://localhost:1234/v1", "http://192.168.1.2:1234/v1", "https://u:p@model.example.test/v1", "https://model.example.test/v1?secret=test-only"} {
		x := cfg
		x.BaseURL = url
		if _, err := NewDirect(x, true); err == nil {
			t.Fatal("不安全端点通过", url)
		}
	}
	for _, url := range []string{"http://127.0.0.1:1234/v1", "http://[::1]:1234/v1"} {
		x := cfg
		x.BaseURL = url
		if _, err := NewDirect(x, false); err == nil {
			t.Fatal("隐式本地评估通过")
		}
		if _, err := NewDirect(x, true); err != nil {
			t.Fatal("显式回环评估未通过", err)
		}
	}
	standard, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var absent *OpenAICompat
	if standard.DirectProfileDigest() != "" || absent.DirectProfileDigest() != "" {
		t.Fatal("普通/空适配器自称直连")
	}
	if standard.MatchesDirect("", cfg.Model) || absent.MatchesDirect("", cfg.Model) {
		t.Fatal("普通/空客户端匹配直连")
	}
	binary, err := json.Marshal(a)
	if err != nil || string(binary) != "{}" {
		t.Fatal("私有配置被序列化", err)
	}
}
