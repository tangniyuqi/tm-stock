// points-check 核对专用账本结构并用回滚事务验证约束；不会充值、扣费或迁移旧余额。
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
	fmt.Println("账本结构与事务约束探针通过；探针数据已回滚。未启用账务或公开 AI。")
}

func run() error {
	return runCheck(func(ctx context.Context, db *sql.DB) error {
		return repository.NewPointsSchemaVerifier(db).Verify(ctx)
	})
}

func runCheck(verify func(context.Context, *sql.DB) error) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return fmt.Errorf("账本自检数据库配置无效")
	}
	// 关闭连接失败不会使已回滚探针变成提交；不输出驱动错误以免泄露连接信息。
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := verify(ctx, db); err != nil {
		return fmt.Errorf("账本自检未通过；请核对迁移、MySQL 版本、约束和账号权限（驱动详情已隐藏）")
	}
	return nil
}
