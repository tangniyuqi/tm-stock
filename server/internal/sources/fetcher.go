package sources

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxBodyBytes = 2 << 20
	defaultFetchTimeout = 15 * time.Second
	maxRedirects        = 3
)

var ErrFetchFailed = errors.New("读取资料页面失败")

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type contextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Fetcher 仅访问已登记来源的精确域名，并在实际拨号时重新解析与校验 DNS。
type Fetcher struct {
	resolver ipResolver
	dialer   contextDialer
	maxBytes int64
	timeout  time.Duration

	mu       sync.Mutex
	perHost  map[int64]chan struct{}
	nextRead map[int64]time.Time
}

// NewFetcher 使用保守的正文、跳转和总时限上限。
func NewFetcher() *Fetcher {
	return &Fetcher{
		resolver: net.DefaultResolver,
		dialer: &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		maxBytes: defaultMaxBodyBytes,
		timeout:  defaultFetchTimeout,
		perHost:  make(map[int64]chan struct{}),
		nextRead: make(map[int64]time.Time),
	}
}

// Fetch 下载一个已授权来源下的 HTML 或纯文本页面。
func (f *Fetcher) Fetch(ctx context.Context, source Source, rawURL string) ([]byte, string, error) {
	if f == nil || source.ID <= 0 {
		return nil, "", ErrUnauthorized
	}
	normalized, err := source.NormalizeRegistration()
	if err != nil || !normalized.HasValidAuthorization(time.Now().UTC()) {
		return nil, "", ErrUnauthorized
	}
	source = normalized
	canonicalURL, err := ValidateURL(source.Domain, rawURL)
	if err != nil {
		return nil, "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()
	if err := f.waitForSource(requestCtx, source.ID, source.MinInterval); err != nil {
		return nil, "", err
	}

	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           f.safeDialContext(source.Domain),
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:     true,
		DisableCompression:    false,
		MaxConnsPerHost:       1,
		MaxIdleConnsPerHost:   1,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       10 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: redirectPolicy(source.Domain),
	}
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, canonicalURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("构造资料请求失败: %w", ErrFetchFailed)
	}
	request.Header.Set("Accept", "text/html, text/plain;q=0.9")
	request.Header.Set("User-Agent", "tm-stock-source-collector/1.0")
	response, err := client.Do(request)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, "", fmt.Errorf("资料请求未完成: %w", err)
	}
	defer response.Body.Close()
	return readResponse(response, f.maxBytes)
}

func readResponse(response *http.Response, maxBytes int64) ([]byte, string, error) {
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("资料服务返回 HTTP %d: %w", response.StatusCode, ErrFetchFailed)
	}
	mediaType, mediaParams, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (strings.ToLower(mediaType) != MediaHTML && strings.ToLower(mediaType) != MediaPlain) || !validCharset(mediaParams["charset"]) {
		return nil, "", ErrUnsupportedMedia
	}
	if response.ContentLength > maxBytes {
		return nil, "", ErrBodyTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("读取资料响应失败: %w", ErrFetchFailed)
	}
	if int64(len(body)) > maxBytes {
		return nil, "", ErrBodyTooLarge
	}
	return body, mime.FormatMediaType(mediaType, mediaParams), nil
}

func redirectPolicy(allowedDomain string) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return fmt.Errorf("超过最大跳转次数: %w", ErrUnsafeURL)
		}
		if _, err := ValidateURL(allowedDomain, request.URL.String()); err != nil {
			return ErrUnsafeURL
		}
		return nil
	}
}

func (f *Fetcher) waitForSource(ctx context.Context, sourceID int64, interval time.Duration) error {
	f.mu.Lock()
	slot, ok := f.perHost[sourceID]
	if !ok {
		slot = make(chan struct{}, 1)
		slot <- struct{}{}
		f.perHost[sourceID] = slot
	}
	f.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-slot:
	}
	defer func() { slot <- struct{}{} }()

	if interval <= 0 {
		interval = time.Second
	}
	f.mu.Lock()
	next := f.nextRead[sourceID]
	f.mu.Unlock()
	if wait := time.Until(next); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	f.mu.Lock()
	f.nextRead[sourceID] = time.Now().Add(interval)
	f.mu.Unlock()
	return nil
}

func (f *Fetcher) safeDialContext(allowedDomain string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if f == nil || f.resolver == nil || f.dialer == nil {
			return nil, ErrUnsafeURL
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil || !strings.EqualFold(strings.TrimSuffix(host, "."), allowedDomain) || port != "443" {
			return nil, ErrUnsafeURL
		}
		ips, err := f.resolver.LookupIPAddr(ctx, allowedDomain)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("解析资料域名失败: %w", ErrFetchFailed)
		}
		for _, address := range ips {
			if address.Zone != "" || !isPublicAddress(address.IP) {
				return nil, ErrUnsafeURL
			}
		}
		var lastErr error
		for _, address := range ips {
			conn, dialErr := f.dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, fmt.Errorf("连接资料域名失败: %w", lastErr)
	}
}

func isPublicAddress(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		blocked := []string{
			"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
			"192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		}
		for _, cidr := range blocked {
			_, network, _ := net.ParseCIDR(cidr)
			if network.Contains(v4) {
				return false
			}
		}
		return true
	}
	_, globalUnicast, _ := net.ParseCIDR("2000::/3")
	if !globalUnicast.Contains(ip) {
		return false
	}
	blocked := []string{"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"}
	for _, cidr := range blocked {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return false
		}
	}
	return true
}
