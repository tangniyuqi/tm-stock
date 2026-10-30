//go:build integration

package points_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// 只控制本轮合成资料；反馈处理仍经独立管理进程的原接口，不改写任何业务回包。
type localH5Assistant struct {
	mu                         sync.Mutex
	env                        *workflowEnv
	qa                         *localProcessQA
	peer                       *localFeedbackPeer
	client                     *http.Client
	resolved, empty, withdrawn bool
}

func (a *localH5Assistant) ready(c localAPIWorkerH5Counts) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.resolved && a.empty && a.withdrawn && c.Questions == 3 && c.Answers >= 4 && c.Feedback == 1 && c.FeedbackReads >= 2
}
func (a *localH5Assistant) action(parent context.Context, path string, c localAPIWorkerH5Counts) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	switch path {
	case "/__test__/resolve":
		if a.resolved {
			return true
		}
		if c.Feedback != 1 {
			return false
		}
		var id int64
		if err := a.env.db.QueryRowContext(ctx, `SELECT id FROM tm_ai_report_feedbacks WHERE user_id=1`).Scan(&id); err != nil {
			return false
		}
		for _, body := range []string{`{"expectedVersion":1,"status":"processing","resolution":""}`, `{"expectedVersion":2,"status":"resolved","resolution":"reviewed"}`} {
			r, err := http.NewRequestWithContext(ctx, "POST", a.peer.base+"/internal/v1/ai-feedback/"+strconv.FormatInt(id, 10)+"/decisions", strings.NewReader(body))
			if err != nil {
				return false
			}
			r.Header.Set("Authorization", "Bearer "+a.peer.credential)
			r.Header.Set("Content-Type", "application/json")
			res, err := a.client.Do(r)
			if err != nil {
				return false
			}
			res.Body.Close()
			if res.StatusCode != 200 {
				return false
			}
		}
		a.resolved = true
		return true
	case "/__test__/empty":
		if a.empty {
			return true
		}
		if c.Questions != 2 || a.qa.server.Count() != 6 {
			return false
		}
		_, err := a.env.db.ExecContext(ctx, `UPDATE tm_ai_qa_scopes SET evidence_json='[]',version=version+1,updated_at=UTC_TIMESTAMP(3)`)
		if err != nil {
			return false
		}
		a.empty = true
		return true
	case "/__test__/withdraw":
		if a.withdrawn {
			return true
		}
		if !a.resolved || !a.empty || c.Questions != 3 {
			return false
		}
		_, err := a.env.db.ExecContext(ctx, `UPDATE tm_ai_sources SET enabled=0`)
		if err != nil {
			return false
		}
		a.withdrawn = true
		return true
	}
	return false
}
func (a *localH5Assistant) verify(t *testing.T) {
	t.Helper()
	e := a.env
	if e.queryInt(`SELECT COUNT(*) FROM tm_ai_conversation_messages WHERE user_id=1 AND kind='question' AND skill='report_qa' AND task_id>0`) != 1 || a.qa.server.Count() != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_calls`) != 6 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests WHERE state='answered'`) != 2 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_qa_requests WHERE state='refused'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_feedbacks WHERE status='resolved' AND version=3 AND resolution='reviewed'`) != 1 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_feedback_events`) != 3 || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_feedback_events WHERE actor_id=7`) != 2 {
		t.Fatal("组合H5的问答存证或反馈处理历史不符")
	}
	t.Log("实际H5组合问答/报告追问各一次、无证据零调用、原管理进程反馈处理及三事件核验通过")
}
