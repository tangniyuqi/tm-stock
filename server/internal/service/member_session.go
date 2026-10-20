package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type MemberIdentityReader interface {
	EnabledMember(context.Context, int64) (bool, error)
}

type MemberSessionService struct {
	verifier *memberauth.Verifier
	members  MemberIdentityReader
}

func NewMemberSessionService(v *memberauth.Verifier, members MemberIdentityReader) (*MemberSessionService, error) {
	if v == nil || members == nil {
		return nil, memberauth.ErrInvalidConfig
	}
	return &MemberSessionService{v, members}, nil
}

func (s *MemberSessionService) Authenticate(ctx context.Context, token string) (int64, error) {
	if s == nil {
		return 0, memberauth.ErrUnavailable
	}
	uid, err := s.verifier.Verify(token)
	if err != nil {
		return 0, memberauth.ErrUnauthorized
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	enabled, err := s.members.EnabledMember(ctx, uid)
	if err != nil {
		return 0, memberauth.ErrUnavailable
	}
	if !enabled {
		return 0, memberauth.ErrUnauthorized
	}
	return uid, nil
}
