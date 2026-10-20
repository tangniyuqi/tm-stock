// Package sources 管理经授权的资料来源及其不可变快照。
package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"mime"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"golang.org/x/text/encoding/simplifiedchinese"
)

var (
	ErrSourceUnavailable = errors.New("资料来源不可用")
	ErrUnauthorized      = errors.New("资料来源缺少有效授权")
	ErrUnsafeURL         = errors.New("资料链接不符合安全规则")
	ErrUnsupportedMedia  = errors.New("资料类型暂不支持")
	ErrBodyTooLarge      = errors.New("资料正文超过大小限制")
)

const (
	MediaHTML         = "text/html"
	MediaPlain        = "text/plain"
	maxSourceURLBytes = 512

	CategoryAnnouncement = "announcement"
	CategoryAnnualReport = "annual_report"
	CategoryProspectus   = "prospectus"
	CategoryOfficialList = "official_catalog"
	CategoryInteractive  = "interactive_qa"
)

// Source 是运营登记的资料来源。Domain 使用精确主机名匹配，不隐式放行子域名。
type Source struct {
	ID             int64
	Domain         string
	Category       string
	LicenseTag     string
	LicenseRef     string
	LicenseExpires *time.Time
	MinInterval    time.Duration
	Enabled        bool
}

// HasValidAuthorization 判断来源是否启用且持有当前有效的书面授权记录。
func (s Source) HasValidAuthorization(now time.Time) bool {
	if !s.Enabled || strings.TrimSpace(s.LicenseTag) == "" || strings.TrimSpace(s.LicenseRef) == "" {
		return false
	}
	return s.LicenseExpires == nil || s.LicenseExpires.After(now)
}

// Sentence 的偏移量是 UTF-8 正文中的字节区间 [StartByte, EndByte)，可直接回取原文。
type Sentence struct {
	Number    int    `json:"number"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Text      string `json:"text"`
}

// Snapshot 是固化后的规范化正文。Hash 为正文 UTF-8 字节的 SHA-256 小写十六进制值。
type Snapshot struct {
	ID             int64
	SourceID       int64
	SourceCategory string
	URL            string
	Hash           string
	Body           string
	Sentences      []Sentence
	LicenseTag     string
	LicenseRef     string
	LicenseExpires *time.Time
	FetchedAt      time.Time
}

// NormalizeRegistration 校验登记字段并返回标准化域名。
func (s Source) NormalizeRegistration() (Source, error) {
	domain, ok := normalizeDomain(s.Domain)
	if !ok {
		return Source{}, fmt.Errorf("来源域名无效")
	}
	s.Domain = domain
	s.Category = strings.TrimSpace(s.Category)
	s.LicenseTag = strings.TrimSpace(s.LicenseTag)
	s.LicenseRef = strings.TrimSpace(s.LicenseRef)
	if !validCategory(s.Category) {
		return Source{}, fmt.Errorf("来源类别不在允许范围内")
	}
	if len(s.LicenseTag) > 128 || len(s.LicenseRef) > 512 {
		return Source{}, fmt.Errorf("授权标签或授权归档引用超过长度限制")
	}
	if s.LicenseExpires != nil {
		expiresAt := s.LicenseExpires.UTC().Truncate(time.Millisecond)
		s.LicenseExpires = &expiresAt
	}
	if s.MinInterval < 0 {
		return Source{}, fmt.Errorf("来源访问间隔不能为负数")
	}
	if s.Enabled && !s.HasValidAuthorization(time.Now().UTC()) {
		return Source{}, ErrUnauthorized
	}
	return s, nil
}

// BuildSnapshot 从网页响应或运营导入的 UTF-8 文本构建可复核快照。
func BuildSnapshot(source Source, rawURL, mediaType string, body []byte, fetchedAt time.Time, maxBytes int64) (Snapshot, error) {
	var out Snapshot
	normalizedSource, err := source.NormalizeRegistration()
	if err != nil || !normalizedSource.HasValidAuthorization(fetchedAt) {
		return out, ErrUnauthorized
	}
	source = normalizedSource
	canonicalURL, err := ValidateURL(source.Domain, rawURL)
	if err != nil {
		return out, err
	}
	if maxBytes <= 0 {
		return out, fmt.Errorf("正文大小限制必须为正数")
	}
	if int64(len(body)) > maxBytes {
		return out, ErrBodyTooLarge
	}
	media, params, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return out, fmt.Errorf("解析资料类型失败: %w", err)
	}
	if !validCharset(params["charset"]) {
		return out, ErrUnsupportedMedia
	}
	decoded, err := decodeBody(body, params["charset"])
	if err != nil {
		return out, err
	}
	var text string
	switch strings.ToLower(media) {
	case MediaHTML:
		text = extractHTMLText(decoded)
	case MediaPlain:
		text = decoded
	default:
		return out, ErrUnsupportedMedia
	}
	text = normalizeBody(text)
	if text == "" {
		return out, fmt.Errorf("资料正文为空")
	}
	if int64(len(text)) > maxBytes {
		return out, ErrBodyTooLarge
	}

	digest := sha256.Sum256([]byte(text))
	out = Snapshot{
		SourceID:       source.ID,
		SourceCategory: source.Category,
		URL:            canonicalURL,
		Hash:           hex.EncodeToString(digest[:]),
		Body:           text,
		Sentences:      splitSentences(text),
		LicenseTag:     strings.TrimSpace(source.LicenseTag),
		LicenseRef:     strings.TrimSpace(source.LicenseRef),
		LicenseExpires: cloneTime(source.LicenseExpires),
		FetchedAt:      fetchedAt.UTC().Truncate(time.Millisecond),
	}
	if err := out.Validate(); err != nil {
		return Snapshot{}, err
	}
	return out, nil
}

func validCharset(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii", "gbk", "gb2312", "gb18030":
		return true
	default:
		return false
	}
}

func decodeBody(body []byte, charset string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8":
		if !utf8.Valid(body) {
			return "", fmt.Errorf("资料正文不是有效的 UTF-8 编码")
		}
		return string(body), nil
	case "us-ascii", "ascii":
		for _, b := range body {
			if b >= utf8.RuneSelf {
				return "", fmt.Errorf("ASCII 编码正文含非 ASCII 字节")
			}
		}
		return string(body), nil
	case "gbk", "gb2312":
		decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(body)
		if err != nil || !utf8.Valid(decoded) {
			return "", fmt.Errorf("资料正文 GBK 解码失败")
		}
		return string(decoded), nil
	case "gb18030":
		decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(body)
		if err != nil || !utf8.Valid(decoded) {
			return "", fmt.Errorf("资料正文 GB18030 解码失败")
		}
		return string(decoded), nil
	default:
		return "", ErrUnsupportedMedia
	}
}

// ValidateURL 只接受 HTTPS、标准端口、无用户信息且主机与登记域名完全相同的链接。
func ValidateURL(allowedDomain, raw string) (string, error) {
	if len(strings.TrimSpace(raw)) > maxSourceURLBytes {
		return "", ErrUnsafeURL
	}
	domain, ok := normalizeDomain(allowedDomain)
	if !ok {
		return "", ErrUnsafeURL
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.User != nil || u.Opaque != "" || u.Host == "" {
		return "", ErrUnsafeURL
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", ErrUnsafeURL
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host != domain || net.ParseIP(host) != nil {
		return "", ErrUnsafeURL
	}
	if port := u.Port(); port != "" && port != "443" {
		return "", ErrUnsafeURL
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || hasCredentialQuery(query) {
		return "", ErrUnsafeURL
	}
	u.Scheme = "https"
	u.Host = domain
	u.Fragment = ""
	u.RawFragment = ""
	canonicalURL := u.String()
	if len(canonicalURL) > maxSourceURLBytes {
		return "", ErrUnsafeURL
	}
	return canonicalURL, nil
}

func hasCredentialQuery(query url.Values) bool {
	for key := range query {
		normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
		switch {
		case strings.Contains(normalized, "token"),
			strings.Contains(normalized, "secret"),
			strings.Contains(normalized, "password"),
			strings.Contains(normalized, "passwd"),
			strings.Contains(normalized, "authorization"),
			strings.Contains(normalized, "signature"),
			strings.Contains(normalized, "credential"),
			strings.Contains(normalized, "session"),
			strings.Contains(normalized, "ticket"),
			strings.Contains(normalized, "auth"),
			normalized == "key", strings.HasSuffix(normalized, "apikey"),
			strings.HasSuffix(normalized, "accesskey"), strings.HasSuffix(normalized, "privatekey"), strings.HasSuffix(normalized, "clientkey"),
			normalized == "sig":
			return true
		}
	}
	return false
}

func normalizeDomain(value string) (string, bool) {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if len(domain) == 0 || len(domain) > 253 || net.ParseIP(domain) != nil {
		return "", false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
				return "", false
			}
		}
	}
	return domain, true
}

func validCategory(value string) bool {
	switch value {
	case CategoryAnnouncement, CategoryAnnualReport, CategoryProspectus, CategoryOfficialList, CategoryInteractive:
		return true
	default:
		return false
	}
}

func normalizeBody(value string) string {
	return guard.Normalize(value)
}

func extractHTMLText(value string) string {
	var b strings.Builder
	for pos := 0; pos < len(value); {
		if strings.HasPrefix(value[pos:], "<!--") {
			end := strings.Index(value[pos+4:], "-->")
			if end < 0 {
				break
			}
			pos += 4 + end + 3
			continue
		}
		if value[pos] != '<' {
			next := strings.IndexByte(value[pos:], '<')
			if next < 0 {
				next = len(value) - pos
			}
			b.WriteString(html.UnescapeString(value[pos : pos+next]))
			pos += next
			continue
		}
		end, ok := htmlTagEnd(value, pos+1)
		if !ok {
			b.WriteByte('<')
			pos++
			continue
		}
		tag, closing := parseTagName(value[pos+1 : end])
		if !closing && (tag == "script" || tag == "style" || tag == "noscript" || tag == "template") {
			closeAt := strings.Index(strings.ToLower(value[end+1:]), "</"+tag)
			if closeAt < 0 {
				break
			}
			closeEnd, found := htmlTagEnd(value, end+1+closeAt+2+len(tag))
			if !found {
				break
			}
			pos = closeEnd + 1
			continue
		}
		if isBlockTag(tag) {
			b.WriteByte('\n')
		}
		pos = end + 1
	}
	return b.String()
}

func htmlTagEnd(value string, start int) (int, bool) {
	var quote byte
	for i := start; i < len(value); i++ {
		c := value[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		} else if c == '>' {
			return i, true
		}
	}
	return 0, false
}

func parseTagName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	closing := strings.HasPrefix(value, "/")
	if closing {
		value = strings.TrimSpace(value[1:])
	}
	end := 0
	for end < len(value) {
		c := value[end]
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') {
			break
		}
		end++
	}
	return strings.ToLower(value[:end]), closing
}

func isBlockTag(tag string) bool {
	switch tag {
	case "address", "article", "blockquote", "br", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "ol", "p", "pre", "section", "table", "tbody", "td", "th", "thead", "tr", "ul":
		return true
	default:
		return false
	}
}

func splitSentences(body string) []Sentence {
	snapshot := guard.NewSnapshot(body)
	out := make([]Sentence, 0, len(snapshot.Sentences))
	for _, sentence := range snapshot.Sentences {
		out = append(out, Sentence{
			Number:    sentence.ID,
			StartByte: sentence.Start,
			EndByte:   sentence.End,
			Text:      snapshot.Body[sentence.Start:sentence.End],
		})
	}
	return out
}

// Validate 确保编号、偏移、逐字文本与不可变正文之间一致。
func (s Snapshot) Validate() error {
	if s.SourceID <= 0 || !validCategory(s.SourceCategory) || s.Hash == "" || s.Body == "" || s.URL == "" || s.FetchedAt.IsZero() || strings.TrimSpace(s.LicenseTag) == "" || strings.TrimSpace(s.LicenseRef) == "" {
		return fmt.Errorf("快照字段不完整")
	}
	if !utf8.ValidString(s.Body) || guard.Normalize(s.Body) != s.Body || (s.LicenseExpires != nil && !s.LicenseExpires.After(s.FetchedAt)) {
		return fmt.Errorf("快照正文编码或授权时点无效")
	}
	digest := sha256.Sum256([]byte(s.Body))
	if s.Hash != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("快照内容哈希不匹配")
	}
	expected := splitSentences(s.Body)
	if len(expected) != len(s.Sentences) {
		return fmt.Errorf("快照句子索引数量与正文不一致")
	}
	for i, sentence := range s.Sentences {
		if sentence != expected[i] {
			return fmt.Errorf("快照句子编号或原文偏移无效，编号=%d", i+1)
		}
	}
	return nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
