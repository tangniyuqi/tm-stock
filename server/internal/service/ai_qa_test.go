package service

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"testing"
)

type qaUnavailableStore struct{ qa.AccessStore }

func (qaUnavailableStore) WithinAccess(context.Context, func(qa.AccessTx) error) error {
	return qa.ErrUnavailable
}

type qaUnitAuth struct {
	uid int64
	err error
}

func (a qaUnitAuth) Authenticate(context.Context, string) (int64, error) { return a.uid, a.err }
func TestQAServiceMissingAndIdentityBoundaries(t *testing.T) {
	if _, err := NewAIQAService(nil, nil, nil, qaUnitAuth{uid: 1}, false); err == nil {
		t.Fatal("空存储")
	}
	if _, err := NewAIQAService(qaUnavailableStore{}, nil, nil, nil, false); err == nil {
		t.Fatal("空身份校验")
	}
	if _, err := NewAIQAService(qaUnavailableStore{}, nil, nil, qaUnitAuth{uid: 1}, true); err == nil {
		t.Fatal("没有模型工厂仍开启")
	}
	s, err := NewAIQAService(qaUnavailableStore{}, nil, nil, qaUnitAuth{uid: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, call := range []func() error{func() error { _, e := s.Receipt(ctx, "test", 1, 1, "bad"); return e }, func() error { _, e := s.Status(ctx, "test", 1, 1, 0); return e }, func() error { _, e := s.Answer(ctx, "test", 1, 1, 0); return e }, func() error { _, e := s.Submit(ctx, "test", 1, 1, "test_question_key_01", dto.AIQAInput{}); return e }, func() error { _, e := s.Status(ctx, "test", 2, 1, 1); return e }, func() error { _, e := s.Status(ctx, "", 1, 1, 1); return e }, func() error { _, e := s.Status(nil, "test", 1, 1, 1); return e }, func() error { _, e := s.Receipt(ctx, "test", 1, 1, "test_question_key_01"); return e }, func() error { _, e := s.Answer(ctx, "test", 1, 1, 1); return e }} {
		if call() == nil {
			t.Fatal("无效/不可用读取被接纳")
		}
	}
	input := dto.AIQAInput{Version: 1, SubjectType: "theme", SubjectID: 1, Skill: "material_qa", Question: "测试问题"}
	if _, err := s.Submit(ctx, "test", 1, 1, "test_question_key_01", input); err == nil {
		t.Fatal("关闭提交仍运行")
	}
	s.auth = qaUnitAuth{err: memberauth.ErrUnauthorized}
	if s.check(ctx, "test", 1) == nil {
		t.Fatal("过期身份仍通过")
	}
}
