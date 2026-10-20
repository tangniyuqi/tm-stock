package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
)

func TestAIReportRepositoryAndVerifierFailClosed(t *testing.T) {
	ctx := context.Background()
	var missing *AIReportSchemaVerifier
	for _, v := range []*AIReportSchemaVerifier{missing, NewAIReportSchemaVerifier(nil)} {
		if err := v.Verify(ctx); err == nil {
			t.Fatal("空报告自检放行")
		}
	}
	db, err := sql.Open("mysql", "test_user@tcp(127.0.0.1:1)/test?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if NewAIReportSchemaVerifier(db).Verify(cancelled) == nil {
		t.Fatal("取消自检放行")
	}
	if NewAITaskRepo(nil).WithinReportTx(ctx, nil) == nil {
		t.Fatal("空回调放行")
	}
	var tx aiTaskTx
	for _, ids := range [][]int64{nil, {0}, {2, 1}, {1, 1}} {
		if _, _, err := tx.ReportEvidence(ctx, ids, 1); err == nil {
			t.Fatal("坏引用访问数据库")
		}
	}
	if tx.InsertReportCommit(ctx, finalreport.Commit{}) == nil {
		t.Fatal("坏提交访问数据库")
	}
	if _, err := sourceByIDWithMode(ctx, nil, 1, sourceLockMode(99)); err == nil {
		t.Fatal("来源锁模式未封闭")
	}
	if _, err := sourceSnapshotByID(ctx, nil, 0, true); err == nil {
		t.Fatal("坏快照编号访问数据库")
	}
}
