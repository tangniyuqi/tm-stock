package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

// 仅受控隔离库的DML回滚自检；不能当作只读命令或执行批准。
func checkLocalAudit(ctx context.Context, args []string, output io.Writer) int {
	stage := 1
	fail := func() int {
		_, _ = fmt.Fprintf(output, "本地调用审计启动核验失败（阶段%d，详情已隐藏），未启动队列或模型。\n", stage)
		return 1
	}
	if ctx == nil || len(args) != 2 || args[0] != "-config" {
		return fail()
	}
	if ctx.Err() != nil {
		return 130
	}
	runtime, err := config.ReadAILocalRuntime(args[1])
	dsn := os.Getenv("TM_DB_DSN")
	if err != nil || runtime.CheckDSN(dsn) != nil {
		return fail()
	}
	stage = 2
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fail()
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	stage = 3
	if err := repository.NewAILocalInstanceVerifier(db).Verify(ctx, runtime.InstanceID); err != nil {
		if ctx.Err() != nil {
			return 130
		}
		return fail()
	}
	stage = 4
	if err := repository.NewAIReportCallAuditSchemaVerifier(db).Verify(ctx); err != nil {
		if ctx.Err() != nil {
			return 130
		}
		return fail()
	}
	if _, err := io.WriteString(output, "本地调用审计及原依赖保护核验通过；业务探针已回滚，未启动队列、模型或实际任务，不等于执行批准。\n"); err != nil {
		return 1
	}
	return 0
}
