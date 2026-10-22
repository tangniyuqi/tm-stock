package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type streamAuthFunc func(context.Context, string) (int64, error)

func (f streamAuthFunc) Authenticate(ctx context.Context, token string) (int64, error) {
	return f(ctx, token)
}

type streamReaderFunc struct {
	read     func(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error)
	progress func(context.Context, int64, int64) (dto.AITaskProgress, error)
}

func (f streamReaderFunc) Events(ctx context.Context, uid, id, after int64, limit int) (dto.AITaskEventPage, error) {
	return f.read(ctx, uid, id, after, limit)
}
func (f streamReaderFunc) Progress(ctx context.Context, uid, id int64) (dto.AITaskProgress, error) {
	if f.progress != nil {
		return f.progress(ctx, uid, id)
	}
	return dto.AITaskProgress{ID: id, Status: 2, EventSeq: 1}, nil
}

func streamPage(after, upper int64, terminal bool) dto.AITaskEventPage {
	p := dto.AITaskEventPage{Items: []dto.AITaskEvent{}, UpperSeq: upper, NextAfterSeq: after}
	for seq := after + 1; seq <= min(upper, after+20); seq++ {
		e := dto.AITaskEvent{Seq: seq, Type: "step_completed", Status: 2, Step: 1, At: 1000 + seq}
		if seq == 1 {
			e.Type, e.Status, e.Step = "created", 1, 0
		} else if seq == 2 {
			e.Type, e.Step = "claimed", 0
		}
		if terminal && seq == upper {
			e.Type, e.Status = "succeeded", 3
		}
		p.Items = append(p.Items, e)
		p.NextAfterSeq = seq
	}
	p.HasMore = p.NextAfterSeq < upper
	return p
}

func streamFixture(t *testing.T, reader streamReaderFunc, auth streamAuthFunc) *AITaskStreamService {
	t.Helper()
	if auth == nil {
		auth = func(ctx context.Context, _ string) (int64, error) { return 42, ctx.Err() }
	}
	s, err := NewAITaskStreamService(reader, auth)
	if err != nil {
		t.Fatal(err)
	}
	s.period, s.life = time.Millisecond, 100*time.Millisecond
	return s
}

func TestAITaskStreamPagesAndResume(t *testing.T) {
	for _, after := range []int64{0, 20, 40} {
		t.Run(strconv.FormatInt(after, 10), func(t *testing.T) {
			var reads, auths int
			s := streamFixture(t, streamReaderFunc{read: func(ctx context.Context, uid, id, cursor int64, limit int) (dto.AITaskEventPage, error) {
				reads++
				if uid != 42 || id != 9 || limit != 20 {
					t.Fatal("错误归属或分页")
				}
				return streamPage(cursor, 41, true), ctx.Err()
			}}, func(ctx context.Context, token string) (int64, error) {
				auths++
				if token != "test-only" {
					t.Fatal("令牌改变")
				}
				return 42, ctx.Err()
			})
			st, err := s.Open(context.Background(), "test-only", 42, 9, after)
			if err != nil {
				t.Fatal(err)
			}
			want := after + 1
			err = st.Run(func(e *dto.AITaskEvent) error {
				if e == nil {
					t.Fatal("有后续页不应等待")
				}
				if e.Seq != want {
					t.Fatal(e.Seq, want)
				}
				want++
				return nil
			})
			if err != nil || want != 42 || reads != auths || reads != int((41-after+19)/20) || s.active != 0 || len(s.users) != 0 {
				t.Fatal(err, want, reads, auths)
			}
			if err := st.Run(func(*dto.AITaskEvent) error { t.Fatal("重复发送"); return nil }); !errors.Is(err, points.ErrInvalid) {
				t.Fatal(err)
			}
			st.Close()
		})
	}
}

func TestAITaskStreamRejectsBrokenPages(t *testing.T) {
	for name, mutate := range map[string]func(*dto.AITaskEventPage){
		"缺事件":  func(p *dto.AITaskEventPage) { p.Items = p.Items[:1] },
		"错序":   func(p *dto.AITaskEventPage) { p.Items[1].Seq = 3 },
		"类型":   func(p *dto.AITaskEventPage) { p.Items[1].Type = "delta" },
		"公开错误": func(p *dto.AITaskEventPage) { p.Items[1].ErrorCode = "模型原文" },
		"状态":   func(p *dto.AITaskEventPage) { p.Items[1].Status = 6 },
		"游标":   func(p *dto.AITaskEventPage) { p.NextAfterSeq = 1 },
		"更多":   func(p *dto.AITaskEventPage) { p.HasMore = true },
		"时间":   func(p *dto.AITaskEventPage) { p.Items[0].At = 0 },
		"步骤":   func(p *dto.AITaskEventPage) { p.Items[1].Step = -1 },
		"空切片":  func(p *dto.AITaskEventPage) { p.Items = nil },
		"越界":   func(p *dto.AITaskEventPage) { p.UpperSeq = 9007199254740992 },
		"假创建":  func(p *dto.AITaskEventPage) { p.Items[0].Type = "retried" },
		"提前终态": func(p *dto.AITaskEventPage) { p.Items[0].Type = "succeeded"; p.Items[0].Status = 3 },
	} {
		t.Run(name, func(t *testing.T) {
			s := streamFixture(t, streamReaderFunc{read: func(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error) {
				p := streamPage(0, 2, false)
				mutate(&p)
				return p, nil
			}}, nil)
			if _, err := s.Open(context.Background(), "test-only", 42, 9, 0); !errors.Is(err, points.ErrInvariant) || s.active != 0 {
				t.Fatal(err, s.active)
			}
		})
	}
	if err := validateStreamPage(streamPage(2, 3, false), 2, 2); !errors.Is(err, points.ErrInvariant) {
		t.Fatal("跨页步骤倒退", err)
	}
	for _, e := range []dto.AITaskEvent{
		{Type: "failed", Status: 4, ErrorCode: "guard_rejected"}, {Type: "cancelled", Status: 5, ErrorCode: "cancelled"}, {Type: "expired", Status: 6, ErrorCode: "expired"}, {Type: "retried", Status: 1}, {Type: "cancel_requested", Status: 2},
	} {
		p := streamPage(2, 3, false)
		e.Seq, e.At, e.Step = 3, 1000, 1
		p.Items[0] = e
		if err := validateStreamPage(p, 2, 0); err != nil {
			t.Fatal(e, err)
		}
	}
}

func TestAITaskStreamAuthenticationAndFailureRelease(t *testing.T) {
	for _, fault := range []error{memberauth.ErrUnauthorized, memberauth.ErrUnavailable, points.ErrNotFound, points.ErrInvariant} {
		var calls int
		s := streamFixture(t, streamReaderFunc{read: func(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error) {
			calls++
			if calls == 1 {
				return streamPage(0, 1, false), nil
			}
			return dto.AITaskEventPage{}, fault
		}}, nil)
		st, err := s.Open(context.Background(), "test-only", 42, 9, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Run(func(*dto.AITaskEvent) error { return nil }); !errors.Is(err, fault) || s.active != 0 {
			t.Fatal(err)
		}
	}
	var reads, auths int
	s := streamFixture(t, streamReaderFunc{read: func(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error) {
		reads++
		return streamPage(0, 1, false), nil
	}}, func(context.Context, string) (int64, error) {
		auths++
		if auths > 1 {
			return 0, memberauth.ErrUnauthorized
		}
		return 42, nil
	})
	st, err := s.Open(context.Background(), "test-only", 42, 9, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Run(func(*dto.AITaskEvent) error { return nil }); !errors.Is(err, memberauth.ErrUnauthorized) || reads != 1 || s.active != 0 {
		t.Fatal(err, reads)
	}
	if _, err := s.Open(context.Background(), "test-only", 7, 9, 0); !errors.Is(err, memberauth.ErrUnauthorized) {
		t.Fatal(err)
	}
}

func TestAITaskStreamFiniteCancellationAndWrites(t *testing.T) {
	for _, mode := range []string{"期限", "父取消", "写失败", "续读终态", "进度失败", "进度矛盾"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := streamFixture(t, streamReaderFunc{read: func(_ context.Context, uid, id, after int64, _ int) (dto.AITaskEventPage, error) {
				return streamPage(after, 1, false), nil
			}, progress: func(context.Context, int64, int64) (dto.AITaskProgress, error) {
				if mode == "进度失败" {
					return dto.AITaskProgress{}, points.ErrNotFound
				}
				if mode == "进度矛盾" {
					return dto.AITaskProgress{ID: 8, Status: 3, EventSeq: 1}, nil
				}
				status := 2
				if mode == "续读终态" {
					status = 3
				}
				return dto.AITaskProgress{ID: 9, Status: status, EventSeq: 1}, nil
			}}, nil)
			s.life = 10 * time.Millisecond
			st, err := s.Open(ctx, "test-only", 42, 9, 1)
			if mode == "进度失败" || mode == "进度矛盾" {
				if err == nil || s.active != 0 {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !st.Deadline().After(time.Now()) {
				t.Fatal("无期限")
			}
			var frames int
			err = st.Run(func(*dto.AITaskEvent) error {
				frames++
				if mode == "父取消" {
					cancel()
				}
				if mode == "写失败" {
					return points.ErrInvariant
				}
				return nil
			})
			switch mode {
			case "期限":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal(err)
				}
			case "父取消":
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case "写失败":
				if !errors.Is(err, points.ErrInvariant) {
					t.Fatal(err)
				}
			case "续读终态":
				if err != nil || frames != 0 {
					t.Fatal(err, frames)
				}
			}
			if s.active != 0 || len(s.users) != 0 {
				t.Fatal("泄漏连接名额")
			}
		})
	}
}

func TestAITaskStreamLimitsAndInputs(t *testing.T) {
	reader := streamReaderFunc{read: func(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error) {
		return streamPage(0, 1, false), nil
	}}
	if _, err := NewAITaskStreamService(nil, nil); err == nil {
		t.Fatal("空依赖")
	}
	s := streamFixture(t, reader, func(ctx context.Context, token string) (int64, error) {
		n, e := strconv.ParseInt(token, 10, 64)
		if e != nil {
			return 0, memberauth.ErrUnauthorized
		}
		return n, ctx.Err()
	})
	s.life = time.Second
	var streams []*AITaskStream
	defer func() {
		for _, st := range streams {
			st.Close()
		}
	}()
	for uid := int64(1); uid <= 32; uid++ {
		st, err := s.Open(context.Background(), strconv.FormatInt(uid, 10), uid, 9, 0)
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, st)
	}
	if _, err := s.Open(context.Background(), "33", 33, 9, 0); !errors.Is(err, ErrAITaskStreamCapacity) {
		t.Fatal(err)
	}
	for _, st := range streams {
		st.Close()
	}
	streams = nil
	for i := 0; i < 2; i++ {
		st, err := s.Open(context.Background(), "42", 42, 9, 0)
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, st)
	}
	if _, err := s.Open(context.Background(), "42", 42, 9, 0); !errors.Is(err, ErrAITaskStreamCapacity) {
		t.Fatal(err)
	}
	for _, v := range []struct {
		token          string
		uid, id, after int64
	}{{"", 42, 9, 0}, {"42", 0, 9, 0}, {"42", 42, 0, 0}, {"42", 42, 9, -1}, {"42", 42, 9007199254740992, 0}, {"42", 42, 9, 9007199254740992}} {
		if _, err := s.Open(context.Background(), v.token, v.uid, v.id, v.after); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(v, err)
		}
	}
	var absent *AITaskStreamService
	if _, err := absent.Open(context.Background(), "42", 42, 9, 0); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Open(nil, "42", 42, 9, 0); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	var closed *AITaskStream
	closed.Close()
	if err := closed.Run(nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal(err)
	}
	if raw := fmt.Sprintf("%#v", streams[0]); raw != "任务进度连接（身份已隐藏）" {
		t.Fatal(raw)
	}
}

func TestAITaskStreamConcurrentOpenClose(t *testing.T) {
	s := streamFixture(t, streamReaderFunc{read: func(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error) {
		return streamPage(0, 1, false), nil
	}}, nil)
	var group sync.WaitGroup
	var unexpected atomic.Int32
	for i := 0; i < 100; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			st, err := s.Open(context.Background(), "test-only", 42, 9, 0)
			if err == nil {
				st.Close()
				st.Close()
			} else if !errors.Is(err, ErrAITaskStreamCapacity) {
				unexpected.Add(1)
			}
		}()
	}
	group.Wait()
	if unexpected.Load() != 0 || s.active != 0 || len(s.users) != 0 {
		t.Fatal("并发连接泄漏", unexpected.Load())
	}
}
