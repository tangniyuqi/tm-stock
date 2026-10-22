package repository

import (
	"context"
	"database/sql"
	"testing"
)

func TestAITaskStreamSchemaMissingAndCanceled(t *testing.T) {
	if err := VerifyAITaskStreamReadSchema(context.Background(), nil); err == nil {
		t.Fatal("空依赖")
	}
	db, err := sql.Open("mysql", "test@tcp(127.0.0.1:1)/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyAITaskStreamReadSchema(ctx, db); err == nil {
		t.Fatal("取消后仍开始检查")
	}
	if err := VerifyAITaskStreamReadSchema(nil, db); err == nil {
		t.Fatal("空上下文")
	}
}
