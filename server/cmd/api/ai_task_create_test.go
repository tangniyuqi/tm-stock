package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// 启动反例控制器：不能提供任何可创建的计划，也不是生产准入。
type createStartupDeny struct{ calls int }

func (d *createStartupDeny) Ready(context.Context) error {
	d.calls++
	return task.ErrAdmissionUnavailable
}
func (*createStartupDeny) Plan(context.Context, task.AdmissionScope) (task.AdmissionPlan, error) {
	return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
}
func (*createStartupDeny) Recheck(context.Context, task.AdmissionPlan) error {
	return task.ErrAdmissionUnavailable
}

func TestAITaskCreateStartupIsClosedWithoutRealAdmission(t *testing.T) {
	ctx := context.Background()
	off := config.AITaskReadConfig{}
	good := config.AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	if s, err := prepareAITaskCreates(ctx, nil, off, false, nil); err != nil || s != nil {
		t.Fatal(s, err)
	}
	if _, err := prepareAITaskCreates(ctx, nil, good, true, nil); err == nil {
		t.Fatal("无真实控制器仍启用创建")
	}
	if _, err := prepareAITaskCreates(ctx, nil, off, true, nil); err == nil {
		t.Fatal("没有读取身份仍启用创建")
	}
	if _, err := prepareAITaskCreates(ctx, nil, config.AITaskReadConfig{Enabled: true}, false, nil); err == nil {
		t.Fatal("无效JWT装配")
	}
	s, err := prepareAITaskCreates(ctx, nil, good, false, nil)
	if err != nil || s == nil {
		t.Fatal(s, err)
	}
	mux := http.NewServeMux()
	if err := registerAITaskCreates(mux, nil, good, s, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/ai/tasks", "/api/v1/ai/tasks/quote"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 404 {
			t.Fatal("默认开放新建")
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/task-receipt", nil))
	if w.Code != 401 {
		t.Fatal("原键核查不要求身份")
	}
	if err := registerAITaskCreates(http.NewServeMux(), nil, off, s, false); err == nil {
		t.Fatal("没有读取开关仍注册")
	}
	if err := registerAITaskCreates(http.NewServeMux(), nil, off, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := registerAITaskCreates(http.NewServeMux(), nil, good, nil, true); err == nil {
		t.Fatal("空服务仍启用")
	}
	db, err := sql.Open("mysql", "tcp(127.0.0.1:1)/test?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deny := &createStartupDeny{}
	if _, err := prepareAITaskCreates(ctx, db, good, true, deny); !errors.Is(err, task.ErrAdmissionUnavailable) || deny.calls != 1 {
		t.Fatal("启动准入未调用", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := prepareAITaskCreates(cancelled, db, good, true, deny); !errors.Is(err, context.Canceled) || deny.calls != 1 {
		t.Fatal("父请求失效仍查批准", err)
	}
	if db.Stats().OpenConnections != 0 {
		t.Fatal("前置拒绝仍访问了数据库")
	}
}
