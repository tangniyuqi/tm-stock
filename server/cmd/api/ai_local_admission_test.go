package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
)

func TestLocalAdmissionRequiresExplicitIsolatedAssembly(t *testing.T) {
	t.Setenv("TM_AI_LOCAL_PLANS", "")
	if control, err := prepareAILocalAdmission(nil, nil); err != nil || control != nil {
		t.Fatal("无文件不应构造控制器")
	}
	t.Setenv("TM_AI_LOCAL_PLANS", "forbidden_private_plan_path")
	if _, err := prepareAILocalAdmission(nil, nil); err == nil || strings.Contains(err.Error(), os.Getenv("TM_AI_LOCAL_PLANS")) {
		t.Fatal("缺隔离配置未拒绝或泄漏路径")
	}
}

func TestLocalReportReadsFailClosedWithoutBoundControl(t *testing.T) {
	if reader, err := prepareAILocalReportReads(nil, nil, config.AITaskReadConfig{}, false, nil); err != nil || reader != nil {
		t.Fatal("关闭读取时不应装配依赖")
	}
	if _, err := prepareAILocalReportReads(context.Background(), nil, config.AITaskReadConfig{}, true, nil); err == nil {
		t.Fatal("缺同源本地控制器仍启用报告读取")
	}
}
