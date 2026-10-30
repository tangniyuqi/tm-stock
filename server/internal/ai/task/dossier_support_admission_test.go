package task

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestDossierSupportAdmissionEncoding(t *testing.T) {
	// 冻结新增字段之前的v1线上编码形状；不能因可选组件使历史摘要全部改变。
	legacy := []byte(fmt.Sprintf(`{"Version":"task-admission-v1","Plan":{"Scope":{"UserID":1,"AnalysisType":1,"SubjectType":"theme","SubjectID":2},"Price":{"Version":"test-v1","Points":7},"Budget":{"max_attempts":2,"max_calls":5,"max_tokens":1000,"max_pages":3,"max_duration_ms":60000},"SubjectName":"示例题材","ModelName":"测试模型","FilingNo":"test-only-filing","ProfileDigest":"%s","ExecutionDigest":"29180c2db580b7c7c2196336580266cbef9f685eab03d86b4d678d0cb5540db8","ApprovalRef":"test-only-model","QualityRef":"test-only-quality","HardBudgetRef":"test-only-budget","PriceRef":"test-only-price","SourceRef":"test-only-source","Materials":[{"SnapshotID":1,"Digest":"%s","Domain":"evidence.example.test","Category":"announcement","URL":"https://evidence.example.test/test","CollectedAt":"2026-10-01T00:00:00Z"}]}}`, strings.Repeat("a", 64), strings.Repeat("b", 64)))
	p, err := DecodeAdmission(legacy)
	if err != nil || p.DossierSupportDigest != "" {
		t.Fatal("原v1记录不能读取", err)
	}
	got, digest, err := admissionTestPlan().Encode()
	if err != nil || !bytes.Equal(got, legacy) || digest != StageDigest(legacy) {
		t.Fatal("空组件改变旧编码或摘要", err)
	}
	for _, component := range []string{"", strings.Repeat("d", 64), "wrong", strings.Repeat("D", 64), strings.Repeat("d", 63)} {
		name := "empty"
		if component != "" {
			name = fmt.Sprint("length_", len(component), "_", component[0])
		}
		t.Run(name, func(t *testing.T) {
			p := admissionTestPlan()
			p.DossierSupportDigest = component
			raw, hash, err := p.Encode()
			valid := component == "" || component == strings.Repeat("d", 64)
			if !valid {
				if err == nil || raw != nil || hash != "" {
					t.Fatal("坏核验组件进入准入")
				}
				return
			}
			decoded, err := DecodeAdmission(raw)
			if err != nil || decoded.DossierSupportDigest != component || component != "" && hash == digest {
				t.Fatal("核验组件未持久绑定", err)
			}
		})
	}
	// 变更任何组件都须使原不可变记录失效，不能只改内存Plan绕过摘要。
	r, v, _ := admissionRecordFixture(t)
	r.Plan.DossierSupportDigest = strings.Repeat("d", 64)
	if r.Bind(v) == nil {
		t.Fatal("新增组件沿用旧记录摘要")
	}
	_, r.Digest, err = r.Plan.Encode()
	if err != nil || r.Bind(v) != nil {
		t.Fatal("合法新记录不能绑定", err)
	}
}
