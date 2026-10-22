package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestAIConversationQuotedCreateNeverFreezesUnexpectedAmount(t *testing.T) {
	for _, mode := range []string{"invalid", "mismatch", "same_version_changed", "success_and_replay", "closed"} {
		t.Run(mode, func(t *testing.T) {
			s, store, ctl, input, key := admissionFixture(t)
			expected := int64(7)
			want := error(nil)
			switch mode {
			case "invalid":
				expected = 0
				want = points.ErrInvalid
			case "mismatch":
				expected = 8
				want = points.ErrPriceChanged
			case "same_version_changed":
				ctl.p.Price.Points = 9
				want = points.ErrPriceChanged
			case "closed":
				s.enabled = false
				want = task.ErrCreationDisabled
			}
			out, err := s.CreateQuoted(context.Background(), 1, key, input, expected)
			if !errors.Is(err, want) {
				t.Fatal(mode, err)
			}
			if mode != "success_and_replay" {
				if store.exists || store.points.account.Frozen != 0 || out.TaskID != 0 {
					t.Fatal("金额拒绝发生在冻结之后")
				}
				return
			}
			if out.PricePoints != 7 || store.points.account.Frozen != 7 {
				t.Fatal(out, err)
			}
			first := out
			out, err = s.CreateQuoted(context.Background(), 1, key, input, 7)
			if err != nil || out != first || len(store.events) != 1 {
				t.Fatal("原价重放不稳定", err)
			}
			if _, err := s.CreateQuoted(context.Background(), 1, key, input, 8); !errors.Is(err, points.ErrIdempotency) || store.points.account.Frozen != 7 || len(store.events) != 1 {
				t.Fatal("错价原键重放", err)
			}
		})
	}
}
func TestAITaskDeepQuoteAndCreatePrice(t *testing.T) {
	s, store, ctl, input, key := admissionFixture(t)
	store.points.lots[0].Total, store.points.lots[0].Remain, store.points.account.Avail = 50, 50, 50
	ctl.p.Scope.AnalysisType = task.DeepAnalysisType
	ctl.p.Price.Points = task.DeepPricePoints
	input.AnalysisType = "deep"
	s.SetDeepLimiter(&fakeDeepLimiter{})
	q, err := s.Quote(context.Background(), 1, quoteInput(input))
	if err != nil || q.PricePoints != task.DeepPricePoints || q.AvailablePoints != 50 || q.AnalysisType != "deep" {
		t.Fatal("深度报价不符", q, err)
	}
	out, err := s.CreateQuoted(context.Background(), 1, key, input, task.DeepPricePoints)
	if err != nil || out.PricePoints != task.DeepPricePoints || store.points.account.Frozen != task.DeepPricePoints {
		t.Fatal("深度创建未按三十积分冻结", out, err)
	}
	// 控制面仍返回 L1 计划时，深度报价/创建必须失败，不能仅凭 analysisType=deep 放行。
	ctl.p.Scope.AnalysisType = task.LightAnalysisType
	ctl.p.Price.Points = 7
	store.points.account.Frozen = 0
	if _, err := s.Quote(context.Background(), 1, quoteInput(input)); err == nil {
		t.Fatal("深度范围接受 L1 计划")
	}
}

type fakeDeepLimiter struct {
	err   error
	calls int
}

func (l *fakeDeepLimiter) Acquire(context.Context, int64) (func(), error) {
	l.calls++
	if l.err != nil {
		return nil, l.err
	}
	return func() {}, nil
}

func TestDeepCreateRequiresLimiter(t *testing.T) {
	s, store, ctl, input, key := admissionFixture(t)
	store.points.lots[0].Total, store.points.lots[0].Remain, store.points.account.Avail = 50, 50, 50
	ctl.p.Scope.AnalysisType, ctl.p.Price.Points = task.DeepAnalysisType, task.DeepPricePoints
	input.AnalysisType = "deep"
	if _, err := s.CreateQuoted(context.Background(), 1, key, input, task.DeepPricePoints); !errors.Is(err, task.ErrAdmissionUnavailable) {
		t.Fatal("未注入并发限制器时深度创建未失败关闭", err)
	}
	if store.points.account.Frozen != 0 || store.exists {
		t.Fatal("失败关闭发生在冻结之后")
	}
	limiter := &fakeDeepLimiter{}
	s.SetDeepLimiter(limiter)
	if _, err := s.CreateQuoted(context.Background(), 1, key, input, task.DeepPricePoints); err != nil || limiter.calls != 1 || store.points.account.Frozen != task.DeepPricePoints {
		t.Fatal("注入限制器后深度创建失败", err)
	}
	limiter.err = task.ErrAdmissionUnavailable
	store.points.account.Frozen = 0
	if _, err := s.CreateQuoted(context.Background(), 1, "test_deep_second_001", input, task.DeepPricePoints); !errors.Is(err, task.ErrAdmissionUnavailable) || store.points.account.Frozen != 0 {
		t.Fatal("并发已满时仍冻结", err)
	}
}
