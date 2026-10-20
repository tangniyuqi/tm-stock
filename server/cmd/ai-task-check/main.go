// ai-task-check 验证任务表及账本依赖，随机写入约束探针后全部回滚。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("任务与账本结构及约束探针通过；探针数据已回滚。未启用 worker、收费或公开 AI。")
}

func run() error {
	return runCheck(func(ctx context.Context, db *sql.DB) error {
		return repository.NewAITaskSchemaVerifier(db).Verify(ctx)
	})
}

func runCheck(verify func(context.Context, *sql.DB) error) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return fmt.Errorf("任务自检数据库配置无效")
	}
	// 不输出驱动错误或 DSN；关闭连接不会把已回滚探针改成提交。
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := verify(ctx, db); err != nil {
		return fmt.Errorf("任务自检未通过；请核对任务/账本迁移、约束、MySQL 版本和账号权限（驱动详情已隐藏）")
	}
	return nil
}
