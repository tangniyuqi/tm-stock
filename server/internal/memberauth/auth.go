// Package memberauth 校验 GVA 会员令牌，不签发令牌，不持有会员资料。
package memberauth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidConfig = errors.New("会员鉴权配置无效")
	ErrUnauthorized  = errors.New("会员身份无效")
	ErrUnavailable   = errors.New("会员身份校验暂不可用")
)

type Config struct {
	SigningKey, PreviousKey string
	Issuer, Audience        string
}

// String/GoString 避免配置被常见格式化日志意外输出。
func (Config) String() string     { return "会员鉴权配置（密钥已隐藏）" }
func (c Config) GoString() string { return c.String() }

func (c Config) Validate() error {
	validKey := func(s string) bool {
		return len(s) >= 32 && len(s) <= 4096 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
	}
	validName := func(s string) bool {
		return len(s) > 0 && len(s) <= 128 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
	}
	if !validKey(c.SigningKey) || (c.PreviousKey != "" && (!validKey(c.PreviousKey) || c.PreviousKey == c.SigningKey)) || !validName(c.Issuer) || !validName(c.Audience) {
		return ErrInvalidConfig
	}
	return nil
}

type Verifier struct {
	keys   [][]byte
	parser *jwt.Parser
}

func NewVerifier(c Config) (*Verifier, error) { return newVerifier(c, time.Now) }

func newVerifier(c Config, now func() time.Time) (*Verifier, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	v := &Verifier{keys: [][]byte{[]byte(c.SigningKey)}, parser: jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(c.Issuer), jwt.WithAudience(c.Audience),
		jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithStrictDecoding(), jwt.WithTimeFunc(now),
	)}
	if c.PreviousKey != "" {
		v.keys = append(v.keys, []byte(c.PreviousKey))
	}
	return v, nil
}

type claims struct {
	ID          int64
	UserType    string
	AuthorityID int64
	jwt.RegisteredClaims
}

// object 拒绝重复键（含大小写变体）、非对象与尾随值，避免不同解析器身份解释不一致。
func object(b []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(b) {
		return nil, ErrUnauthorized
	}
	d := json.NewDecoder(bytes.NewReader(b))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, ErrUnauthorized
	}
	out := make(map[string]json.RawMessage)
	seen := make(map[string]bool)
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return nil, ErrUnauthorized
		}
		k, ok := t.(string)
		if !ok || seen[strings.ToLower(k)] || len(out) >= 32 {
			return nil, ErrUnauthorized
		}
		seen[strings.ToLower(k)] = true
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, ErrUnauthorized
		}
		out[k] = raw
	}
	if _, err := d.Token(); err != nil {
		return nil, ErrUnauthorized
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrUnauthorized
	}
	return out, nil
}

func (c *claims) UnmarshalJSON(b []byte) error {
	m, err := object(b)
	if err != nil {
		return err
	}
	for k := range m {
		switch k {
		case "UUID", "ID", "Username", "NickName", "AuthorityId", "UserType", "BufferTime", "mustChangePwd", "iss", "aud", "exp", "nbf", "iat", "sub", "jti":
		default:
			return ErrUnauthorized
		}
	}
	if json.Unmarshal(m["ID"], &c.ID) != nil || c.ID <= 0 || json.Unmarshal(m["UserType"], &c.UserType) != nil || c.UserType != "client" {
		return ErrUnauthorized
	}
	if raw, ok := m["AuthorityId"]; ok {
		if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &c.AuthorityID) != nil || c.AuthorityID != 0 {
			return ErrUnauthorized
		}
	}
	if err := json.Unmarshal(b, &c.RegisteredClaims); err != nil {
		return ErrUnauthorized
	}
	if c.NotBefore == nil || c.ExpiresAt == nil || !c.ExpiresAt.After(c.NotBefore.Time) {
		return ErrUnauthorized
	}
	return nil
}

func (v *Verifier) Verify(raw string) (int64, error) {
	if v == nil || len(raw) == 0 || len(raw) > 8192 {
		return 0, ErrUnauthorized
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return 0, ErrUnauthorized
	}
	header, err := base64.RawURLEncoding.Strict().DecodeString(parts[0])
	if err != nil {
		return 0, ErrUnauthorized
	}
	h, err := object(header)
	if err != nil {
		return 0, ErrUnauthorized
	}
	for k := range h {
		if k != "alg" && k != "typ" {
			return 0, ErrUnauthorized
		}
	}
	for _, key := range v.keys {
		c := &claims{}
		t, err := v.parser.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, ErrUnauthorized
			}
			return key, nil
		})
		if err == nil && t != nil && t.Valid {
			return c.ID, nil
		}
	}
	return 0, ErrUnauthorized
}
