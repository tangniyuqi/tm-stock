package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"strings"
	"testing"
)

func TestAIConversationRepoMissingAndCanceled(t *testing.T) {
	if NewAIConversationSchemaVerifier(nil).Verify(context.Background()) == nil {
		t.Fatal("空探针")
	}
	if NewAILocalConversationSchemaVerifier(nil, strings.Repeat("a", 32)).Verify(context.Background()) == nil {
		t.Fatal("本地空探针")
	}
	var absent *AIConversationRepo
	if absent.Within(context.Background(), true, func(conversation.Tx) error { return nil }) == nil {
		t.Fatal("空仓储")
	}
	db, err := sql.Open("mysql", "test@tcp(127.0.0.1:1)/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if NewAIConversationRepo(db).Within(ctx, false, func(conversation.Tx) error { return nil }) == nil {
		t.Fatal("取消仍开启事务")
	}
	if NewAIConversationSchemaVerifier(db).Verify(ctx) == nil {
		t.Fatal("取消仍探针成功")
	}
	if NewAIConversationSchemaVerifier(db).Verify(nil) == nil {
		t.Fatal("空上下文")
	}
	if NewAILocalConversationSchemaVerifier(db, "").Verify(context.Background()) == nil || NewAILocalConversationSchemaVerifier(db, strings.Repeat("a", 32)).Verify(ctx) == nil {
		t.Fatal("本地标记无效或取消仍成功")
	}
	if NewAIConversationRepoWithIsolation(db, sql.LevelSerializable).Within(ctx, false, func(conversation.Tx) error { return nil }) == nil {
		t.Fatal("未验证隔离级别")
	}
}

func TestAIConversationSchemaErrorHidesDetails(t *testing.T) {
	private := errors.New("仅测试内部SQL详情")
	err := &AIConversationSchemaError{Stage: 2, cause: private}
	if !errors.Is(err, private) || strings.Contains(err.Error(), private.Error()) || !strings.Contains(err.Error(), "阶段2") {
		t.Fatal("保护探针错误未保留类型或泄漏详情")
	}
}
