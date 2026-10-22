package dto

import (
	"strings"
	"testing"
)

func TestAIQAInputValid(t *testing.T) {
	base := AIQAInput{Version: 1, SubjectType: "theme", SubjectID: 7, Skill: "material_qa", Question: "测试问题"}
	if !base.Valid() {
		t.Fatal("资料问答最小输入应通过")
	}
	report := base
	report.Skill, report.TaskID = "report_qa", 9
	if !report.Valid() {
		t.Fatal("报告追问带任务号应通过")
	}
	cases := map[string]func(*AIQAInput){
		"material_task": func(v *AIQAInput) { v.TaskID = 9 },
		"report_no_task": func(v *AIQAInput) {
			v.Skill = "report_qa"
		},
		"unknown_skill": func(v *AIQAInput) { v.Skill = "other" },
		"topic":         func(v *AIQAInput) { v.SubjectType = "topic" },
		"version":       func(v *AIQAInput) { v.Version = 0 },
		"subject":       func(v *AIQAInput) { v.SubjectID = 0 },
		"empty":         func(v *AIQAInput) { v.Question = "" },
		"trim":          func(v *AIQAInput) { v.Question = " 前后空白 " },
		"control":       func(v *AIQAInput) { v.Question = "含控制\u0000字符" },
		"too_long":      func(v *AIQAInput) { v.Question = strings.Repeat("字", 1001) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			value := base
			mutate(&value)
			if value.Valid() {
				t.Fatal("非法输入未被拒绝")
			}
		})
	}
}
