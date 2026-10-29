//go:build integration && browser

package points_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestIntegrationAIReportH5Deep(t *testing.T) {
	if os.Getenv("TM_TEST_AI_H5_DEEP") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_INTENTS") != "local-fixture-only" || os.Getenv("TM_TEST_AI_H5_LOGIN") != "local-fixture-only" {
		t.Fatal("深度H5仅允许显式原密码登录及会话意图隔离模式")
	}
	for _, key := range []string{"TM_TEST_AI_LIVE", "TM_TEST_AI_H5_STREAM_REPLAY", "TM_TEST_AI_H5_STREAM_FAULTS", "TM_TEST_AI_H5_TIMEOUT_RECOVERY", "TM_TEST_AI_H5_MATERIAL_QA", "TM_TEST_AI_H5_REPORT_QA", "TM_TEST_AI_H5_QA_HISTORY", "TM_TEST_AI_H5_ASSISTANT"} {
		if os.Getenv(key) != "" {
			t.Fatal("深度H5不能混用真实调用或其它专项")
		}
	}
	ctx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
	defer stop()
	g := newDeepH5Gateway(t, sql.LevelReadCommitted, os.Getenv("TM_TEST_AI_H5_ORIGIN"), ctx)
	t.Log("深度实际H5入口（示例资料/积分/回环模型，不是真实多Agent）：", g.server.URL+"/__test__/")
	for {
		select {
		case job := <-g.actions:
			switch job.kind {
			case "create":
				g.process(t, job, "deep")
			case "finish":
				g.serial.Lock()
				reads := g.reportReads
				g.serial.Unlock()
				if reads < 2 {
					if job.done != nil {
						close(job.done)
					}
					t.Fatal("深度H5尚未通过公开接口读取并重读报告")
				}
				finishDeepH5Gateway(t, g, job)
				return
			default:
				if job.done != nil {
					close(job.done)
				}
				t.Fatal("深度H5收到范围外动作")
			}
		case <-ctx.Done():
			t.Fatal("深度实际H5未完成报告与结算核验")
		}
	}
}
