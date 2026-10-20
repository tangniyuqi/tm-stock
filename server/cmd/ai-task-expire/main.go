// ai-task-expire 显式限量清扫已有任务，不启动模型、调度循环或成功结算。
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type options struct {
	sweep   task.ExpiryOptions
	timeout time.Duration
}

type receipt struct {
	Event   string            `json:"event"`
	Sweep   *task.ExpirySweep `json:"sweep,omitempty"`
	Failed  bool              `json:"failed"`
	Message string            `json:"message,omitempty"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runCommand(ctx, os.Args[1:], os.Stdout)
	stop()
	os.Exit(code)
}

func parseOptions(args []string) (options, error) {
	var out options
	fs := flag.NewFlagSet("ai-task-expire", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int64Var(&out.sweep.AfterID, "after", 0, "本轮已确认的任务游标；新轮从 0 开始")
	fs.IntVar(&out.sweep.PageSize, "page-size", 100, "每页读取行数（1–1000）")
	fs.IntVar(&out.sweep.MaxTasks, "max-tasks", 1000, "本次最多检查的任务数（1–100000）")
	fs.DurationVar(&out.timeout, "timeout", 30*time.Second, "本次期限（1 秒–5 分钟）")
	if err := fs.Parse(args); err != nil {
		return out, err
	}
	if fs.NArg() != 0 || out.sweep.Validate() != nil || out.timeout < time.Second || out.timeout > 5*time.Minute {
		return out, points.ErrInvalid
	}
	return out, nil
}

func emit(w io.Writer, v receipt) int {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return 1
	}
	return 0
}

func failure(w io.Writer, message string) int {
	// 输出失败仍非零退出，不能追加驱动详情、DSN 或原始错误。
	emit(w, receipt{Event: "ai_task_expiry_failure", Failed: true, Message: message})
	return 1
}

func runCommand(ctx context.Context, args []string, output io.Writer) int {
	opts, err := parseOptions(args)
	if errors.Is(err, flag.ErrHelp) {
		if _, err := io.WriteString(output, "用法：ai-task-expire [-after 0] [-page-size 100] [-max-tasks 1000] [-timeout 30s]\n显式修改已到期任务及冻结账务；连接仅从 TM_DB_DSN 读取。退出码 0=本轮可见上界读完，2=达到限量可续扫，1=失败/未知结果，130=中断。新轮必须从 0 开始，不是周期调度或远端调用已停止的证明。\n"); err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		return failure(output, "清扫参数无效，请查看 -help")
	}
	cfg, err := config.Load()
	if err != nil {
		return failure(output, "清扫数据库环境配置无效")
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return failure(output, "清扫数据库连接配置无效")
	}
	// 只释放本命令连接；关闭失败不改写已提交业务结果。
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	repo := repository.NewAITaskRepo(db)
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return failure(output, "清扫账本装配失败")
	}
	tasks, err := service.NewAITaskService(repo, ledger)
	if err != nil {
		return failure(output, "清扫任务装配失败")
	}
	svc, err := service.NewAITaskExpiryService(tasks, repo)
	if err != nil {
		return failure(output, "清扫服务装配失败")
	}
	return runSweep(ctx, opts, output, repository.NewAITaskSchemaVerifier(db).Verify, svc.Sweep)
}

func runSweep(ctx context.Context, opts options, output io.Writer, verify func(context.Context) error, sweep func(context.Context, task.ExpiryOptions) (task.ExpirySweep, error)) int {
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err := verify(checkCtx)
	checkErr := checkCtx.Err()
	cancel()
	if err != nil || checkErr != nil {
		failure(output, "联合结构/拒写自检失败，未开始清扫")
		if ctx.Err() != nil {
			return 130
		}
		return 1
	}
	roundCtx, stop := context.WithTimeout(ctx, opts.timeout)
	out, err := sweep(roundCtx, opts.sweep)
	roundErr := roundCtx.Err()
	stop()
	failed := err != nil || roundErr != nil
	if failed {
		out.Complete = false
	}
	v := receipt{Event: "ai_task_expiry_round", Sweep: &out, Failed: failed}
	if failed {
		v.Message = "本次未完成；仅可从已确认游标重查，失败项可能已经提交，不得补扣或重发调用"
	}
	if emit(output, v) != 0 {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	if failed {
		return 1
	}
	if !out.Complete {
		return 2
	}
	return 0
}
