// points-audit 只核对专用积分账本并输出告警，不迁移、不扣费、不自动修账。
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
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type options struct {
	pageSize int
	timeout  time.Duration
	interval time.Duration
}

type auditEvent struct {
	Event   string              `json:"event"`
	At      int64               `json:"at"`
	UserID  int64               `json:"user_id,omitempty"`
	Issues  []points.AuditIssue `json:"issues,omitempty"`
	Sweep   *points.AuditSweep  `json:"sweep,omitempty"`
	Failed  bool                `json:"failed,omitempty"`
	Message string              `json:"message,omitempty"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runCommand(ctx, os.Args[1:], os.Stdout)
	stop()
	os.Exit(code)
}

func parseOptions(args []string) (options, error) {
	var out options
	fs := flag.NewFlagSet("points-audit", flag.ContinueOnError)
	// 失败只输出固定提示，不能把未知参数或连接字符串原样打印。
	fs.SetOutput(io.Discard)
	fs.IntVar(&out.pageSize, "page-size", 100, "每页用户数（1–1000）")
	fs.DurationVar(&out.timeout, "round-timeout", 10*time.Minute, "整轮对账期限（1 秒–24 小时）")
	fs.DurationVar(&out.interval, "interval", 0, "完成本轮后等待多久再运行；0 表示单次")
	if err := fs.Parse(args); err != nil {
		return out, err
	}
	if fs.NArg() != 0 || out.pageSize < 1 || out.pageSize > 1000 || out.timeout < time.Second || out.timeout > 24*time.Hour || out.interval < 0 || (out.interval > 0 && out.interval < time.Second) || out.interval > 24*time.Hour {
		return out, points.ErrInvalid
	}
	return out, nil
}

func emit(output io.Writer, event auditEvent) error {
	event.At = time.Now().UnixMilli()
	return json.NewEncoder(output).Encode(event)
}

func commandFailure(output io.Writer, message string) int {
	// 输出通道本身失败时无处再次告警；退出非零交给进程监督器处理。
	_ = emit(output, auditEvent{Event: "points_audit_failure", Failed: true, Message: message})
	return 1
}

func runCommand(ctx context.Context, args []string, output io.Writer) int {
	opts, err := parseOptions(args)
	if errors.Is(err, flag.ErrHelp) {
		if _, err := io.WriteString(output, "用法：points-audit [-page-size 100] [-round-timeout 10m] [-interval 0]\n连接配置仅从环境变量 TM_DB_DSN 读取；退出码 0=本轮健康，2=账本异常，1=未完成或配置/输出失败，130=中断。周期模式异常不退出，须接入 JSON 告警采集。\n"); err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		return commandFailure(output, "对账参数无效，请查看 -help")
	}
	cfg, err := config.Load()
	if err != nil {
		return commandFailure(output, "对账数据库环境配置无效")
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return commandFailure(output, "对账数据库连接配置无效")
	}
	// 关闭失败不改变只读对账结果；不输出驱动细节或 DSN。
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	repo := repository.NewPointsRepo(db)
	svc, err := service.NewPointsService(repo)
	if err != nil {
		return commandFailure(output, "对账服务装配失败")
	}
	return runAuditLoop(ctx, opts, output, repository.NewPointsSchemaVerifier(db).Verify,
		func(ctx context.Context, notify func(points.AuditReport) error) (points.AuditSweep, error) {
			return svc.ReconcileAll(ctx, repo, opts.pageSize, notify)
		})
}

func runAuditLoop(ctx context.Context, opts options, output io.Writer, verify func(context.Context) error, sweep func(context.Context, func(points.AuditReport) error) (points.AuditSweep, error)) int {
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err := verify(checkCtx)
	checkErr := checkCtx.Err()
	cancel()
	if err != nil || checkErr != nil {
		code := commandFailure(output, "账本结构与事务约束自检失败，未开始扫描（驱动详情已隐藏）")
		if ctx.Err() != nil {
			return 130
		}
		return code
	}
	for {
		if ctx.Err() != nil {
			return 130
		}
		roundCtx, roundCancel := context.WithTimeout(ctx, opts.timeout)
		outputFailed := false
		result, err := sweep(roundCtx, func(report points.AuditReport) error {
			err := emit(output, auditEvent{Event: "points_audit_issue", UserID: report.UserID, Issues: report.Issues})
			outputFailed = outputFailed || err != nil
			return err
		})
		roundErr := roundCtx.Err()
		roundCancel()
		if outputFailed {
			return 1
		}
		failed := err != nil || roundErr != nil || !result.Complete
		if failed {
			// 不允许错误的适配器把中断结果标为完整。
			result.Complete = false
		}
		event := auditEvent{Event: "points_audit_round", Sweep: &result, Failed: failed}
		if failed {
			event.Message = "本轮对账未完成；不得当作健康，须排查后重跑（驱动详情已隐藏）"
		}
		if err := emit(output, event); err != nil {
			return 1
		}
		if ctx.Err() != nil {
			return 130
		}
		if opts.interval == 0 {
			if failed {
				return 1
			}
			if result.Unhealthy > 0 {
				return 2
			}
			return 0
		}
		// 单循环串行执行，不补跑错过的时点，不创建脱离上下文的 goroutine。
		timer := time.NewTimer(opts.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 130
		case <-timer.C:
		}
	}
}
