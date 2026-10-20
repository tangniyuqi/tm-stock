package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReadRequestIsClosedAndBounded(t *testing.T) {
	for _, v := range []string{`{"request_id":"test_run_01","symbol":"600001"}`, `{"symbol":"600001","request_id":"test_run_01"}`} {
		if _, err := readRequest(strings.NewReader(v), true); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{`{}`, `[]`, `{"Request_ID":"test_run_01","symbol":"600001"}`, `{"request_id":"test_run_01","request_id":"test_run_02","symbol":"600001"}`, `{"request_id":"test_run_01","symbol":"600001","user_id":1}`, `{"request_id":"test_run_01","symbol":600001}`, `{"request_id":"test_run_01","symbol":"600001"}{}`, strings.Repeat(" ", 1025), string([]byte{0xff}), `{"request_id":"test_run_01"`} {
		if _, err := readRequest(strings.NewReader(v), true); err == nil {
			t.Fatalf("非法输入放行：%.80s", v)
		}
	}
	if _, err := readRequest(strings.NewReader(`{"request_id":"test_run_01"}`), false); err != nil {
		t.Fatal(err)
	}
	if _, err := readRequest(strings.NewReader(`{"request_id":"test_run_01","symbol":"600001"}`), false); err == nil {
		t.Fatal("查询接受了多余字段")
	}
}

func TestCommandGatesBeforeNetworkOrDB(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"show", "extra"}} {
		if run(args, strings.NewReader(`{}`), &bytes.Buffer{}) == nil {
			t.Fatal("非法命令放行")
		}
	}
	t.Setenv("TM_AI_AGENTS_ACTOR_ID", "")
	if run([]string{"show"}, strings.NewReader(`{"request_id":"test_run_01"}`), &bytes.Buffer{}) == nil {
		t.Fatal("未绑定操作人")
	}
	t.Setenv("TM_AI_AGENTS_ACTOR_ID", "7")
	t.Setenv("TM_AI_AGENTS_ENABLED", "false")
	if run([]string{"submit"}, strings.NewReader(`{"request_id":"test_run_01","symbol":"600001"}`), &bytes.Buffer{}) == nil {
		t.Fatal("关闭开关仍可提交")
	}
}

type unreadVersionInput struct{ t *testing.T }

func (r unreadVersionInput) Read([]byte) (int, error) {
	r.t.Fatal("版本描述不应等待或读取标准输入")
	return 0, io.EOF
}

type failedVersionOutput struct{}

func (failedVersionOutput) Write([]byte) (int, error) {
	return 0, errors.New("test-only:禁止回显写入器原文")
}

func TestVersionDoesNotLoadConfigReadInputOrCallServices(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer remote.Close()
	// 即使显式开启或配置损坏，version 也只返回适配器参照，不探测服务或数据库。
	for _, enabled := range []string{"", "false", "true", "bad-switch"} {
		t.Run("开关_"+enabled, func(t *testing.T) {
			t.Setenv("TM_AI_AGENTS_ENABLED", enabled)
			t.Setenv("TM_AI_AGENTS_URL", remote.URL)
			t.Setenv("TM_DB_DSN", "test-only:invalid-dsn")
			t.Setenv("TM_AI_AGENTS_ACTOR_ID", "not-an-actor")
			t.Setenv("TM_AI_AGENTS_TOKEN", "test-only:禁止回显凭据")
			var out bytes.Buffer
			if err := run([]string{"version"}, unreadVersionInput{t}, &out); err != nil {
				t.Fatal(err)
			}
			const want = `{"scope":"adapter_baseline","edition":"community","commit":"51060a7682cf51810b4bc3a704829438d18cd8dc","contract":"v2_async_analysis","software_activation_required":false,"service_authentication_required":true,"remote_version_verified":false}`
			if out.String() != want+"\n" || !json.Valid(out.Bytes()) || calls.Load() != 0 {
				t.Fatal("版本命令改变执行边界或输出不符合闭集契约")
			}
		})
	}
	for _, args := range [][]string{{"version", "extra"}, {"Version"}} {
		var out bytes.Buffer
		if run(args, unreadVersionInput{t}, &out) == nil || out.Len() != 0 {
			t.Fatal("非法命令不得输出候选版本结果")
		}
	}
	if err := run([]string{"version"}, unreadVersionInput{t}, failedVersionOutput{}); err == nil || strings.Contains(err.Error(), "test-only:") {
		t.Fatal("输出失败未关闭或泄露原文")
	}
}
