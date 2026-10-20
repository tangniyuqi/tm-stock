package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

// AIAgentService 是内部执行链路，不接会员积分、不发布上游正文、不替换 L0 证据审核。
type AIAgentService struct {
	store    agents.Store
	executor agents.Executor
	profile  agents.Profile
}

func NewAIAgentService(store agents.Store, executor agents.Executor, profile agents.Profile) (*AIAgentService, error) {
	if store == nil || executor != nil && !profile.Valid() {
		return nil, agents.ErrInvalid
	}
	return &AIAgentService{store: store, executor: executor, profile: profile}, nil
}

func (s *AIAgentService) Show(ctx context.Context, actorID int64, requestID string) (agents.Record, error) {
	if actorID <= 0 || !agents.ValidRequestID(requestID) {
		return agents.Record{}, agents.ErrInvalid
	}
	r, err := s.store.ByRequest(ctx, requestID)
	if err != nil {
		return agents.Record{}, err
	}
	if !r.Valid() {
		return agents.Record{}, agents.ErrInvalid
	}
	if r.ActorID != actorID {
		return agents.Record{}, agents.ErrNotFound
	}
	return r, nil
}

func (s *AIAgentService) Submit(ctx context.Context, actorID int64, req agents.Request) (agents.Record, error) {
	if actorID <= 0 || !req.Valid() {
		return agents.Record{}, agents.ErrInvalid
	}
	if s.executor == nil {
		return agents.Record{}, agents.ErrDisabled
	}
	var record agents.Record
	var started bool
	err := s.store.WithinAgentTx(ctx, func(tx agents.Tx) error {
		var err error
		record, started, err = tx.Reserve(ctx, s.profile, req, actorID)
		return err
	})
	if err != nil {
		return agents.Record{}, err
	}
	if !started {
		if record.State == agents.Submitting || record.State == agents.SubmissionUnknown {
			return record, agents.ErrUncertain
		}
		return record, nil
	}
	accepted, callErr := s.executor.Submit(ctx, req.Symbol)
	// 不自动重试提交。400 也可能是上游在入队后的异常，并非“确定未执行”。
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	confirmed := record
	err = s.store.WithinAgentTx(finishCtx, func(tx agents.Tx) error {
		r, err := tx.Lock(finishCtx, record.ID)
		if err != nil {
			return err
		}
		event := "submitted"
		if callErr != nil || !agents.ValidRemoteID(accepted.RemoteID) {
			r.State = agents.SubmissionUnknown
			r.ErrorCode = "submission_unknown"
			event = "submission_unknown"
		} else {
			r.RemoteID = accepted.RemoteID
			r.State = agents.Accepted
		}
		if err := tx.Save(finishCtx, r, event, nil); err != nil {
			return err
		}
		confirmed = r
		return nil
	})
	if err != nil {
		return record, agents.ErrUncertain
	}
	record = confirmed
	if record.State == agents.SubmissionUnknown {
		return record, agents.ErrUncertain
	}
	return record, nil
}

func (s *AIAgentService) executable(ctx context.Context, actorID int64, requestID string) (agents.Record, error) {
	r, err := s.Show(ctx, actorID, requestID)
	if err != nil {
		return r, err
	}
	if s.executor == nil {
		return r, agents.ErrDisabled
	}
	if r.ProfileDigest != s.profile.Digest {
		return r, agents.ErrConflict
	}
	if r.RemoteID == "" {
		return r, agents.ErrUncertain
	}
	return r, nil
}

func (s *AIAgentService) Status(ctx context.Context, actorID int64, requestID string) (agents.Record, error) {
	r, err := s.executable(ctx, actorID, requestID)
	if err != nil {
		return r, err
	}
	if agents.Terminal(r.State) {
		return r, nil
	}
	observation, err := s.executor.Status(ctx, r.RemoteID)
	if err != nil {
		return r, err
	}
	if observation.RemoteID != r.RemoteID || !agents.ValidRemoteState(observation.State) {
		return r, agents.ErrInvalid
	}
	confirmed := r
	err = s.store.WithinAgentTx(ctx, func(tx agents.Tx) error {
		latest, err := tx.Lock(ctx, r.ID)
		if err != nil {
			return err
		}
		if agents.Terminal(latest.State) || latest.State == agents.Running && observation.State == agents.Accepted {
			confirmed = latest
			return nil
		}
		latest.State = observation.State
		if err := tx.Save(ctx, latest, "observed", nil); err != nil {
			return err
		}
		confirmed = latest
		return nil
	})
	if err == nil {
		r = confirmed
	}
	return r, err
}

func (s *AIAgentService) Result(ctx context.Context, actorID int64, requestID string) (agents.Record, error) {
	r, err := s.executable(ctx, actorID, requestID)
	if err != nil {
		return r, err
	}
	if r.State == agents.Quarantined {
		return r, nil
	}
	if r.State != agents.Completed {
		return r, agents.ErrConflict
	}
	result, err := s.executor.Result(ctx, r.RemoteID)
	if err != nil {
		return r, err
	}
	if !result.Valid() || result.RemoteID != r.RemoteID {
		return r, agents.ErrInvalid
	}
	confirmed := r
	err = s.store.WithinAgentTx(ctx, func(tx agents.Tx) error {
		latest, err := tx.Lock(ctx, r.ID)
		if err != nil {
			return err
		}
		if latest.State == agents.Quarantined {
			confirmed = latest
			return nil
		}
		latest.State = agents.Quarantined
		latest.ResultDigest = result.Digest()
		if err := tx.Save(ctx, latest, "quarantined", &result); err != nil {
			return err
		}
		confirmed = latest
		return nil
	})
	if err == nil {
		r = confirmed
	}
	return r, err
}

func (s *AIAgentService) Cancel(ctx context.Context, actorID int64, requestID string) (agents.Record, error) {
	r, err := s.executable(ctx, actorID, requestID)
	if err != nil {
		return r, err
	}
	started := false
	confirmed := r
	err = s.store.WithinAgentTx(ctx, func(tx agents.Tx) error {
		latest, err := tx.Lock(ctx, r.ID)
		if err != nil {
			return err
		}
		if latest.CancelRequested || agents.Terminal(latest.State) {
			confirmed = latest
			return nil
		}
		latest.CancelRequested = true
		latest.ErrorCode = "cancel_pending"
		if err := tx.Save(ctx, latest, "cancel_requested", nil); err != nil {
			return err
		}
		confirmed = latest
		started = true
		return nil
	})
	if err == nil {
		r = confirmed
	}
	if err != nil || !started {
		if err == nil && (r.ErrorCode == "cancel_pending" || r.ErrorCode == "cancel_unknown") {
			err = agents.ErrUncertain
		}
		return r, err
	}
	callErr := s.executor.Cancel(ctx, r.RemoteID)
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err = s.store.WithinAgentTx(finishCtx, func(tx agents.Tx) error {
		latest, err := tx.Lock(finishCtx, r.ID)
		if err != nil {
			return err
		}
		event := "cancel_acknowledged"
		latest.ErrorCode = ""
		if callErr != nil {
			event = "cancel_unknown"
			latest.ErrorCode = event
		}
		if err := tx.Save(finishCtx, latest, event, nil); err != nil {
			return err
		}
		confirmed = latest
		return nil
	})
	if err != nil || callErr != nil {
		if err == nil {
			r = confirmed
		}
		return r, agents.ErrUncertain
	}
	return confirmed, nil
}
