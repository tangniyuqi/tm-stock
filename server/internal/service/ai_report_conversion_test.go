package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func conversionFixture(t *testing.T) (*AIReportService, *reportScript, task.StageRequest, *finalreport.SelectionStage) {
	t.Helper()
	s, st, old := reportFixture(t)
	sel, err := finalreport.NewSelectionStage(s.policy, st.v.No, st.v.SubjectID, st.theme, st.snaps)
	if err != nil {
		t.Fatal(err)
	}
	r := old.Stage
	r.Stage = "extract"
	r.InputDigest = sel.InputDigest()
	in := finalreport.SelectionResponse{}
	for i, key := range []render.SectionKey{render.SecDefinition, render.SecTimeline, render.SecSegment, render.SecCompanies} {
		in.Selections = append(in.Selections, finalreport.Selection{Section: key, SnapshotID: 1, SentenceIDs: []int{i + 1}})
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	// 仅单测夹具替换原先的 judge；真库用例从空任务按正式 Capture/Complete 写入。
	st.rows[0].InputDigest = r.InputDigest
	res := st.results[1]
	res.Stage = r.Stage
	res.InputDigest = r.InputDigest
	res.Payload = raw
	res.OutputDigest = task.StageDigest(raw)
	st.results[1] = res
	cp := st.steps[1]
	cp.Stage = r.Stage
	cp.InputDigest = r.InputDigest
	cp.OutputDigest = res.OutputDigest
	cp.PolicyDigest = sel.Policy().Digest
	st.steps[1] = cp
	return s, st, r, sel
}

func TestAIReportConversionReceiptAndReplay(t *testing.T) {
	s, st, r, sel := conversionFixture(t)
	got, err := s.Convert(context.Background(), r, sel)
	if err != nil || !finalreport.ValidDigest(got.InputDigest) || len(got.Payload) == 0 {
		t.Fatal(got, err)
	}
	if _, err := s.policy.Decode(string(got.Payload)); err != nil {
		t.Fatal(err)
	}
	again, err := s.Convert(context.Background(), r, sel)
	if err != nil || got.InputDigest != again.InputDigest || string(got.Payload) != string(again.Payload) {
		t.Fatal("重放不稳定", err)
	}
	if st.reviewCalls != 0 || st.commit != nil || st.v.Status != task.Running || st.v.CurrentStep != 1 || st.p.hold.State != points.Holding || st.state.Count != 1 {
		t.Fatal("转换越权调用终检、冻结或结算")
	}
	out, err := json.Marshal(got)
	if err != nil || strings.Contains(string(out), st.theme) || strings.Contains(fmt.Sprintf("%+v %#v", got, got), st.theme) {
		t.Fatal("候选正文外泄", err)
	}
}

func TestAIReportConversionFailClosed(t *testing.T) {
	for name, change := range map[string]func(*AIReportService, *reportScript, *task.StageRequest){
		"关闭":  func(s *AIReportService, _ *reportScript, _ *task.StageRequest) { s.enabled = false },
		"旧围栏": func(_ *AIReportService, _ *reportScript, r *task.StageRequest) { r.Fence++ },
		"错会员": func(_ *AIReportService, _ *reportScript, r *task.StageRequest) { r.UserID++ },
		"输入": func(_ *AIReportService, _ *reportScript, r *task.StageRequest) {
			r.InputDigest = strings.Repeat("f", 64)
		},
		"步骤": func(_ *AIReportService, _ *reportScript, r *task.StageRequest) { r.Stage = "judge" },
		"取消": func(_ *AIReportService, st *reportScript, _ *task.StageRequest) { st.v.CancelRequested = true },
		"来源撤回": func(_ *AIReportService, st *reportScript, _ *task.StageRequest) {
			st.evidenceError = sources.ErrUnauthorized
		},
		"来源归档变更": func(_ *AIReportService, st *reportScript, _ *task.StageRequest) { st.snaps[0].LicenseRef += "-changed" },
		"题材变更":   func(_ *AIReportService, st *reportScript, _ *task.StageRequest) { st.theme = "另一示例题材" },
		"任务范围":   func(_ *AIReportService, st *reportScript, _ *task.StageRequest) { st.v.SubjectID++ },
		"未完成":    func(_ *AIReportService, st *reportScript, _ *task.StageRequest) { st.v.CurrentStep = 0 },
		"已有后续步骤": func(_ *AIReportService, st *reportScript, _ *task.StageRequest) { st.v.CurrentStep = 2 },
		"响应篡改": func(_ *AIReportService, st *reportScript, _ *task.StageRequest) {
			v := st.results[1]
			v.Payload = []byte(`{}`)
			st.results[1] = v
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, st, r, sel := conversionFixture(t)
			change(s, st, &r)
			got, err := s.Convert(context.Background(), r, sel)
			if err == nil || got.InputDigest != "" || len(got.Payload) != 0 || st.commit != nil {
				t.Fatal("边界未关闭", got, err)
			}
			if name == "关闭" && st.p.calls != 0 {
				t.Fatal("关闭仍读DB")
			}
		})
	}
	s, _, r, sel := conversionFixture(t)
	var empty *AIReportService
	if _, err := empty.Convert(context.Background(), r, sel); err == nil {
		t.Fatal("空服务未拒绝")
	}
	if _, err := s.Convert(context.Background(), r, nil); err == nil {
		t.Fatal("空策略未拒绝")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := s.Convert(ctx, r, sel); !errors.Is(err, context.Canceled) || len(got.Payload) != 0 {
		t.Fatal(got, err)
	}
}

func TestAIReportConversionStorageFaults(t *testing.T) {
	s, baseline, r, sel := conversionFixture(t)
	if _, err := s.Convert(context.Background(), r, sel); err != nil {
		t.Fatal(err)
	}
	for fail := 1; fail <= baseline.p.calls; fail++ {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			s, st, r, sel := conversionFixture(t)
			st.p.failAt = fail
			out, err := s.Convert(context.Background(), r, sel)
			if !errors.Is(err, errPointsInjected) || out.InputDigest != "" || len(out.Payload) != 0 {
				t.Fatal("存储故障泄露候选", err)
			}
		})
	}
	for phase := 1; phase <= 2; phase++ {
		s, st, r, sel := conversionFixture(t)
		st.failCommitAt = phase
		if got, err := s.Convert(context.Background(), r, sel); !errors.Is(err, errPointsInjected) || len(got.Payload) != 0 {
			t.Fatal("事务退出未知泄露候选", err)
		}
	}
}
