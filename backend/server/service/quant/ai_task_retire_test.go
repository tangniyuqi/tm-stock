package quant

import (
	"context"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
)

// 四类 AI 任务已全部下线：题材股票的 ai_add、ai_update_one、ai_update_batch（docs/specs/ai-analysis F1–F4），
// 以及基础股票的 ai_analyze（F15，决策 D16）。库里遗留的这类任务——尤其是"待调度"的定时任务——不能被
// 调度器再捡起来执行，也不能被"重启"，而要落成失败并给出原因，否则会每 30 秒被捞一次。

var retiredTaskTypes = []string{"ai_add", "ai_update_one", "ai_update_batch", "ai_analyze"}

// 下线名单必须与这里写死的四类完全一致：少一类意味着有一类 AI 任务重新可执行，多一类可能误伤将来的新类型。
func TestRetiredAiTaskTypeSetIsExactlyTheFourKnownTypes(t *testing.T) {
	if len(retiredAiTaskReasons) != len(retiredTaskTypes) {
		t.Fatalf("下线名单有 %d 项，期望 %d 项", len(retiredAiTaskReasons), len(retiredTaskTypes))
	}
	for _, typ := range retiredTaskTypes {
		if !isRetiredAiTaskType(typ) {
			t.Errorf("类型 %s 应在下线名单里", typ)
		}
		if retiredAiTaskReasons[typ] == "" {
			t.Errorf("类型 %s 缺少下线原因", typ)
		}
	}
	for _, typ := range []string{"", "no_such_type", "AI_ANALYZE", "ai_analyze "} {
		if isRetiredAiTaskType(typ) {
			t.Errorf("%q 不应被当作已下线类型（名单是精确匹配）", typ)
		}
	}
}

func TestRetiredAiTaskTypesAreNotRestartable(t *testing.T) {
	db := testutil.NewMemoryDB(t, &quant.QuantAiTask{})
	svc := &AiTaskService{}
	for _, typ := range retiredTaskTypes {
		task := quant.QuantAiTask{Type: typ, Name: "遗留任务", Status: AiTaskStatusFailed, Params: `{"theme_id":1}`}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		err := svc.RestartAiTask(context.Background(), task.ID, 1, true)
		if err == nil || !strings.Contains(err.Error(), "已下线") || !strings.Contains(err.Error(), typ) {
			t.Errorf("类型 %s 不应能被重启，报错应说明已下线：%v", typ, err)
		}
		var after quant.QuantAiTask
		db.First(&after, task.ID)
		if after.Status != AiTaskStatusFailed {
			t.Errorf("被拒绝的重启不应改变任务状态，实际 %d", after.Status)
		}
	}
	// 其他未知类型仍按原来的方式报错
	other := quant.QuantAiTask{Type: "no_such_type", Status: AiTaskStatusFailed, Params: `{}`}
	db.Create(&other)
	if err := svc.RestartAiTask(context.Background(), other.ID, 1, true); err == nil || !strings.Contains(err.Error(), "不支持的任务类型") {
		t.Errorf("未知类型应报不支持：%v", err)
	}
}

func TestRetiredAiTaskTypesAreFailedByTheScheduler(t *testing.T) {
	db := testutil.NewMemoryDB(t, &quant.QuantAiTask{})
	svc := &AiTaskService{}
	for _, typ := range retiredTaskTypes {
		task := quant.QuantAiTask{Type: typ, Name: "遗留定时任务", Status: AiTaskStatusPending, Params: `{"theme_id":1}`}
		task.CreatedBy = 1
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		svc.dispatchScheduledTask(task)
		var after quant.QuantAiTask
		if err := db.First(&after, task.ID).Error; err != nil {
			t.Fatal(err)
		}
		if after.Status != AiTaskStatusFailed || !strings.Contains(after.Error, "已下线") || after.FinishedAt == nil {
			t.Errorf("类型 %s 的遗留定时任务应被落成失败并说明原因：%+v", typ, after)
		}
	}
}

func TestErrAiTaskRetiredMessage(t *testing.T) {
	for _, typ := range retiredTaskTypes {
		err := errAiTaskRetired(typ)
		if err == nil || !strings.Contains(err.Error(), typ) || !strings.Contains(err.Error(), "已下线") {
			t.Fatalf("报错应点名任务类型并说明已下线：%v", err)
		}
		if !strings.Contains(err.Error(), retiredAiTaskReasons[typ]) {
			t.Errorf("类型 %s 的报错应带上各自的下线原因：%v", typ, err)
		}
	}
	// 题材个股与基础股票的下线原因不同，不能共用一句话
	if retiredAiTaskReasons["ai_add"] == retiredAiTaskReasons["ai_analyze"] {
		t.Error("题材个股与基础股票分析的下线原因应分别说明")
	}
	// 名单外的类型：调用方本不会这样用，但不能 panic，且报错仍带类型名
	if err := errAiTaskRetired("no_such_type"); err == nil || !strings.Contains(err.Error(), "no_such_type") {
		t.Errorf("名单外的类型也应返回带类型名的错误：%v", err)
	}
}
