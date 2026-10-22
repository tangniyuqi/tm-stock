//go:build integration

package testutil

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewFeedbackMySQLDB仅供反馈专项使用：只接受回环测试实例，另建固定专库。
// 专库已存在则失败，绝不接管或清空；只有本次CREATE成功才登记DROP清理。
// 不使用产品配置，不打印DSN；建表/外键策略只属于测试夹具。
func NewFeedbackMySQLDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置TM_TEST_DSN，反馈MySQL专项未执行")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("反馈MySQL测试DSN无效")
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Net != "tcp" || host != "127.0.0.1" || port == "" || (cfg.DBName != "tm_stock_it" && cfg.DBName != "tm_ai_source_test") {
		t.Fatal("反馈MySQL专项仅允许字面回环地址及指定测试库")
	}
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	cfg.ParseTime = true
	open := func() *gorm.DB {
		t.Helper()
		db, err := gorm.Open(mysql.New(mysql.Config{DSN: cfg.FormatDSN(), DefaultStringSize: 191}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true})
		if err != nil {
			t.Fatal("连接反馈MySQL测试库失败（连接信息不输出）")
		}
		pool, err := db.DB()
		if err != nil {
			t.Fatal("取得反馈MySQL测试连接失败")
		}
		pool.SetMaxOpenConns(12)
		t.Cleanup(func() {
			if err := pool.Close(); err != nil {
				t.Error("关闭反馈MySQL测试连接失败")
			}
		})
		return db
	}
	admin := open()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 数据库标识符固定，不拼接外来标识符；禁止IF NOT EXISTS接管已有资源。
	if err := admin.WithContext(ctx).Exec("CREATE DATABASE tm_feedback_console_it CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci").Error; err != nil {
		t.Fatal("创建反馈专项专库失败；若已存在须人工核对，不自动删除")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := admin.WithContext(ctx).Exec("DROP DATABASE tm_feedback_console_it").Error; err != nil {
			t.Error("清理本次创建的反馈专项专库失败")
		}
	})
	cfg.DBName = "tm_feedback_console_it"
	return open()
}
