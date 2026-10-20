package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestAIReportAdmissionClosedReads(t *testing.T) {
	for _, name := range []string{"missing", "between", "dependency"} {
		t.Run(name, func(t *testing.T) {
			s, st, ctl := reportReadFixture(t)
			hold, v, calls := st.p.hold, st.v, st.reviewCalls
			want := points.ErrInvariant
			if name == "dependency" {
				want = errPointsInjected
			}
			if name == "between" {
				ctl.hook = func() { st.admissionError = want }
			} else {
				st.admissionError = want
			}
			out, err := s.Read(context.Background(), v.UserID, v.ID)
			if !errors.Is(err, want) || out.TaskID != 0 || len(out.Sections) != 0 {
				t.Fatal("准入异常仍返回正文", out, err)
			}
			if st.p.hold != hold || st.v != v || st.reviewCalls != calls {
				t.Fatal("读取改变账务或重跑终检")
			}
			if available, err := s.Available(context.Background(), v.UserID, v.ID); available || err == nil {
				t.Fatal("准入故障伪装报告正常不可用")
			}
		})
	}
}

func TestAIReportAdmissionClosedFinalize(t *testing.T) {
	for _, name := range []string{"missing", "between", "replay"} {
		t.Run(name, func(t *testing.T) {
			s, st, r := reportFixture(t)
			if name == "replay" {
				if _, err := s.Finalize(context.Background(), r); err != nil {
					t.Fatal(err)
				}
			}
			hold, v, calls := st.p.hold, st.v, st.reviewCalls
			if name == "between" {
				st.reviewHook = func() { st.admissionError = points.ErrInvariant }
			} else {
				st.admissionError = points.ErrInvariant
			}
			out, err := s.Finalize(context.Background(), r)
			if !errors.Is(err, points.ErrInvariant) || out != (finalreport.Receipt{}) {
				t.Fatal("损坏准入仍终检/重放成功", out, err)
			}
			if st.p.hold != hold || st.v != v || name != "replay" && st.commit != nil || name != "between" && st.reviewCalls != calls {
				t.Fatal("准入故障仍改变报告或账务")
			}
			if name != "replay" && (st.v.Status != task.Running || st.p.hold.State != points.Holding) {
				t.Fatal("不明现场被结算或释放")
			}
		})
	}
}

func TestAIReportAdmissionClosedConversion(t *testing.T) {
	s, st, r, sel := conversionFixture(t)
	st.admissionError = points.ErrInvariant
	out, err := s.Convert(context.Background(), r, sel)
	if !errors.Is(err, points.ErrInvariant) || out.InputDigest != "" || len(out.Payload) != 0 || st.commit != nil || st.reviewCalls != 0 {
		t.Fatal("准入异常仍生成候选", out, err)
	}
}
