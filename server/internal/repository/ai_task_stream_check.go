package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// VerifyAITaskStreamReadSchema 复用原任务元数据契约，只SELECT，不执行账本写探针。
// 它不证明历史账务或被删除约束的实际拒写效果；事件服务仍逐页校验实际数据。
func VerifyAITaskStreamReadSchema(parent context.Context, db *sql.DB) error {
	if parent == nil || db == nil {
		return fmt.Errorf("任务进度读取数据库未配置")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("开启任务进度结构检查失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 已提交/上下文取消后不再保留事务。
	if err := verifyPointsSession(ctx, tx); err != nil {
		return err
	}
	if err := verifyAITaskMetadata(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("结束任务进度结构检查失败: %w", err)
	}
	return nil
}
