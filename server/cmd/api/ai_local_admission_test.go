package main

import (
	"os"
	"strings"
	"testing"
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
