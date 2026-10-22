package qa

import (
	"context"
	"fmt"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"testing"
)

func TestQAExecutionSessionCheckAtEveryBoundary(t *testing.T) {
	for cut := 1; cut <= 17; cut++ {
		t.Run(fmt.Sprint(cut), func(t *testing.T) {
			c, e, _, _ := durableFixture(t)
			queueQA(t, c)
			checks := 0
			v, err := c.RunChecked(context.Background(), e, func(context.Context) error {
				checks++
				if checks >= cut {
					return ErrUnavailable
				}
				return nil
			})
			if err == nil || v != nil || checks < cut {
				t.Fatal("阶段身份失效仍交付结果", cut, checks)
			}
		})
	}
}
func TestQAReadWithoutProviderAndKnownQueuedRejection(t *testing.T) {
	t.Run("历史不需要模型实例", func(t *testing.T) {
		c, e, _, _ := durableFixture(t)
		queueQA(t, c)
		if _, err := c.RunChecked(context.Background(), e, func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		e.models = [3]*provider.OpenAICompat{}
		if out, err := c.Read(context.Background()); err != nil || len(out.Quotes) != 1 {
			t.Fatal("无模型不能读存证", err)
		}
	})
	t.Run("配额拒绝封存未执行问题", func(t *testing.T) {
		c, e, s, _ := durableFixture(t)
		queueQA(t, c)
		s.policy.UserTokens = 1
		if _, err := c.Run(context.Background(), e); err != ErrQuota {
			t.Fatal("未保留已知配额分类", err)
		}
		if err := c.RejectQueued(context.Background()); err != nil || s.data.Question.State != "failed" || len(s.data.Quotas) != 0 {
			t.Fatal("未执行问题封存或预留错误", err)
		}
	})
	t.Run("不能封存他人已开始执行", func(t *testing.T) {
		c, _, s, p := durableFixture(t)
		queueQA(t, c)
		if err := c.Begin(context.Background(), p.Proof()); err != nil {
			t.Fatal(err)
		}
		if err := c.RejectQueued(context.Background()); err != nil || s.data.Question.State != "running" {
			t.Fatal("配额拒绝覆盖了并发执行", err)
		}
	})
	var empty *DurableControl
	if _, err := empty.Read(context.Background()); err == nil {
		t.Fatal("空控制器可读")
	}
}
