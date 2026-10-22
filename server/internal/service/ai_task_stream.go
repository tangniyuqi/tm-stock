package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// ErrAITaskStreamCapacity 只表示本实例连接配额不足，不承诺跨实例限流。
var ErrAITaskStreamCapacity = errors.New("任务进度连接已达本实例上限")

type aiTaskStreamReader interface {
	Events(context.Context, int64, int64, int64, int) (dto.AITaskEventPage, error)
	Progress(context.Context, int64, int64) (dto.AITaskProgress, error)
}
type aiTaskStreamAuthenticator interface {
	Authenticate(context.Context, string) (int64, error)
}

// AITaskStreamService 只读原任务事件；不持有事务等待，不调用模型或账本。
type AITaskStreamService struct {
	reader aiTaskStreamReader
	auth   aiTaskStreamAuthenticator
	mu     sync.Mutex
	users  map[int64]int
	active int
	period time.Duration
	life   time.Duration
}

func NewAITaskStreamService(reader aiTaskStreamReader, auth aiTaskStreamAuthenticator) (*AITaskStreamService, error) {
	if reader == nil || auth == nil {
		return nil, points.ErrInvalid
	}
	return &AITaskStreamService{reader: reader, auth: auth, users: map[int64]int{}, period: time.Second, life: 15 * time.Second}, nil
}

// AITaskStream 有界、单消费者；Close在未开始发送或任何失败出口也必须调用。
type AITaskStream struct {
	svc       *AITaskStreamService
	ctx       context.Context
	cancel    context.CancelFunc
	token     string
	uid, id   int64
	after     int64
	lastStep  int
	first     dto.AITaskEventPage
	terminal  bool
	closeOnce sync.Once
	runOnce   sync.Once
}

func (*AITaskStream) String() string     { return "任务进度连接（身份已隐藏）" }
func (s *AITaskStream) GoString() string { return s.String() }

func (s *AITaskStream) Deadline() time.Time {
	deadline, _ := s.ctx.Deadline()
	return deadline
}

func (s *AITaskStreamService) acquire(uid int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active >= 32 || s.users[uid] >= 2 {
		return false
	}
	s.active++
	s.users[uid]++
	return true
}

func (s *AITaskStream) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.cancel()
		s.svc.mu.Lock()
		defer s.svc.mu.Unlock()
		s.svc.active--
		s.svc.users[s.uid]--
		if s.svc.users[s.uid] == 0 {
			delete(s.svc.users, s.uid)
		}
	})
}

// Open 在写200之前验证身份、归属、断点及首个事件页；失败不占用连接名额。
func (s *AITaskStreamService) Open(parent context.Context, token string, uid, id, after int64) (*AITaskStream, error) {
	if s == nil || s.reader == nil || s.auth == nil || parent == nil || token == "" || uid <= 0 || uid > 9007199254740991 || id <= 0 || id > 9007199254740991 || after < 0 || after > 9007199254740991 {
		return nil, points.ErrInvalid
	}
	if !s.acquire(uid) {
		return nil, ErrAITaskStreamCapacity
	}
	ctx, cancel := context.WithTimeout(parent, s.life)
	stream := &AITaskStream{svc: s, ctx: ctx, cancel: cancel, token: token, uid: uid, id: id, after: after, lastStep: -1}
	page, err := stream.read()
	if err != nil {
		stream.Close()
		return nil, err
	}
	stream.first = page
	return stream, nil
}

func (s *AITaskStream) read() (dto.AITaskEventPage, error) {
	uid, err := s.svc.auth.Authenticate(s.ctx, s.token)
	if err != nil {
		return dto.AITaskEventPage{}, err
	}
	if uid != s.uid {
		return dto.AITaskEventPage{}, memberauth.ErrUnauthorized
	}
	page, err := s.svc.reader.Events(s.ctx, s.uid, s.id, s.after, 20)
	if err != nil {
		return dto.AITaskEventPage{}, err
	}
	if err = validateStreamPage(page, s.after, s.lastStep); err != nil {
		return dto.AITaskEventPage{}, err
	}
	if len(page.Items) == 0 {
		// 已续读至终态末序号的连接无需空等；单独读取更新快照不授予报告许可。
		progress, err := s.svc.reader.Progress(s.ctx, s.uid, s.id)
		if err != nil {
			return dto.AITaskEventPage{}, err
		}
		if progress.ID != s.id || progress.EventSeq < page.UpperSeq || progress.Status < task.Queued || progress.Status > task.Expired {
			return dto.AITaskEventPage{}, points.ErrInvariant
		}
		s.terminal = progress.EventSeq == s.after && progress.Status >= task.Succeeded
	}
	return page, s.ctx.Err()
}

// validateStreamPage 约束跨页连续性及唯一公开DTO，不能因上游替换而推送半页损坏数据。
func validateStreamPage(p dto.AITaskEventPage, after int64, lastStep int) error {
	if p.UpperSeq < 1 || p.UpperSeq < after || p.UpperSeq > 9007199254740991 || p.Items == nil || len(p.Items) != int(min(p.UpperSeq-after, 20)) || p.NextAfterSeq != after+int64(len(p.Items)) || p.HasMore != (p.NextAfterSeq < p.UpperSeq) {
		return points.ErrInvariant
	}
	for i, e := range p.Items {
		if e.Seq != after+int64(i)+1 || e.Step < 0 || e.Step > task.MaxSteps || e.Step < lastStep || e.At <= 0 || e.At > 8640000000000000 {
			return points.ErrInvariant
		}
		expected := 0
		switch e.Type {
		case "created":
			expected = task.Queued
			if e.Seq != 1 || e.Step != 0 {
				return points.ErrInvariant
			}
		case "retried":
			expected = task.Queued
		case "claimed", "cancel_requested", "step_completed":
			expected = task.Running
			if e.Type == "step_completed" && e.Step == 0 {
				return points.ErrInvariant
			}
		case "succeeded":
			expected = task.Succeeded
		case "failed":
			expected = task.Failed
		case "cancelled":
			expected = task.Cancelled
		case "expired":
			expected = task.Expired
		default:
			return points.ErrInvariant
		}
		validError := e.ErrorCode == ""
		if expected == task.Failed {
			validError = task.ValidFailure(e.ErrorCode)
		} else if expected == task.Cancelled {
			validError = e.ErrorCode == "cancelled"
		} else if expected == task.Expired {
			validError = e.ErrorCode == "expired"
		}
		if e.Status != expected || !validError || e.Seq == 1 && e.Type != "created" || expected >= task.Succeeded && e.Seq != p.UpperSeq {
			return points.ErrInvariant
		}
		lastStep = e.Step
	}
	return nil
}

// Run 同步发送进度；nil事件表示保活注释，不推进任何游标。不产生后台goroutine。
func (s *AITaskStream) Run(send func(*dto.AITaskEvent) error) error {
	if s == nil || send == nil {
		return points.ErrInvalid
	}
	started := false
	s.runOnce.Do(func() { started = true })
	if !started {
		return points.ErrInvalid
	}
	defer s.Close()
	page := s.first
	for {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if s.terminal {
			return nil
		}
		for _, event := range page.Items {
			if err := s.ctx.Err(); err != nil {
				return err
			}
			if err := send(&event); err != nil {
				return err
			}
			s.after, s.lastStep = event.Seq, event.Step
			if event.Status >= task.Succeeded {
				return nil
			}
		}
		if !page.HasMore {
			if err := send(nil); err != nil {
				return err
			}
			timer := time.NewTimer(s.svc.period)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return s.ctx.Err()
			case <-timer.C:
			}
		}
		var err error
		page, err = s.read()
		if err != nil {
			return err
		}
	}
}
