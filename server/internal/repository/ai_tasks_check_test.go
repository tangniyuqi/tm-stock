package repository

import (
	"context"
	"database/sql"
	"testing"
)

func TestAITaskSchemaVerifierFailsClosed(t *testing.T) {
	var absent *AITaskSchemaVerifier
	for _, verifier := range []*AITaskSchemaVerifier{absent, NewAITaskSchemaVerifier(nil)} {
		if err := verifier.Verify(context.Background()); err == nil {
			t.Fatal("空任务自检放行")
		}
	}
	db, err := sql.Open("mysql", "test_user@tcp(127.0.0.1:1)/test?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewAITaskSchemaVerifier(db).Verify(ctx); err == nil {
		t.Fatal("已取消自检放行")
	}
}
