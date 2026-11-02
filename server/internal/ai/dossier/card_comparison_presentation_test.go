package dossier

import (
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"testing"
)

func TestReportComparisonAxesProjection(t *testing.T) {
	p := comparisonPlan(t)
	r, _ := comparisonReview(t, p, nil)
	before, err := p.VisibleReview(r)
	if err != nil {
		t.Fatal(err)
	}
	axes, err := p.ReviewedAxes(r)
	if err != nil || len(axes) != 7 {
		t.Fatal("合法轴投影失败", err)
	}
	for i, axis := range axes {
		if axis.Required != p.axes[i].Required || axis.CardComparisonAxis != r.axes[i] {
			t.Fatal("投影脱离原轴")
		}
	}
	axes[0].Field = "外部修改"
	again, err := p.ReviewedAxes(r)
	if err != nil || again[0].Field == axes[0].Field {
		t.Fatal("投影别名污染", err)
	}
	after, err := p.VisibleReview(r)
	if err != nil || before != after {
		t.Fatal("投影改变原语义终检呈现", err)
	}
	if out, err := p.ReviewedAxes(nil); err == nil || out != nil {
		t.Fatal("缺回执仍投影")
	}
	var absent *CardComparisonPlan
	if out, err := absent.ReviewedAxes(r); err == nil || out != nil {
		t.Fatal("缺计划仍投影")
	}
	changed := *p
	changed.digest = task.StageDigest([]byte("foreign-comparison-presentation"))
	if out, err := changed.ReviewedAxes(r); err == nil || out != nil {
		t.Fatal("跨输入回执仍投影")
	}
}
