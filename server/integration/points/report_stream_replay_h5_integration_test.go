//go:build integration && browser

package points_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 原H5/密码登录/worker/数据库事件及报告账本，只有CN和模型为回环替身。
func TestIntegrationAIReportH5StreamReplay(t *testing.T) {
	if os.Getenv("TM_TEST_AI_H5_STREAM_REPLAY") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_INTENTS") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_ORIGIN") == "" {
		t.Fatal("原事件复放必须显式、单次本地浏览器验收")
	}
	e := newSubmissionHTTPEnv(t, sql.LevelReadCommitted, "agent_running")
	ctx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
	defer stop()
	e.pointsTestEnv.ctx = ctx
	g := newH5ReportGateway(t, e, os.Getenv("TM_TEST_AI_H5_ORIGIN"))
	if g.memberLogin == nil || g.streamReplay == nil {
		t.Fatal("缺原登录或原事件传输包装")
	}
	t.Log("原事件SSE续读/去重本地入口（回环Agent/模型，不是真实多Agent）：", g.server.URL+"/__test__/")
	created, started, completed := false, false, false
	for {
		select {
		case job := <-g.actions:
			switch job.kind {
			case "create":
				if created {
					t.Fatal("原任务被重复创建")
				}
				created = true
				g.serial.Lock()
				e.bindNewTask(t, job.id)
				g.serial.Unlock()
				if e.queryInt("SELECT frozen FROM tm_points_accounts WHERE user_id=1") != 7 {
					t.Fatal("原报价没有冻结")
				}
			case "start-stream-worker":
				if !created || started {
					t.Fatal("worker未由首次原SSE连接唯一启动")
				}
				started = true
				g.serial.Lock()
				g.startRunning(t)
				g.serial.Unlock()
			case "complete-stream-agent":
				if !started || completed {
					t.Fatal("回环Agent完成放行不唯一")
				}
				completed = true
				e.agentComplete.Store(true)
			case "finish":
				if job.done != nil {
					defer close(job.done)
					job.done = nil
				}
				if !created || !started || !completed {
					t.Fatal("原SSE连接/重连/worker闭环缺失")
				}
				select {
				case <-g.workerDone:
					if g.workerErr != nil {
						t.Fatal("原worker未成功结束", g.workerErr)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("原worker仍未完成")
				}
				r, payload, err := e.payloads.Quarantined(ctx, e.planner.p.Agent.ActorID, e.planner.p.Agent.RequestID())
				if err != nil || r.State != agents.Quarantined || !payload.Valid() || r.CancelRequested {
					t.Fatal("回环Agent原隔离结果与摘要未核实", err)
				}
				var upstreamCompleted int
				if err := e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tm_ai_agent_events WHERE run_id=? AND event='observed' AND JSON_UNQUOTE(JSON_EXTRACT(record_json,'$.state'))=?`, r.ID, agents.Completed).Scan(&upstreamCompleted); err != nil || upstreamCompleted != 1 {
					t.Fatal("缺少恰好一次上游完成原审计事件", err)
				}
				s := g.streamReplay
				s.mu.Lock()
				if s.invalid || s.cuts != 1 || s.copies != 2 || s.first <= 0 || len(s.cursors) != 2 || s.cursors[0] != 1 || s.cursors[1] != s.first || len(s.frames) < 2 {
					s.mu.Unlock()
					t.Fatal("原事件复制/中断/确认游标/后续事件未完整")
				}
				frames := make(map[int64][]byte, len(s.frames))
				for seq, raw := range s.frames {
					frames[seq] = bytes.Clone(raw)
				}
				s.mu.Unlock()
				read, err := service.NewAITaskReadServiceWithReports(e.repo, e.web.reader)
				if err != nil {
					t.Fatal(err)
				}
				for seq, raw := range frames {
					page, err := read.Events(ctx, 1, e.v.ID, seq-1, 1)
					if err != nil || len(page.Items) != 1 || page.Items[0].Seq != seq {
						t.Fatal("所发送帧不是原库事件", err)
					}
					body, err := json.Marshal(page.Items[0])
					if err != nil {
						t.Fatal(err)
					}
					original := []byte("id: " + strconv.FormatInt(seq, 10) + "\nevent: progress\ndata: " + string(body) + "\n\n")
					if !bytes.Equal(raw, original) {
						t.Fatal("传输帧与原公开事件逐字不一致")
					}
				}
				t.Log("原库公开帧逐字核对：首次重复及重连复放各1、原确认游标续读、后续新事件；浏览器去重须另核DOM")
				g.process(t, job, "success")
				return
			default:
				if job.done != nil {
					close(job.done)
				}
				t.Fatal("本专项收到范围外测试动作")
			}
		case <-ctx.Done():
			t.Fatal("原事件SSE实际H5未在期限内完成")
		}
	}
}
