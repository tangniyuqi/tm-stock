package repository

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"strings"
	"testing"
)

func TestAdmissionRepositoryAndVerifierFailClosed(t *testing.T) {
	ctx := context.Background()
	var repo *AITaskRepo
	if repo.WithinAdmissionTx(ctx, func(task.AdmissionReadTx) error { return nil }) != points.ErrInvalid {
		t.Fatal("空仓储放行")
	}
	if NewAITaskRepo(nil).WithinAdmissionTx(ctx, nil) != points.ErrInvalid {
		t.Fatal("空回调放行")
	}
	for _, tx := range []*aiTaskTx{nil, {}} {
		if _, err := tx.InsertAdmittedTask(ctx, task.Task{}, strings.Repeat("a", 64)); err != points.ErrInvalid {
			t.Fatal(err)
		}
		if err := tx.InsertAdmission(ctx, task.AdmissionRecord{}); err != points.ErrInvalid {
			t.Fatal(err)
		}
		if _, _, err := tx.ReadAdmission(ctx, task.Task{}); err != points.ErrInvalid {
			t.Fatal(err)
		}
	}
	var verifier *AIAdmissionSchemaVerifier
	if verifier.Verify(ctx) != points.ErrInvalid || NewAIAdmissionSchemaVerifier(nil).Verify(ctx) != points.ErrInvalid {
		t.Fatal("空数据库自检放行")
	}
	ddl := readMigration(t, "20261018_create_tm_ai_task_admissions.sql")
	for _, required := range []string{"admission_digest char(64)", "tm_ai_task_admission_identity", "chk_tm_ai_task_admission", "fk_tm_ai_admission_task", "tm_ai_admission_insert", "tm_ai_admission_no_update", "tm_ai_admission_no_delete", "plan_digest=SHA2(payload,256)", "h.expire_at=t.deadline_at"} {
		if !strings.Contains(ddl, required) {
			t.Fatal("迁移缺少保护", required)
		}
	}
	if strings.Contains(ddl, "UPDATE tm_ai_tasks SET admission_digest") || strings.Contains(ddl, "INSERT INTO tm_ai_task_admissions SELECT") {
		t.Fatal("迁移补造历史批准")
	}
}
