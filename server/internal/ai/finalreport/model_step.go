package finalreport

import (
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

// ModelStep 只允许本包的封闭判定/编号选择计划进入同一单次调用协议。
// 外部实现不能注入自由提示、重试或默认通过策略。
type ModelStep interface {
	modelStep()
	Valid() bool
	InputDigest() string
	Config() CheckConfig
	MatchesTask(task.Task) bool
	OperationKey(task.Task, int) string
	Units() task.Units
	Request() provider.Request
	StageName() string
	StagePolicy() task.StagePolicy
	CapturePayload(provider.Response) ([]byte, error)
}

func (*CheckPlan) modelStep()        {}
func (*CheckPlan) StageName() string { return "judge" }
