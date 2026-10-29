//go:build integration && browser

package points_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// 自然超时/租约等待较长，单列精确入口，不挤占原报告套件20分钟看门狗；无真实浏览器或模型。
func TestIntegrationAIReportTimeoutRecovery(t *testing.T) {
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			e := newSubmissionHTTPEnv(t, level, "agent_running")
			front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "仅自动化代理，非实际H5") }))
			t.Cleanup(front.Close)
			g := newH5ReportGateway(t, e, front.URL)
			token, foreign := e.web.token(t, 1, "client"), e.web.token(t, 2, "client")
			checkBalance := func(available, frozen int64) {
				t.Helper()
				var body struct {
					Data struct {
						Available int64 `json:"availablePoints"`
						Frozen    int64 `json:"frozenPoints"`
						CheckedAt int64 `json:"checkedAt"`
					} `json:"data"`
				}
				if err := json.Unmarshal(h5GatewayRequest(t, g, "GET", "/api/v1/ai/points/balance", token, "", "", 200), &body); err != nil || body.Data.Available != available || body.Data.Frozen != frozen || body.Data.CheckedAt <= 0 {
					t.Fatal("超时取消期间公开余额与账本不一致", body, err)
				}
			}
			checkBalance(20, 0)
			created := decodeCreation(t, h5GatewayRequest(t, g, "POST", "/api/v1/ai/tasks", token, e.key, sqlSubmissionBody, 200))
			select {
			case job := <-g.actions:
				g.process(t, job, "timeout_cancel")
			case <-e.ctx.Done():
				t.Fatal("原创建未驱动worker")
			}
			g.confirmWorkerTimeout(t)
			checkBalance(13, 7)
			path := fmt.Sprintf("/api/v1/ai/tasks/%d", created.TaskID)
			h5GatewayRequest(t, g, "POST", path+"/cancel", foreign, "", "{}", 404)
			h5GatewayRequest(t, g, "POST", path+"/cancel", token, "", "{}", 200)
			var progress struct {
				Data dto.AITaskProgress `json:"data"`
			}
			if json.Unmarshal(h5GatewayRequest(t, g, "GET", path, token, "", "", 200), &progress) != nil || progress.Data.Status != task.Running || !progress.Data.CancelRequested || progress.Data.ReportAvailable {
				t.Fatal("取消受理被公开接口冒充终态")
			}
			checkBalance(13, 7)
			g.recoverTimeoutCancellation(t)
			checkBalance(20, 0)
			if json.Unmarshal(h5GatewayRequest(t, g, "GET", path, token, "", "", 200), &progress) != nil || progress.Data.Status != task.Cancelled || !progress.Data.CancelRequested || progress.Data.ReportAvailable {
				t.Fatal("独立查询未确认恢复后的本地取消终态")
			}
			h5GatewayRequest(t, g, "GET", path+"/report", token, "", "", 422)
			g.process(t, h5ReportAction{kind: "finish"}, "timeout_cancel")
		})
	}
}

func TestIntegrationAIReportH5TimeoutRecovery(t *testing.T) {
	if os.Getenv("TM_TEST_AI_H5_TIMEOUT_RECOVERY") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_INTENTS") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_LOGIN") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_STREAM_REPLAY") != "" || os.Getenv("TM_TEST_AI_H5_STREAM_FAULTS") != "" {
		t.Fatal("超时恢复只允许独立、显式原密码登录及会话浏览器验收")
	}
	e := newSubmissionHTTPEnv(t, sql.LevelReadCommitted, "agent_running")
	ctx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
	defer stop()
	e.pointsTestEnv.ctx = ctx
	g := newH5ReportGateway(t, e, os.Getenv("TM_TEST_AI_H5_ORIGIN"))
	g.timeoutRecovery = true
	t.Log("超时恢复实际H5入口（仅回环Agent/模型，不是真实多Agent）：", g.server.URL+"/__test__/")
	var timedOut <-chan struct{}
	observed := false
	for {
		select {
		case job := <-g.actions:
			switch job.kind {
			case "create":
				g.process(t, job, "timeout_cancel")
				timedOut = g.workerDone
			case "recover-timeout":
				if !observed || g.timeoutRecovered {
					t.Fatal("没有先观察原执行超时，或重复触发恢复")
				}
				g.recoverTimeoutCancellation(t)
			case "finish":
				g.process(t, job, "timeout_cancel")
				return
			default:
				if job.done != nil {
					close(job.done)
				}
				t.Fatal("超时恢复收到范围外动作")
			}
		case <-timedOut:
			g.confirmWorkerTimeout(t)
			observed, timedOut = true, nil
		case <-ctx.Done():
			t.Fatal("超时恢复实际H5未完成")
		}
	}
}
