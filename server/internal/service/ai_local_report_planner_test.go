package service

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestLocalReportPlannerBindsOriginalTaskAndPlan(t *testing.T) {
	for _, kind := range []int{1, 2} {
		for _, mode := range []string{"正常", "会员", "任务", "任务号", "题材", "操作键", "模型", "预算", "计划撤回", "取消", "空上下文"} {
			t.Run(strconv.Itoa(kind)+"/"+mode, func(t *testing.T) {
				admission, st, _, path, _ := localAdmissionFixture(t)
				ctx := context.Background()
				v := st.v
				v.AnalysisType = kind
				original, err := admission.Plan(ctx, task.AdmissionScope{UserID: v.UserID, AnalysisType: kind, SubjectType: v.SubjectType, SubjectID: v.SubjectID})
				if err != nil {
					t.Fatal(err)
				}
				v.Budget, v.PriceVersion, v.PricePoints = original.Budget, original.Price.Version, original.Price.Points
				planner, err := NewAILocalReportPlanner(admission)
				if err != nil {
					t.Fatal(err)
				}
				plan, err := planner.Plan(ctx, v)
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "会员":
					plan.Agent.UserID++
				case "任务":
					plan.Agent.TaskID++
				case "任务号":
					plan.Agent.TaskNo += "other"
				case "题材":
					plan.Agent.SubjectID++
				case "操作键":
					plan.Agent.OperationKey = "other_operation_key"
				case "模型":
					plan.Model.Model = "仅测试:其它模型"
				case "预算":
					v.Budget.MaxTokens++
				case "计划撤回":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				case "取消":
					cancelled, cancel := context.WithCancel(ctx)
					cancel()
					ctx = cancelled
				case "空上下文":
					ctx = nil
				}
				if err := planner.Recheck(ctx, v, plan); (err == nil) != (mode == "正常") {
					t.Fatal("本地执行计划复核边界不符", err)
				}
			})
		}
	}
	if _, err := NewAILocalReportPlanner(nil); err == nil {
		t.Fatal("空依赖没有拒绝")
	}
}
