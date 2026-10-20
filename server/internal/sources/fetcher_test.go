package sources

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type staticResolver []net.IPAddr

func (r staticResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return append([]net.IPAddr(nil), r...), nil
}

type resolverError struct{}

func (resolverError) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return nil, errors.New("test DNS failure")
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func TestIsPublicAddress(t *testing.T) {
	tests := []struct {
		ip       string
		isPublic bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"10.0.0.1", false},
		{"127.0.0.1", false},
		{"100.64.0.1", false},
		{"169.254.1.1", false},
		{"192.0.2.1", false},
		{"192.88.99.1", false},
		{"198.18.0.1", false},
		{"198.51.100.1", false},
		{"203.0.113.1", false},
		{"240.0.0.1", false},
		{"::", false},
		{"::1", false},
		{"::ffff:127.0.0.1", false},
		{"64:ff9b::c0a8:1", false},
		{"fc00::1", false},
		{"fe80::1", false},
		{"2001:db8::1", false},
		{"2002::1", false},
	}
	for _, test := range tests {
		t.Run(test.ip, func(t *testing.T) {
			got := isPublicAddress(net.ParseIP(test.ip))
			if got != test.isPublic {
				t.Fatalf("isPublicAddress(%s) = %v, want %v", test.ip, got, test.isPublic)
			}
		})
	}
}

func TestSafeDialContextPinsHostAndRejectsUnsafeDNSAnswers(t *testing.T) {
	t.Run("only dials validated public resolution", func(t *testing.T) {
		called := false
		f := &Fetcher{
			resolver: staticResolver{{IP: net.ParseIP("8.8.8.8")}},
			dialer: dialerFunc(func(_ context.Context, network, address string) (net.Conn, error) {
				called = true
				if network != "tcp" || address != "8.8.8.8:443" {
					t.Fatalf("拨号目标未固定到已校验 IP: %s %s", network, address)
				}
				client, peer := net.Pipe()
				_ = peer.Close()
				return client, nil
			}),
		}
		conn, err := f.safeDialContext("example.test")(context.Background(), "tcp", "example.test:443")
		if err != nil {
			t.Fatalf("安全公网地址被拒绝: %v", err)
		}
		_ = conn.Close()
		if !called {
			t.Fatal("未调用拨号器")
		}
	})

	for _, test := range []struct {
		name string
		ips  []net.IPAddr
	}{
		{"回环地址", []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}},
		{"私网地址", []net.IPAddr{{IP: net.ParseIP("10.1.2.3")}}},
		{"链路本地地址", []net.IPAddr{{IP: net.ParseIP("169.254.1.1")}}},
		{"公网与私网混合 DNS", []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("192.168.1.1")}}},
		{"IPv6 带 zone", []net.IPAddr{{IP: net.ParseIP("2606:4700:4700::1111"), Zone: "eth0"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			f := &Fetcher{
				resolver: staticResolver(test.ips),
				dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
					called = true
					return nil, nil
				}),
			}
			if _, err := f.safeDialContext("example.test")(context.Background(), "tcp", "example.test:443"); !errors.Is(err, ErrUnsafeURL) {
				t.Fatalf("不安全 DNS 应返回 ErrUnsafeURL，实际为 %v", err)
			}
			if called {
				t.Fatal("DNS 含不安全地址时不应尝试拨号")
			}
		})
	}
}

func TestSafeDialContextRejectsHostAndPortConfusion(t *testing.T) {
	called := false
	f := &Fetcher{
		resolver: staticResolver{{IP: net.ParseIP("8.8.8.8")}},
		dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			called = true
			return nil, nil
		}),
	}
	for _, address := range []string{"other.test:443", "example.test:80", "example.test"} {
		if _, err := f.safeDialContext("example.test")(context.Background(), "tcp", address); !errors.Is(err, ErrUnsafeURL) {
			t.Errorf("address %q 应拒绝，得到 %v", address, err)
		}
	}
	if called {
		t.Fatal("不匹配的主机或端口不应拨号")
	}
}

func TestSafeDialContextRejectsDNSFailure(t *testing.T) {
	f := &Fetcher{
		resolver: resolverError{},
		dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("DNS 失败后不应拨号")
			return nil, nil
		}),
	}
	if _, err := f.safeDialContext("example.test")(context.Background(), "tcp", "example.test:443"); !errors.Is(err, ErrFetchFailed) {
		t.Fatalf("DNS 失败应归类为 ErrFetchFailed，实际为 %v", err)
	}
}

func TestRedirectPolicy(t *testing.T) {
	makeRequest := func(raw string) *http.Request {
		request, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		return request
	}
	policy := redirectPolicy("example.test")
	if err := policy(makeRequest("https://example.test/next"), []*http.Request{makeRequest("https://example.test/start")}); err != nil {
		t.Fatalf("同域名跳转应允许: %v", err)
	}
	if err := policy(makeRequest("https://evil.test/next"), []*http.Request{makeRequest("https://example.test/start")}); !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("跨域跳转应拒绝: %v", err)
	}
	via := make([]*http.Request, maxRedirects+1)
	for i := range via {
		via[i] = makeRequest("https://example.test/start")
	}
	if err := policy(makeRequest("https://example.test/too-many"), via); !errors.Is(err, ErrUnsafeURL) {
		t.Fatalf("超过重定向上限应拒绝: %v", err)
	}
}

func TestReadResponseBoundsStatusMediaAndBody(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		contentLen  int64
		limit       int64
		wantErr     error
	}{
		{"成功正文", http.StatusOK, "text/plain; charset=utf-8", "年度报告正文。", -1, 100, nil},
		{"拒绝部分响应", http.StatusPartialContent, "text/plain", "不完整", -1, 100, ErrFetchFailed},
		{"拒绝 PDF", http.StatusOK, "application/pdf", "%PDF", -1, 100, ErrUnsupportedMedia},
		{"Content-Length 预检", http.StatusOK, "text/plain", "正文", 101, 100, ErrBodyTooLarge},
		{"未知长度仍受限", http.StatusOK, "text/plain", strings.Repeat("甲", 101), -1, 100, ErrBodyTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &http.Response{
				StatusCode:    test.status,
				Header:        http.Header{"Content-Type": []string{test.contentType}},
				Body:          io.NopCloser(strings.NewReader(test.body)),
				ContentLength: test.contentLen,
			}
			body, mediaType, err := readResponse(response, test.limit)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("错误 = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil || string(body) != test.body || mediaType != "text/plain; charset=utf-8" {
				t.Fatalf("结果 body=%q media=%q err=%v", body, mediaType, err)
			}
		})
	}
}
