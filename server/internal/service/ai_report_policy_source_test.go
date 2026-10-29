package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type reportPolicySourceFixture struct {
	st    *reportScript
	value AIReportReadPolicy
	err   error
	calls int
}

func (s *reportPolicySourceFixture) Resolve(ctx context.Context, _, _ int64) (AIReportReadPolicy, error) {
	s.calls++
	if s.st.inTx {
		return AIReportReadPolicy{}, points.ErrInvariant
	}
	if err := ctx.Err(); err != nil {
		return AIReportReadPolicy{}, err
	}
	return s.value, s.err
}

func TestReportPolicySourceKeepsOriginalReadChecks(t *testing.T) {
	for _, mode := range []string{"正常", "会员", "任务", "题材", "类型", "空策略", "依赖", "终检损坏", "结算损坏", "关闭"} {
		t.Run(mode, func(t *testing.T) {
			old, st, control := reportReadFixture(t)
			source := &reportPolicySourceFixture{st: st, value: AIReportReadPolicy{UserID: st.v.UserID, TaskID: st.v.ID, SubjectID: st.v.SubjectID, AnalysisType: st.v.AnalysisType, Policy: old.base.policy}}
			reader, err := NewAIReportReadServiceWithPolicySource(old.base, source, control, mode != "关闭")
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "会员":
				source.value.UserID++
			case "任务":
				source.value.TaskID++
			case "题材":
				source.value.SubjectID++
			case "类型":
				source.value.AnalysisType = 2
			case "空策略":
				source.value.Policy = nil
			case "依赖":
				source.err = errPointsInjected
			case "终检损坏":
				st.commit.PolicyDigest = "invalid"
			case "结算损坏":
				st.pt.terminal.Hash = "invalid"
			}
			beforeTask, beforeHold, beforeReview := st.v, st.p.hold, st.reviewCalls
			out, err := reader.Read(context.Background(), st.v.UserID, st.v.ID)
			if (err == nil) != (mode == "正常") {
				t.Fatal("策略源或原读取保护边界不符", err)
			}
			if mode == "正常" && (out.TaskID != st.v.ID || len(out.Sections) != 4 || source.calls != 2 || control.calls != 2) {
				t.Fatal("动态策略未按原两次复核交付")
			}
			if mode == "关闭" && (!errors.Is(err, finalreport.ErrDisabled) || source.calls != 0) {
				t.Fatal("关闭时仍解析策略")
			}
			if st.v != beforeTask || st.p.hold != beforeHold || st.reviewCalls != beforeReview {
				t.Fatal("只读解析改变任务/账务或调用模型")
			}
		})
	}
	if _, err := NewAIReportReadServiceWithPolicySource(nil, nil, nil, true); err == nil {
		t.Fatal("空依赖未拒绝")
	}
	base, _, _ := reportFixture(t)
	if _, err := NewAIReportReadServiceWithPolicySource(base, nil, nil, true); err == nil {
		t.Fatal("缺策略/发布控制器未拒绝")
	}
}
