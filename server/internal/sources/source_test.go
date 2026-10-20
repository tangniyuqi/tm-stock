package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func authorizedTestSource(now time.Time) Source {
	expires := now.Add(time.Hour).UTC().Truncate(time.Millisecond)
	return Source{
		ID:             7,
		Domain:         "EXAMPLE.TEST.",
		Category:       CategoryAnnualReport,
		LicenseTag:     "测试授权范围",
		LicenseRef:     "test-fixture:license-7",
		LicenseExpires: &expires,
		MinInterval:    time.Second,
		Enabled:        true,
	}
}

func TestBuildSnapshotNormalizesAndIndexesReversibleSentences(t *testing.T) {
	now := time.Now().UTC().Add(30 * time.Second)
	source := authorizedTestSource(now)
	body := []byte(`<html><head><style>.hidden{display:none}</style><script>token = "not evidence"</script></head><body><!-- not evidence --><p>Ａ公司披露营业收入为 12.5 亿元。</p><p>下一段来自公告原文&nbsp;。</p></body></html>`)
	snapshot, err := BuildSnapshot(source, "https://example.test/reports/1#page=2", "text/html; charset=utf-8", body, now, 4096)
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.URL != "https://example.test/reports/1" {
		t.Fatalf("规范化链接不符合预期: %q", snapshot.URL)
	}
	if strings.Contains(snapshot.Body, "not evidence") || strings.Contains(snapshot.Body, "Ａ") || strings.Contains(snapshot.Body, "\u00a0") {
		t.Fatalf("快照正文未正确清理或规范化: %q", snapshot.Body)
	}
	if snapshot.Body != "A公司披露营业收入为 12.5 亿元。\n下一段来自公告原文 。" {
		t.Fatalf("规范化正文不匹配: %q", snapshot.Body)
	}
	if len(snapshot.Sentences) != 2 {
		t.Fatalf("句子数 = %d，want 2: %#v", len(snapshot.Sentences), snapshot.Sentences)
	}
	for _, sentence := range snapshot.Sentences {
		if got := snapshot.Body[sentence.StartByte:sentence.EndByte]; got != sentence.Text {
			t.Fatalf("编号 %d 的偏移回取 = %q, want %q", sentence.Number, got, sentence.Text)
		}
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("有效快照未通过 Validate: %v", err)
	}
}

func TestBuildSnapshotDecodesGBKAndGB18030(t *testing.T) {
	now := time.Now().UTC()
	source := authorizedTestSource(now)
	for _, charset := range []string{"gbk", "gb2312", "gb18030"} {
		t.Run(charset, func(t *testing.T) {
			encoder := simplifiedchinese.GBK.NewEncoder()
			if charset == "gb18030" {
				encoder = simplifiedchinese.GB18030.NewEncoder()
			}
			raw, err := encoder.Bytes([]byte("公司披露年度报告数据。"))
			if err != nil {
				t.Fatalf("编码测试正文失败: %v", err)
			}
			snapshot, err := BuildSnapshot(source, "https://example.test/report", "text/plain; charset="+charset, raw, now, 1024)
			if err != nil {
				t.Fatalf("GB 系列字符集未能导入: %v", err)
			}
			if snapshot.Body != "公司披露年度报告数据。" {
				t.Fatalf("解码正文 = %q", snapshot.Body)
			}
		})
	}
}

func TestBuildSnapshotRejectsInvalidOrUnauthorizedInput(t *testing.T) {
	now := time.Now().UTC()
	valid := authorizedTestSource(now)
	expired := now.Add(-time.Hour)
	withoutLicense := valid
	withoutLicense.LicenseRef = ""
	unsupportedType := "application/pdf"
	badUTF8 := string([]byte{0xff, 0xfe})
	tests := []struct {
		name      string
		source    Source
		url       string
		mediaType string
		body      []byte
		limit     int64
	}{
		{"授权已过期", func() Source { s := valid; s.LicenseExpires = &expired; return s }(), "https://example.test/a", "text/plain", []byte("正文。"), 100},
		{"缺少书面授权引用", withoutLicense, "https://example.test/a", "text/plain", []byte("正文。"), 100},
		{"URL 不在登记域名", valid, "https://other.test/a", "text/plain", []byte("正文。"), 100},
		{"不支持 PDF 二进制", valid, "https://example.test/a", unsupportedType, []byte("%PDF"), 100},
		{"未知字符集", valid, "https://example.test/a", "text/plain; charset=big5", []byte("正文。"), 100},
		{"ASCII 声明含非 ASCII", valid, "https://example.test/a", "text/plain; charset=us-ascii", []byte("中"), 100},
		{"UTF-8 编码无效", valid, "https://example.test/a", "text/plain; charset=utf-8", []byte(badUTF8), 100},
		{"原始正文超限", valid, "https://example.test/a", "text/plain", []byte("超过"), 2},
		{"正文为空", valid, "https://example.test/a", "text/plain", []byte(" \n\t"), 100},
		{"大小限制非正数", valid, "https://example.test/a", "text/plain", []byte("正文。"), 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildSnapshot(test.source, test.url, test.mediaType, test.body, now, test.limit)
			if err == nil {
				t.Fatal("BuildSnapshot() 应拒绝该输入")
			}
		})
	}
}

func TestSnapshotValidateRejectsTamperingAndNonCanonicalBody(t *testing.T) {
	now := time.Now().UTC()
	snapshot, err := BuildSnapshot(authorizedTestSource(now), "https://example.test/a", "text/plain", []byte("公司披露营业收入 12 亿元。"), now, 1024)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"正文被改但哈希不变", func(s *Snapshot) { s.Body += "篡改" }},
		{"哈希被改", func(s *Snapshot) { s.Hash = strings.Repeat("0", 64) }},
		{"句子偏移被改", func(s *Snapshot) { s.Sentences[0].StartByte++ }},
		{"句子编号被改", func(s *Snapshot) { s.Sentences[0].Number++ }},
		{"非 NFKC 正文", func(s *Snapshot) {
			s.Body = "Ａ公司披露营业收入 12 亿元。"
			digest := sha256.Sum256([]byte(s.Body))
			s.Hash = hex.EncodeToString(digest[:])
			s.Sentences = splitSentences(s.Body)
		}},
		{"句子表缺项", func(s *Snapshot) { s.Sentences = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := snapshot
			candidate.Sentences = append([]Sentence(nil), snapshot.Sentences...)
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("Validate() 应拒绝被篡改快照")
			}
		})
	}
}

func TestValidateURLRejectsCredentialsAndDomainConfusion(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"规范 HTTPS", "https://example.test/report?id=3#page", "https://example.test/report?id=3"},
		{"不接受子域", "https://sub.example.test/report", ""},
		{"不接受域名后缀欺骗", "https://example.test.evil/report", ""},
		{"不接受 HTTP", "http://example.test/report", ""},
		{"不接受用户信息", "https://user@example.test/report", ""},
		{"不接受非标准端口", "https://example.test:444/report", ""},
		{"不接受 IP 地址", "https://127.0.0.1/report", ""},
		{"不接受访问令牌参数", "https://example.test/report?access_token=abc", ""},
		{"不接受签名参数", "https://example.test/report?client_signature=abc", ""},
		{"不接受超过依据字段上限的链接", "https://example.test/report?ref=" + strings.Repeat("x", maxSourceURLBytes), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateURL("example.test", test.url)
			if test.want == "" {
				if !errors.Is(err, ErrUnsafeURL) {
					t.Fatalf("ValidateURL() error = %v, want ErrUnsafeURL", err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("ValidateURL() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestHasCredentialQueryNormalizesKeySpelling(t *testing.T) {
	for _, key := range []string{"access_token", "api-key", "client_secret", "Authorization", "signature", "password"} {
		t.Run(key, func(t *testing.T) {
			if !hasCredentialQuery(url.Values{key: {"value"}}) {
				t.Fatalf("敏感参数名 %q 未被拒绝", key)
			}
		})
	}
}
