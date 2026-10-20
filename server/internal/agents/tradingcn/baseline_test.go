package tradingcn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBaselineIsFixedCommunityContract(t *testing.T) {
	v := Baseline()
	if v.Scope != "adapter_baseline" || v.Edition != "community" || v.Commit != "51060a7682cf51810b4bc3a704829438d18cd8dc" || v.Contract != "v2_async_analysis" || v.SoftwareActivationRequired || !v.ServiceAuthenticationRequired || v.RemoteVersionVerified {
		t.Fatal("社区版协议参照错误或冒充远端已验收")
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 7 {
		t.Fatal("版本输出不再是闭集元数据")
	}
	// 返回值被修改不能改变其他调用者或既有持久配置摘要。
	v.Edition = "pro"
	if Baseline().Edition != "community" {
		t.Fatal("调用方改动污染协议参照")
	}
}

func TestEvaluationLicenseReferenceIsNotActivationCredential(t *testing.T) {
	cfg := testConfig()
	cfg.LicenseRef = "test-only:personal-evaluation:LICENSE@51060a7"
	c, err := New(cfg)
	if err != nil {
		t.Fatal("用途引用不应被要求提供商业激活码", err)
	}
	defer c.Close()
	calls := 0
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "Bearer "+cfg.Bearer {
			t.Fatal("服务凭据被许可引用替代")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), cfg.LicenseRef) || strings.Contains(r.URL.String(), cfg.LicenseRef) || strings.Contains(r.Header.Get("Authorization"), cfg.LicenseRef) {
			t.Fatal("许可引用被发送到上游")
		}
		return response(200, `{"success":true,"data":{"task_id":"test-community-1","status":"pending"}}`), nil
	})
	if _, err := c.Submit(context.Background(), "600001"); err != nil || calls != 1 {
		t.Fatal("社区版测试提交契约失败", err)
	}
	cfg.Bearer = ""
	if _, err := New(cfg); err == nil {
		t.Fatal("无需激活不等于服务可匿名调用")
	}
}
