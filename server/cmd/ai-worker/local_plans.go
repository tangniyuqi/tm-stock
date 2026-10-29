package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 显式检查同一份计划和当前资料，不启动队列或模型；任一缺项整体拒绝。
func checkLocalPlans(ctx context.Context, args []string, output io.Writer) int {
	stage := 1
	fail := func() int {
		_, _ = fmt.Fprintf(output, "本地合成资料计划核验失败（阶段%d，详情已隐藏），未启动任务或真实调用。\n", stage)
		return 1
	}
	if ctx == nil || len(args) != 4 || args[0] != "-config" || args[2] != "-plans" {
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
	source, err := config.NewAILocalPlanSource(args[3], runtime)
	stage = 2
	if err != nil {
		return fail()
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fail()
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	control, err := service.NewAILocalAdmissionControl(source, repository.NewAILocalInstanceVerifier(db), repository.NewAITaskRepo(db))
	stage = 3
	if err != nil {
		return fail()
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err = control.Ready(checkCtx)
	stage = 4
	if ctx.Err() != nil {
		return 130
	}
	if err != nil || checkCtx.Err() != nil {
		return fail()
	}
	if _, err := io.WriteString(output, "本地合成资料计划与当前来源授权核验通过；未启动队列、模型或积分写入，不等于生产批准。\n"); err != nil {
		return 1
	}
	return 0
}
