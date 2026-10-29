package repository

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AILocalInstanceVerifier 只读核对本地隔离实例；不自动创建标记，不授予生产批准。
type AILocalInstanceVerifier struct{ db *sql.DB }

func NewAILocalInstanceVerifier(db *sql.DB) *AILocalInstanceVerifier {
	return &AILocalInstanceVerifier{db: db}
}

func (v *AILocalInstanceVerifier) Verify(parent context.Context, instanceID string) error {
	invalid := func() error {
		return fmt.Errorf("本地AI数据库隔离标记未确认（详情已隐藏）: %w", points.ErrInvariant)
	}
	b, err := hex.DecodeString(instanceID)
	if parent == nil || v == nil || v.db == nil || err != nil || len(b) != 16 || hex.EncodeToString(b) != instanceID {
		return invalid()
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return invalid()
	}
	defer tx.Rollback() // 所有路径仅回滚只读事务；不修复或生成标记。
	var database, server string
	if err := tx.QueryRowContext(ctx, "SELECT DATABASE(), @@server_uuid").Scan(&database, &server); err != nil || database != "tm_ai_local_"+instanceID || len(server) != 36 {
		return invalid()
	}
	rows, err := tx.QueryContext(ctx, "SELECT instance_id, database_name, server_uuid, environment, contains_real_users FROM tm_ai_local_instance LIMIT 2")
	if err != nil {
		return invalid()
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, name, uuid, environment string
		var realUsers int
		if rows.Scan(&id, &name, &uuid, &environment, &realUsers) != nil || id != instanceID || name != database || uuid != server || environment != "local-isolated" || realUsers != 0 {
			return invalid()
		}
		count++
	}
	if rows.Err() != nil || count != 1 {
		return invalid()
	}
	if err := rows.Close(); err != nil {
		return invalid()
	}
	if err := tx.Rollback(); err != nil {
		return invalid()
	}
	return ctx.Err()
}
