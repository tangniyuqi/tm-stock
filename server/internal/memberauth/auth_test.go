package memberauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func authConfig() Config {
	return Config{SigningKey: strings.Repeat("a", 40), PreviousKey: strings.Repeat("b", 40), Issuer: "GVA", Audience: "GVA"}
}
func payload() jwt.MapClaims {
	return jwt.MapClaims{"ID": int64(42), "UserType": "client", "AuthorityId": 0, "UUID": "00000000-0000-0000-0000-000000000000", "Username": "", "NickName": "", "BufferTime": 86400, "mustChangePwd": false, "iss": "GVA", "aud": []string{"GVA"}, "exp": 2000, "nbf": 900}
}
func signed(t *testing.T, c jwt.MapClaims, key string, method jwt.SigningMethod) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func rawSigned(h, p, key string) string {
	enc := base64.RawURLEncoding.EncodeToString
	s := enc([]byte(h)) + "." + enc([]byte(p))
	m := hmac.New(sha256.New, []byte(key))
	_, _ = m.Write([]byte(s))
	return s + "." + enc(m.Sum(nil))
}

func TestVerifierGVAAndRotation(t *testing.T) {
	c := authConfig()
	v, err := newVerifier(c, func() time.Time { return time.Unix(1000, 0) })
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{c.SigningKey, c.PreviousKey} {
		token := signed(t, payload(), key, jwt.SigningMethodHS256)
		if uid, err := v.Verify(token); err != nil || uid != 42 {
			t.Fatal(uid, err)
		}
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if uid, err := v.Verify(token); err != nil || uid != 42 {
					t.Error(uid, err)
				}
			}()
		}
		wg.Wait()
	}
	c.PreviousKey = ""
	v, err = newVerifier(c, func() time.Time { return time.Unix(1000, 0) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(signed(t, payload(), authConfig().PreviousKey, jwt.SigningMethodHS256)); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := NewVerifier(c); err != nil {
		t.Fatal(err)
	}
}

func TestVerifierRejectsUntrustedClaims(t *testing.T) {
	c := authConfig()
	v, err := newVerifier(c, func() time.Time { return time.Unix(1000, 0) })
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, key string
		value     any
	}{
		{"后台令牌", "UserType", "admin"}, {"空类型", "UserType", ""}, {"未知类型", "UserType", "other"}, {"空类型值", "UserType", nil},
		{"零编号", "ID", 0}, {"负编号", "ID", -1}, {"字符串编号", "ID", "42"}, {"小数编号", "ID", 42.5}, {"溢出编号", "ID", json.Number("9223372036854775808")},
		{"后台角色", "AuthorityId", 1}, {"空角色", "AuthorityId", nil}, {"错误签发者", "iss", "other"}, {"错误受众", "aud", []string{"other"}},
		{"到期边界", "exp", 1000}, {"已到期", "exp", 999}, {"未来生效", "nbf", 1001}, {"未来签发", "iat", 1001}, {"反向时间", "nbf", 2001},
		{"小写身份", "id", 42}, {"未知字段", "user_id", 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := payload()
			p[tc.key] = tc.value
			if uid, err := v.Verify(signed(t, p, c.SigningKey, jwt.SigningMethodHS256)); uid != 0 || !errors.Is(err, ErrUnauthorized) {
				t.Fatal(uid, err)
			}
		})
	}
	for _, k := range []string{"ID", "UserType", "iss", "aud", "exp", "nbf"} {
		t.Run("缺失"+k, func(t *testing.T) {
			p := payload()
			delete(p, k)
			if _, err := v.Verify(signed(t, p, c.SigningKey, jwt.SigningMethodHS256)); !errors.Is(err, ErrUnauthorized) {
				t.Fatal(err)
			}
		})
	}
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		if _, err := v.Verify(signed(t, payload(), c.SigningKey, method)); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if _, err := v.Verify(signed(t, payload(), strings.Repeat("c", 40), jwt.SigningMethodHS256)); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}

func TestVerifierStrictJSONAndBounds(t *testing.T) {
	c := authConfig()
	v, err := newVerifier(c, func() time.Time { return time.Unix(1000, 0) })
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(payload())
	base := string(p)
	badPayloads := []string{
		strings.Replace(base, `"ID":42`, `"ID":42,"ID":43`, 1),
		strings.Replace(base, `"ID":42`, `"ID":42,"id":43`, 1),
		strings.Replace(base, `"ID":42`, `"ID":42,"\u0049D":43`, 1),
		strings.Replace(base, `"nbf":900`, `"nbf":null`, 1),
		strings.Replace(base, `"exp":2000`, `"exp":null`, 1),
		strings.Replace(base, `"iss":"GVA"`, `"ISS":"GVA"`, 1),
		base + " {}", "[]", "{", strings.Replace(base, `"NickName":""`, "\"NickName\":\""+string([]byte{255})+"\"", 1),
	}
	for i, p := range badPayloads {
		if _, err := v.Verify(rawSigned(`{"alg":"HS256","typ":"JWT"}`, p, c.SigningKey)); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(i, err)
		}
	}
	for _, h := range []string{`{"alg":"HS256","alg":"HS256"}`, `{"alg":"HS256","jku":"https://example.invalid"}`, `{"alg":"none"}`, `[]`, `{}`} {
		if _, err := v.Verify(rawSigned(h, base, c.SigningKey)); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(h, err)
		}
	}
	for _, s := range []string{"", "abc", "...", "!!..", strings.Repeat("a", 8193)} {
		if _, err := v.Verify(s); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	var absent *Verifier
	if _, err := absent.Verify("a"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := object([]byte(`{"a":`)); err == nil {
		t.Fatal("截断未拒绝")
	}
	if _, err := object([]byte(`{"a":1,`)); err == nil {
		t.Fatal("截断键未拒绝")
	}
	if _, err := object([]byte(`{"a":1`)); err == nil {
		t.Fatal("缺少闭括号未拒绝")
	}
	m := map[string]int{}
	for i := 0; i < 33; i++ {
		m[fmt.Sprint(i)] = i
	}
	b, _ := json.Marshal(m)
	if _, err := object(b); err == nil {
		t.Fatal("键过多未拒绝")
	}
}

func TestAuthConfigFailClosedAndRedacted(t *testing.T) {
	for _, change := range []func(*Config){func(c *Config) { c.SigningKey = "" }, func(c *Config) { c.SigningKey = "short" }, func(c *Config) { c.PreviousKey = c.SigningKey }, func(c *Config) { c.PreviousKey = "short" }, func(c *Config) { c.Issuer = "" }, func(c *Config) { c.Audience = "" }, func(c *Config) { c.Issuer = " GVA" }, func(c *Config) { c.SigningKey += "\n" }} {
		c := authConfig()
		change(&c)
		if _, err := NewVerifier(c); !errors.Is(err, ErrInvalidConfig) {
			t.Fatal(err)
		}
	}
	c := authConfig()
	for _, s := range []string{fmt.Sprint(c), fmt.Sprintf("%#v", c)} {
		if strings.Contains(s, c.SigningKey) || strings.Contains(s, c.PreviousKey) {
			t.Fatal("密钥泄漏")
		}
	}
}
