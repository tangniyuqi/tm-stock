//go:build integration

package points_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

func TestIntegrationAIReportAgentInputsProcess(t *testing.T) {
	if os.Getenv("TM_TEST_DSN") == "" {
		t.Skip("需要隔离MySQL")
	}
	api, worker, login := buildLocalReportBinaries(t)
	for _, level := range []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead} {
		t.Run(level.String(), func(t *testing.T) {
			verifyLocalAPIWorker(t, level, 1, api, worker, login, localProcessCase{inputs: true})
		})
	}
}

func verifyLocalReportInputs(t *testing.T, f *localPublicationFixture, client *http.Client, base, path, dsn string, tokens []string, wantInputs bool) {
	t.Helper()
	e := f.env
	beforeTasks, beforeTxns, beforeCalls := e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`), e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`), e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`)
	read := func(token string, status int) dto.AIReportInputs {
		t.Helper()
		r, err := http.NewRequestWithContext(e.ctx, "GET", base+path+"/report/inputs", nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("实际报告输入读取失败")
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(res.Body, 65537))
		if err != nil || len(raw) > 65536 || res.StatusCode != status || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("报告输入HTTP契约不符 got=%d want=%d", res.StatusCode, status)
		}
		var out struct {
			Code int
			Data json.RawMessage
		}
		if json.Unmarshal(raw, &out) != nil || (out.Code == 0) != (status == 200) {
			t.Fatal("报告输入包装错误")
		}
		if status != 200 {
			if len(out.Data) > 0 && string(out.Data) != "null" {
				t.Fatal("拒读泄漏输入正文/计数")
			}
			return dto.AIReportInputs{}
		}
		for _, private := range []string{"隔离提示私有标记", "request_json", "call_id", "input_audit", "ProfileDigest", "ApprovalRef"} {
			if strings.Contains(string(raw), private) {
				t.Fatal("会员接口泄漏私有输入")
			}
		}
		var view dto.AIReportInputs
		if json.Unmarshal(out.Data, &view) != nil || len(view.Calls) < 14 || len(view.Facts) != 1 || view.Scope != "synthetic-fixture-only" || view.Calls[0].CostMicros != nil {
			t.Fatal("输入记录投影不符")
		}
		return view
	}
	status := 422
	if wantInputs {
		status = 200
	}
	read(tokens[0], status)
	read(tokens[0], status)
	read(tokens[1], 404)
	read("", 401)
	if wantInputs {
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			t.Fatal(err)
		}
		e.mustExec(`REVOKE SELECT ON ` + e.database + `.tm_ai_agent_runs FROM '` + cfg.User + `'@'127.0.0.1'`)
		read(tokens[0], 503)
		e.mustExec(`GRANT SELECT ON ` + e.database + `.tm_ai_agent_runs TO '` + cfg.User + `'@'127.0.0.1'`)
		read(tokens[0], 200)
		e.mustExec(`UPDATE tm_ai_sources s JOIN tm_ai_source_snapshots p ON p.source_id=s.id SET s.enabled=0 WHERE p.id=?`, f.spec.Materials[0].SnapshotID)
		read(tokens[0], 422)
		e.mustExec(`UPDATE tm_ai_sources s JOIN tm_ai_source_snapshots p ON p.source_id=s.id SET s.enabled=1 WHERE p.id=?`, f.spec.Materials[0].SnapshotID)
		read(tokens[0], 200)
		if e.queryInt(`SELECT COUNT(*) FROM tm_ai_tasks`) != beforeTasks || e.queryInt(`SELECT COUNT(*) FROM tm_points_txns`) != beforeTxns || e.queryInt(`SELECT COUNT(*) FROM tm_ai_report_call_audits`) != beforeCalls {
			t.Fatal("输入读取增加任务、积分或模型审计")
		}
		t.Log("TM_STOCK_AGENT_INPUTS_HTTP_VERIFIED")
	}
}
