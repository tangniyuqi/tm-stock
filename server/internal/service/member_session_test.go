package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type memberReaderFunc func(context.Context, int64) (bool, error)

func (f memberReaderFunc) EnabledMember(c context.Context, id int64) (bool, error) { return f(c, id) }

func TestMemberSessionRechecksStatusAndFailsClosed(t *testing.T) {
	key := strings.Repeat("k", 40)
	v, err := memberauth.NewVerifier(memberauth.Config{SigningKey: key, Issuer: "GVA", Audience: "GVA"})
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	fault := error(nil)
	calls := 0
	r := memberReaderFunc(func(ctx context.Context, id int64) (bool, error) {
		calls++
		d, ok := ctx.Deadline()
		if !ok || time.Until(d) > 3*time.Second || id != 42 {
			t.Fatal("身份/期限丢失")
		}
		return enabled, fault
	})
	s, err := NewMemberSessionService(v, r)
	if err != nil {
		t.Fatal(err)
	}
	makeToken := func(kind string) string {
		p := jwt.MapClaims{"ID": 42, "UserType": kind, "iss": "GVA", "aud": []string{"GVA"}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix()}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, p).SignedString([]byte(key))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	token := makeToken("client")
	if uid, err := s.Authenticate(context.Background(), token); uid != 42 || err != nil {
		t.Fatal(uid, err)
	}
	enabled = false
	if uid, err := s.Authenticate(context.Background(), token); uid != 0 || !errors.Is(err, memberauth.ErrUnauthorized) {
		t.Fatal(uid, err)
	}
	fault = errors.New("模拟内部数据库细节")
	if uid, err := s.Authenticate(context.Background(), token); uid != 0 || err != memberauth.ErrUnavailable {
		t.Fatal(uid, err)
	}
	before := calls
	for _, token := range []string{"bad", makeToken("admin")} {
		if _, err := s.Authenticate(context.Background(), token); !errors.Is(err, memberauth.ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if calls != before {
		t.Fatal("无效身份触达会员表")
	}
	if _, err := NewMemberSessionService(nil, r); err == nil {
		t.Fatal("空校验器")
	}
	if _, err := NewMemberSessionService(v, nil); err == nil {
		t.Fatal("空会员存储")
	}
	var absent *MemberSessionService
	if _, err := absent.Authenticate(context.Background(), token); err != memberauth.ErrUnavailable {
		t.Fatal(err)
	}
}
