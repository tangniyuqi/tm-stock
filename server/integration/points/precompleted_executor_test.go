//go:build integration

package points_test

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
)

type precompletedExecutor struct {
	remote  string
	payload []byte
}

func (e precompletedExecutor) Submit(context.Context, string) (agents.Acceptance, error) {
	return agents.Acceptance{RemoteID: e.remote}, nil
}
func (e precompletedExecutor) Status(context.Context, string) (agents.Observation, error) {
	return agents.Observation{RemoteID: e.remote, State: agents.Completed}, nil
}
func (e precompletedExecutor) Result(context.Context, string) (agents.Result, error) {
	return agents.Result{RemoteID: e.remote, Payload: e.payload}, nil
}
func (e precompletedExecutor) Cancel(context.Context, string) error { return nil }
