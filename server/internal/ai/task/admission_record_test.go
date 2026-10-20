package task

import (
	"bytes"
	"fmt"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"strings"
	"testing"
	"time"
)

func admissionRecordFixture(t *testing.T) (AdmissionRecord, Task, []byte) {
	t.Helper()
	p := admissionTestPlan()
	b, d, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	at := p.Materials[0].CollectedAt.Add(time.Hour)
	v := Task{ID: 3, UserID: p.Scope.UserID, No: strings.Repeat("a", 32), Hash: strings.Repeat("b", 64), AnalysisType: 1, SubjectType: Theme, SubjectID: p.Scope.SubjectID, PriceVersion: p.Price.Version, PricePoints: p.Price.Points, Budget: p.Budget, Deadline: at.Add(time.Duration(p.Budget.MaxDurationMS) * time.Millisecond)}
	r := AdmissionRecord{TaskID: v.ID, UserID: v.UserID, TaskNo: v.No, RequestHash: v.Hash, Digest: d, Plan: p, CreatedAt: at}
	return r, v, b
}
func TestAdmissionRecordCanonicalAndBinding(t *testing.T) {
	for _, s := range []string{"", strings.Repeat("g", 32), strings.Repeat("A", 32)} {
		if regexpTaskNo(s) {
			t.Fatal("任务号格式放行")
		}
	}
	r, v, b := admissionRecordFixture(t)
	if r.Bind(v) != nil {
		t.Fatal("有效绑定失败")
	}
	p, err := DecodeAdmission(b)
	if err != nil {
		t.Fatal(err)
	}
	a, d, err := p.Encode()
	if err != nil || !bytes.Equal(a, b) || d != r.Digest {
		t.Fatal("规范编码不稳定")
	}
	p.Materials[0].CollectedAt = p.Materials[0].CollectedAt.In(time.FixedZone("测试东八区", 8*3600))
	a, _, err = p.Encode()
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("等价时点编码不一致")
	}
	for _, raw := range [][]byte{nil, []byte(`[]`), append(append([]byte{}, b...), ' '), []byte(strings.Replace(string(b), `"Version":`, `"unexpected":0,"Version":`, 1)), []byte(strings.Replace(string(b), AdmissionVersion, "unknown-version", 1)), append([]byte{'{', '}'}, b...), []byte(strings.Repeat("x", MaxAdmissionBytes+1))} {
		if _, err := DecodeAdmission(raw); err == nil {
			t.Fatal("非规范记录被接受")
		}
	}
	for _, change := range []func(*AdmissionRecord){func(r *AdmissionRecord) { r.TaskID++ }, func(r *AdmissionRecord) { r.UserID++ }, func(r *AdmissionRecord) { r.TaskNo = "bad" }, func(r *AdmissionRecord) { r.RequestHash = "bad" }, func(r *AdmissionRecord) { r.Digest = "bad" }, func(r *AdmissionRecord) { r.Plan = r.Plan.Clone(); r.Plan.QualityRef = "changed" }, func(r *AdmissionRecord) { r.CreatedAt = r.CreatedAt.Add(time.Millisecond) }, func(r *AdmissionRecord) { r.CreatedAt = time.Time{} }} {
		bad := r
		change(&bad)
		if bad.Bind(v) == nil {
			t.Fatal("损坏准入绑定被接受")
		}
	}
	for _, change := range []func(*Task){func(v *Task) { v.ID = 0 }, func(v *Task) { v.No = strings.Repeat("g", 32) }, func(v *Task) { v.Hash = "bad" }, func(v *Task) { v.SubjectID++ }, func(v *Task) { v.Budget.MaxCalls++ }, func(v *Task) { v.PricePoints++ }, func(v *Task) { v.Deadline = v.Deadline.Add(time.Second) }} {
		bad := v
		change(&bad)
		if r.Bind(bad) == nil {
			t.Fatal("任务换身份或额度被接受")
		}
	}
	bad := r
	bad.Plan = bad.Plan.Clone()
	bad.Plan.Materials[0].CollectedAt = r.CreatedAt.Add(time.Second)
	_, bad.Digest, _ = bad.Plan.Encode()
	if bad.Bind(v) == nil {
		t.Fatal("未来资料被接受")
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", r, r), r.Plan.SourceRef) {
		t.Fatal("格式化泄漏批准引用")
	}
	p.ExecutionDigest = ""
	if _, _, err := p.Encode(); err != points.ErrInvalid {
		t.Fatal(err)
	}
}
