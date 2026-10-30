//go:build integration

package points_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// 原密码、实际API/worker/管理进程；模型仅回环协议替身。
func TestIntegrationAIReportLocalFeedbackProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	admin := buildLocalFeedbackAdmin(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			for _, kind := range []int{1, 2} {
				t.Run(strconv.Itoa(kind), func(t *testing.T) {
					verifyLocalAPIWorker(t, level, kind, api, worker, login, localProcessCase{feedbackAdmin: admin})
				})
			}
		})
	}
}

func buildLocalFeedbackAdmin(t *testing.T) string {
	t.Helper()
	admin := filepath.Join(t.TempDir(), "feedback-admin.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", admin, "./cmd/ai-admin")
	build.Dir = filepath.Join("..", "..")
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("构建实际管理服务失败")
	}
	return admin
}

type localFeedbackPeer struct{ base, credential string }

func localFeedbackAdminDSN(t *testing.T, e *workflowEnv) string {
	t.Helper()
	tables := []string{"tm_points_accounts", "tm_points_txns", "tm_points_lots", "tm_points_entries", "tm_points_holds", "tm_points_hold_allocs", "tm_points_debts", "tm_points_debt_allocs", "tm_ai_tasks", "tm_ai_task_events", "tm_ai_task_budgets", "tm_ai_task_reservations", "tm_ai_task_step_results", "tm_ai_task_steps", "tm_ai_task_admissions", "tm_ai_report_commits", "tm_ai_report_feedbacks", "tm_ai_report_feedback_events"}
	return localProcessDSN(t, e, tables, false)
}

func startLocalFeedbackPeer(t *testing.T, f *localPublicationFixture, binary string, client *http.Client) *localFeedbackPeer {
	t.Helper()
	e := f.env
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	runtime := f.runtime
	runtime.Listen = l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "admin-runtime.json")
	raw, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	peer := &localFeedbackPeer{base: "http://" + runtime.Listen, credential: hex.EncodeToString(secret[:])}
	reviewers, err := json.Marshal([]config.AIAdminReviewer{{ActorID: 7, Token: peer.credential}})
	if err != nil {
		t.Fatal(err)
	}
	dsn := localFeedbackAdminDSN(t, e)
	ctx, cancel := context.WithCancel(e.ctx)
	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = append(localWorkerEnvironment(dsn, ""), "TM_AI_LOCAL_CONFIG="+file, "TM_AI_ADMIN_REVIEWERS="+string(reviewers), "TM_AI_FEEDBACK_ADMIN_ENABLED=true", "TM_AI_L0_ENABLED=false")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal("实际管理服务启动失败")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ended := false
	t.Cleanup(func() {
		cancel()
		if !ended {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("实际管理服务未退出")
			}
		}
	})
	deadline := time.NewTimer(40 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		r, err := http.NewRequestWithContext(ctx, "GET", peer.base+"/internal/v1/ai-feedback?limit=1", nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+peer.credential)
		res, err := client.Do(r)
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				return peer
			}
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("实际管理服务就绪超时")
		case <-done:
			ended = true
			t.Fatal("实际管理服务提前退出", strings.ReplaceAll(output.String(), peer.credential, "[隐藏凭据]"))
		}
	}
}

func verifyLocalFeedbackProcess(t *testing.T, f *localPublicationFixture, peer *localFeedbackPeer, client *http.Client, base, taskPath string, tokens []string) {
	t.Helper()
	e := f.env
	before := e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`)
	call := func(origin, method, path, credential, body string, want int) json.RawMessage {
		t.Helper()
		r, err := http.NewRequestWithContext(e.ctx, method, origin+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if credential != "" {
			r.Header.Set("Authorization", "Bearer "+credential)
		}
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("实际反馈进程HTTP请求失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(raw) > 65536 || res.StatusCode != want {
			t.Fatalf("实际反馈HTTP状态不符：%s %s got=%d want=%d", method, path, res.StatusCode, want)
		}
		for _, secret := range []string{tokens[0], tokens[1], peer.credential} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("实际反馈响应泄漏凭据")
			}
		}
		if want == 404 && !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
			return nil
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("实际反馈响应未禁止缓存")
		}
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &envelope) != nil || (want == 200) != (envelope.Code == 0) {
			t.Fatal("实际反馈响应契约不符")
		}
		if origin == base {
			for _, key := range []string{"actorId", "userId", "reportDigest"} {
				if strings.Contains(string(raw), `"`+key+`"`) {
					t.Fatal("会员反馈响应含内部字段")
				}
			}
		}
		return envelope.Data
	}
	path := taskPath + "/feedback"
	call(base, "GET", path, tokens[1], "", 404)
	call(base, "GET", path, "", "", 401)
	call(base, "GET", path, peer.credential, "", 401)
	call(peer.base, "GET", "/internal/v1/ai-feedback", tokens[0], "", 401)
	for _, path := range []string{"/internal/v1/ai-sources", "/internal/v1/ai-extractions", "/api/v1/ai/tasks"} {
		call(peer.base, "GET", path, peer.credential, "", 404)
	}
	var receipt dto.AIFeedback
	if json.Unmarshal(call(base, "POST", path, tokens[0], `{"category":"display"}`, 200), &receipt) != nil || receipt.ID <= 0 || receipt.Status != "received" || receipt.Version != 1 {
		t.Fatal("实际反馈受理回执不符")
	}
	var duplicate dto.AIFeedback
	if json.Unmarshal(call(base, "POST", path, tokens[0], `{"category":"display"}`, 200), &duplicate) != nil || duplicate != receipt {
		t.Fatal("重复反馈另建受理单")
	}
	call(base, "POST", path, tokens[0], `{"category":"display","text":"不接受自由正文"}`, 400)
	internal := "/internal/v1/ai-feedback/" + strconv.FormatInt(receipt.ID, 10)
	var queue dto.AIFeedbackPage
	if json.Unmarshal(call(peer.base, "GET", "/internal/v1/ai-feedback?limit=1", peer.credential, "", 200), &queue) != nil || len(queue.Items) != 1 || queue.Items[0] != receipt {
		t.Fatal("管理进程未读取原受理单")
	}
	call(peer.base, "GET", internal, peer.credential, "", 200)
	processing := `{"expectedVersion":1,"status":"processing","resolution":""}`
	call(peer.base, "POST", internal+"/decisions", peer.credential, processing, 200)
	call(peer.base, "POST", internal+"/decisions", peer.credential, processing, 200)
	// 版本1只能进入处理中；跳级请求应在参数层拒绝。
	call(peer.base, "POST", internal+"/decisions", peer.credential, `{"expectedVersion":1,"status":"resolved","resolution":"referred"}`, 400)
	resolved := `{"expectedVersion":2,"status":"resolved","resolution":"referred"}`
	call(peer.base, "POST", internal+"/decisions", peer.credential, resolved, 200)
	call(peer.base, "POST", internal+"/decisions", peer.credential, resolved, 200)
	// 结构合法的同版本不同决定才是终态竞争冲突，不能覆盖原处理结果。
	call(peer.base, "POST", internal+"/decisions", peer.credential, `{"expectedVersion":2,"status":"resolved","resolution":"reviewed"}`, 409)
	e.mustExec(`UPDATE tm_ai_sources SET enabled=0`)
	defer e.mustExec(`UPDATE tm_ai_sources SET enabled=1`)
	call(base, "GET", taskPath+"/report", tokens[0], "", 422)
	var list dto.AIFeedbackList
	if json.Unmarshal(call(base, "GET", path, tokens[0], "", 200), &list) != nil || len(list.Items) != 1 || list.Items[0].ID != receipt.ID || list.Items[0].Status != "resolved" || list.Items[0].Version != 3 || list.Items[0].Resolution != "referred" {
		t.Fatal("会员未查询到实际管理处理结果")
	}
	var detail dto.AIInternalFeedback
	if json.Unmarshal(call(peer.base, "GET", internal, peer.credential, "", 200), &detail) != nil || len(detail.Events) != 3 || detail.Events[1].ActorID != 7 || detail.Events[2].ActorID != 7 {
		t.Fatal("实际操作人或事件链不符")
	}
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_feedbacks`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_feedback_events`) != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != before || e.submits.Load() != 1 || len(e.server.Requests()) != 3 {
		t.Fatal("反馈处理污染模型、报告或账务")
	}
	t.Log("实际API反馈提交/复读→独立管理进程受理/处理→会员查询通过；默认无真实调用，零新增模型/账务")
}
