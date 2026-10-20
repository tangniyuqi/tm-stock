// ai-worker 当前只恢复已有隔离结果，不执行 Provider/Agent、采集、报告或成功结算。
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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
	userID, taskID int64
	plan           string
}

const maxPlanBytes = 32 * 1024

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := runCommand(ctx, os.Args[1:], os.Stdout)
	stop()
	os.Exit(code)
}
func parseOptions(args []string) (options, error) {
	var out options
	if len(args) == 1 && (args[0] == "-help" || args[0] == "--help") {
		return out, flag.ErrHelp
	}
	if len(args) == 0 || args[0] != "recover" {
		return out, points.ErrInvalid
	}
	fs := flag.NewFlagSet("ai-worker recover", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int64Var(&out.userID, "user", 0, "内部会员编号")
	fs.Int64Var(&out.taskID, "task", 0, "已有任务编号")
	fs.StringVar(&out.plan, "plan", "", "可信本地恢复计划文件")
	if err := fs.Parse(args[1:]); err != nil {
		return out, err
	}
	if fs.NArg() != 0 || out.userID <= 0 || out.taskID <= 0 || out.plan == "" {
		return out, points.ErrInvalid
	}
	return out, nil
}
func readPlan(path string) ([]service.ExtractionRecoverySpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, points.ErrInvalid
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPlanBytes {
		return nil, points.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxPlanBytes+1))
	if err != nil || len(raw) > maxPlanBytes || task.CheckStageJSON(raw) != nil {
		return nil, points.ErrInvalid
	}
	// 键名精确匹配，不能借 encoding/json 大小写宽容或忽略未知字段改变计划含义。
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || len(top) != 1 || top["steps"] == nil {
		return nil, points.ErrInvalid
	}
	var items []json.RawMessage
	if err := json.Unmarshal(top["steps"], &items); err != nil || len(items) < 1 || len(items) > task.MaxSteps {
		return nil, points.ErrInvalid
	}
	var specs []service.ExtractionRecoverySpec
	for _, item := range items {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil || len(fields) != 2 || fields["snapshot_id"] == nil || fields["operation_key"] == nil {
			return nil, points.ErrInvalid
		}
		var spec service.ExtractionRecoverySpec
		d := json.NewDecoder(bytes.NewReader(item))
		d.DisallowUnknownFields()
		if err := d.Decode(&spec); err != nil || spec.SnapshotID <= 0 || !points.ValidKey(spec.OperationKey) {
			return nil, points.ErrInvalid
		}
		specs = append(specs, spec)
	}
	return specs, nil
}
func failure(output io.Writer) int {
	// 固定出口，不输出参数、计划路径、DSN、来源引用、正文或驱动错误。
	// 输出本身失败也返回非零，交由调用方处理，没有另一个可安全写入的通道。
	_, _ = io.WriteString(output, "恢复未完成；请核对参数、计划、授权、租约及数据库保护，保留原任务/操作键（详情已隐藏）。\n")
	return 1
}
func runCommand(ctx context.Context, args []string, output io.Writer) int {
	opts, err := parseOptions(args)
	if errors.Is(err, flag.ErrHelp) {
		_, err := io.WriteString(output, "用法：ai-worker recover -user <编号> -task <编号> -plan <可信本地JSON文件>\n仅恢复已有隔离响应；不发起模型/Agent、不新增预留、不生成报告或成功结算。连接只读环境变量 TM_DB_DSN。0=恢复完成但待报告/已有终态，2=响应未知，1=未完成，130=中断。\n")
		if err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		return failure(output)
	}
	specs, err := readPlan(opts.plan)
	if err != nil {
		return failure(output)
	}
	cfg, err := config.Load()
	if err != nil {
		return failure(output)
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return failure(output)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return failure(output)
	}
	repo := repository.NewAITaskRepo(db)
	tasks, err := service.NewAITaskService(repo, ledger)
	if err != nil {
		return failure(output)
	}
	planner, err := service.NewAIExtractionRecoveryPlanner(repository.NewAISourceRepo(db), repository.NewAIExtractionCatalog(db), specs)
	if err != nil {
		return failure(output)
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return failure(output)
	}
	w, err := service.NewAIRecoveryWorker(tasks, repo, planner, "worker_"+hex.EncodeToString(random[:]), task.RecoveryOptions{Lease: time.Minute, Heartbeat: 20 * time.Second, Timeout: 5 * time.Minute})
	if err != nil {
		return failure(output)
	}
	return execute(ctx, output, func(ctx context.Context) error { return repository.NewAITaskSchemaVerifier(db).Verify(ctx) }, func(ctx context.Context) (task.RecoveryResult, error) {
		return w.Recover(ctx, opts.userID, opts.taskID)
	})
}
func execute(ctx context.Context, output io.Writer, verify func(context.Context) error, recover func(context.Context) (task.RecoveryResult, error)) int {
	if ctx.Err() != nil {
		return 130
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err := verify(checkCtx)
	checkErr := checkCtx.Err()
	cancel()
	if ctx.Err() != nil {
		return 130
	}
	if err != nil || checkErr != nil {
		return failure(output)
	}
	out, err := recover(ctx)
	if ctx.Err() != nil {
		return 130
	}
	if err != nil {
		return failure(output)
	}
	valid := out.TaskID > 0 && !out.ReportAvailable && out.CurrentStep >= 0 && out.CurrentStep <= task.MaxSteps && out.PlannedSteps >= 0 && out.PlannedSteps <= task.MaxSteps
	switch out.Outcome {
	case "terminal":
		valid = valid && out.TaskStatus >= task.Succeeded && out.TaskStatus <= task.Expired
	case "report_pending":
		valid = valid && out.TaskStatus == task.Running && out.PlannedSteps > 0 && out.CurrentStep == out.PlannedSteps
	case "response_unknown":
		valid = valid && out.TaskStatus == task.Running && out.PlannedSteps > out.CurrentStep
	default:
		valid = false
	}
	if !valid {
		return failure(output)
	}
	if err := json.NewEncoder(output).Encode(out); err != nil {
		return 1
	}
	if out.Outcome == "response_unknown" {
		return 2
	}
	return 0
}
