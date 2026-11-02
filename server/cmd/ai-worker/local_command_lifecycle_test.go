package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type localCommandWriter struct {
	bytes.Buffer
	failAt, calls int
}

func (w *localCommandWriter) Write(b []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(b)
}
func (w *localCommandWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

type localCommandInput struct {
	io.Reader
	closed bool
}

func (r *localCommandInput) Close() error {
	r.closed = true
	if closer, ok := r.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type localCommandReadError struct{}

func (localCommandReadError) Read([]byte) (int, error) { return 0, errors.New("private_reader_error") }

func localCommandFiles(t *testing.T) []string {
	t.Helper()
	_, p, e := localGuardFiles(t, nil)
	r := config.AILocalRuntime{Version: 1, Environment: "local-isolated", InstanceID: strings.Repeat("a", 32), Database: "tm_ai_local_" + strings.Repeat("a", 32), Listen: "127.0.0.1:8099", PollIntervalMS: 1000, BatchSize: 10}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	runtime := filepath.Join(filepath.Dir(p), "runtime.json")
	if err = os.WriteFile(runtime, b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TM_DB_DSN", "worker:fixture_secret@tcp(127.0.0.1:65534)/"+r.Database+"?parseTime=true")
	t.Setenv("TM_AI_LOCAL_AGENT_BEARER", strings.Repeat("x", 40))
	t.Setenv("TM_AI_LOCAL_MODEL_SESSION", "")
	return []string{"-config", runtime, "-plans", p, "-execution", e}
}

func TestLocalWorkerCommandLifecycle(t *testing.T) {
	for _, mode := range []string{"stop", "eof", "bad_input", "long_input", "read_error", "queue_error", "queue_nil", "parent_cancel", "ready_write_error", "stop_write_error", "reason_write_error", "startup_admission_error", "startup_schema_error", "startup_cancel", "session", "resume_other_member"} {
		t.Run(mode, func(t *testing.T) {
			args := localCommandFiles(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			input := &localCommandInput{Reader: strings.NewReader("stop\n")}
			out := &localCommandWriter{}
			wantCode, wantRuns := 0, 1
			var target *localReportTarget
			switch mode {
			case "eof":
				input.Reader = strings.NewReader("")
			case "bad_input":
				input.Reader = strings.NewReader("go\n")
				wantCode = 1
			case "long_input":
				input.Reader = strings.NewReader(strings.Repeat("x", 30))
				wantCode = 1
			case "read_error":
				input.Reader = localCommandReadError{}
				wantCode = 1
			case "queue_error", "queue_nil", "reason_write_error":
				input = nil
				wantCode = 1
			case "parent_cancel":
				input = nil
				wantCode = 130
			case "ready_write_error":
				reader, writer := io.Pipe()
				defer writer.Close()
				input.Reader = reader // 输出失败也必须解阻塞等待中的输入并等待读协程退出。
				out.failAt = 1
				wantCode, wantRuns = 1, 0
			case "stop_write_error":
				out.failAt = 2
				wantCode = 1
			case "startup_admission_error", "startup_schema_error":
				wantCode, wantRuns = 1, 0
			case "startup_cancel":
				wantCode, wantRuns = 130, 0
			case "session":
				t.Setenv("TM_AI_LOCAL_MODEL_SESSION", strings.Repeat("a1", 32))
			case "resume_other_member":
				target = &localReportTarget{userID: 8, taskID: 7}
				wantCode, wantRuns = 1, 0
			}
			if mode == "reason_write_error" {
				out.failAt = 2
			}
			if input != nil {
				args = append(args, "-stop-on-stdin")
			}
			verifyCalls, runs := 0, 0
			verify := func(c context.Context, a *service.AILocalAdmissionControl, db *sql.DB, payloads *repository.AIAgentRepo) (int, error) {
				verifyCalls++
				if a == nil || db == nil || payloads == nil || strings.Contains(out.String(), "READY") {
					t.Fatal("启动核验之前发布就绪或依赖未装配")
				}
				d, ok := c.Deadline()
				if !ok || d.After(time.Now().Add(90*time.Second)) {
					t.Fatal("启动核验缺时限")
				}
				switch mode {
				case "startup_admission_error":
					return 2, errors.New("private_startup_error")
				case "startup_schema_error":
					return 3, errors.New("private_schema_error")
				case "startup_cancel":
					cancel()
					return 2, c.Err()
				}
				return 3, nil
			}
			run := func(c context.Context, light, deep *service.AIL1ReportQueue, max int, interval time.Duration) error {
				runs++
				if !strings.Contains(out.String(), "LOCAL_REPORT_WORKER_READY") || max != 10 || interval != time.Second {
					t.Fatal("队列早于就绪或原运行参数变更")
				}
				// 原队列校验先核轻深身份，再看取消；这里不产生SQL或模型请求。
				stopped, stop := context.WithCancel(c)
				stop()
				if err := service.RunAIReportQueues(stopped, light, deep, max, interval); !errors.Is(err, context.Canceled) {
					t.Fatal("命令没有装配原轻深队列")
				}
				switch mode {
				case "queue_error", "reason_write_error":
					return errors.New("private_queue_error")
				case "queue_nil":
					return nil
				case "parent_cancel":
					cancel()
				}
				<-c.Done()
				return c.Err()
			}
			var reader io.ReadCloser
			if input != nil {
				reader = input
			}
			if code := runLocalReportModeWith(ctx, args, reader, out, target, verify, run); code != wantCode || verifyCalls != 1 || runs != wantRuns {
				t.Fatal("命令生命周期不符", code, verifyCalls, runs, out.String())
			}
			if input != nil && (wantRuns > 0 || mode == "ready_write_error") && !input.closed {
				t.Fatal("退出未关闭输入")
			}
			for _, secret := range []string{"fixture_secret", "private_", args[1], args[3], args[5]} {
				if strings.Contains(out.String(), secret) {
					t.Fatal("命令输出泄漏内部信息")
				}
			}
			if strings.HasPrefix(mode, "startup_") && strings.Contains(out.String(), "READY") {
				t.Fatal("启动失败仍发布就绪")
			}
		})
	}
}

func TestLocalWorkerCommandFilesBeforeStartup(t *testing.T) {
	for _, mode := range []string{"runtime", "plans", "execution", "session", "missing_input", "nil_verifier", "nil_runner", "default_verifier"} {
		t.Run(mode, func(t *testing.T) {
			args := localCommandFiles(t)
			var out bytes.Buffer
			verifyCalls := 0
			verify := localReportStartupVerifier(func(context.Context, *service.AILocalAdmissionControl, *sql.DB, *repository.AIAgentRepo) (int, error) {
				verifyCalls++
				return 2, points.ErrInvalid
			})
			run := localReportQueueRunner(service.RunAIReportQueues)
			switch mode {
			case "runtime", "plans", "execution":
				index := map[string]int{"runtime": 1, "plans": 3, "execution": 5}[mode]
				if err := os.WriteFile(args[index], []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "session":
				t.Setenv("TM_AI_LOCAL_MODEL_SESSION", "invalid_local_session")
			case "missing_input":
				args = append(args, "-stop-on-stdin")
			case "nil_verifier":
				verify = nil
			case "nil_runner":
				run = nil
			case "default_verifier":
				verify = func(c context.Context, a *service.AILocalAdmissionControl, db *sql.DB, p *repository.AIAgentRepo) (int, error) {
					stopped, stop := context.WithCancel(c)
					stop()
					return verifyLocalReportStartup(stopped, a, db, p)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			code := runLocalReportModeWith(ctx, args, nil, &out, nil, verify, run)
			if code != 1 && !(mode == "default_verifier" && code == 130) || verifyCalls != 0 || strings.Contains(out.String(), "READY") {
				t.Fatal("启动前置未拒绝", code, verifyCalls)
			}
		})
	}
}
