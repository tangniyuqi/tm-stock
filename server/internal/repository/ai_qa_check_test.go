package repository

import (
	"context"
	"database/sql"
	"github.com/tangniyuqi/tm-stock/server/migrations"
	"strings"
	"testing"
)

func TestQATriggerDefinitionsUseMigrationSingleSource(t *testing.T) {
	defs, err := qaTriggerDefinitions(migrations.QASchema)
	if err != nil || len(defs) != 15 {
		t.Fatal("问答DDL未完整解析", err)
	}
	if qaCompactSQL("BEGIN IF X = 'Case Value' THEN; END") != qaCompactSQL("begin\nif `X`='Case Value' then;end") {
		t.Fatal("SQL格式化比对错误")
	}
	if qaCompactSQL("'Case Value'") == qaCompactSQL("'casevalue'") {
		t.Fatal("不能抹掉引用内的语义差异")
	}
	if qaCompactSQL("SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='schema-probe';") != qaCompactSQL("SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='schema-probe'") {
		t.Fatal("单句结尾分号被误作定义变化")
	}
	for _, raw := range []string{"", strings.Replace(migrations.QASchema, "FOR EACH ROW", "FOR ROW", 1), strings.Replace(migrations.QASchema, "BEFORE INSERT ON", "AFTER INSERT ON", 1), strings.Replace(migrations.QASchema, "END$$", "END", -1)} {
		if _, err := qaTriggerDefinitions(raw); err == nil {
			t.Fatal("畸形编译期DDL未拒绝")
		}
	}
	if NewAIQASchemaVerifier(nil).Verify(context.Background()) == nil {
		t.Fatal("空数据库验证")
	}
	db, err := sql.Open("mysql", "test@tcp(127.0.0.1:1)/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if NewAIQASchemaVerifier(db).VerifyRead(ctx) == nil || NewAIQASchemaVerifier(db).Verify(nil) == nil {
		t.Fatal("取消仍通过保护检查")
	}
}
