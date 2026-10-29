package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type reportReadDenyTest struct{}

func (reportReadDenyTest) Recheck(context.Context, service.AIReportPublication) error {
	return finalreport.ErrUnavailable
}

func startupTestReportPolicy(t *testing.T, deep bool) *finalreport.Policy {
	t.Helper()
	c := finalreport.Config{Ref: "test-only-policy", ModelName: "测试模型", FilingNo: "test-only-filing", EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{"evidence.example.test"}}
	if deep {
		c.AnalysisType, c.MinQuotes = 2, 6
	}
	p, err := finalreport.NewPolicy(c, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAIReportReadDeepStartupRejectsWrongPolicy(t *testing.T) {
	good := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	light, deep := startupTestReportPolicy(t, false), startupTestReportPolicy(t, true)
	for _, policies := range [][2]*finalreport.Policy{{light, light}, {deep, deep}, {light, {}}, {nil, deep}} {
		if r, err := prepareAIReportReadsWithDeep(context.Background(), nil, good, true, policies[0], policies[1], reportReadDenyTest{}); r != nil || !errors.Is(err, memberauth.ErrInvalidConfig) {
			t.Fatal("错误策略组合未在连接数据库前拒绝", err)
		}
	}
	if r, err := prepareAIReportReadsWithDeep(nil, nil, good, false, light, deep, nil); r != nil || err != nil {
		t.Fatal("关闭读取仍装配", err)
	}
	if r, err := prepareAIReportReadsWithDeep(nil, nil, good, true, light, deep, reportReadDenyTest{}); r != nil || err == nil {
		t.Fatal("缺上下文或数据库仍装配")
	}
}

func TestAIReportReadStartupIsExplicitAndClosed(t *testing.T) {
	good := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	if _, err := prepareAIReportReads(context.Background(), nil, good, true, nil, nil); err == nil {
		t.Fatal("仅身份配置放行报告")
	}
	r, err := prepareAIReportReads(context.Background(), nil, config.AITaskReadConfig{}, false, nil, nil)
	if err != nil || r != nil {
		t.Fatal(r, err)
	}
	if _, err := prepareAIReportReads(context.Background(), nil, config.AITaskReadConfig{Enabled: true}, true, nil, nil); err == nil {
		t.Fatal("缺真实发布证明仍装配")
	}
	mux := http.NewServeMux()
	if err := registerAIReportReads(mux, nil, config.AITaskReadConfig{}, nil); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/tasks/1/report", nil))
	if w.Code != 404 {
		t.Fatal("默认开放报告")
	}
}
