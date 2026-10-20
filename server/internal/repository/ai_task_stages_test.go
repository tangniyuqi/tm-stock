package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestAITaskStageRepositoryGuards(t *testing.T) {
	var absent *AITaskRepo
	for _, r := range []*AITaskRepo{absent, NewAITaskRepo(nil)} {
		if err := r.WithinStageTx(context.Background(), func(task.StageTx) error { t.Fatal("空阶段仓储执行事务"); return nil }); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
		if err := r.WithinStageTx(context.Background(), nil); !errors.Is(err, points.ErrInvalid) {
			t.Fatal(err)
		}
	}
}
