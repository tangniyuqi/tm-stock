package main

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"net/http"
	"testing"
)

func TestAIConversationTaskStartupClosed(t *testing.T) {
	if err := registerAIConversationTasks(nil, nil, nil, config.AITaskReadConfig{}, nil, false, false); err != nil {
		t.Fatal(err)
	}
	if err := registerAIConversationTasks(context.Background(), http.NewServeMux(), nil, config.AITaskReadConfig{}, nil, true, true); err == nil {
		t.Fatal("缺少原创建器和数据库未拒绝")
	}
	if err := registerAIConversationTasksWithLocal(nil, nil, nil, config.AITaskReadConfig{}, nil, false, false, &config.AILocalRuntime{}); err != nil {
		t.Fatal("关闭时不应装配本地写入口")
	}
	if err := registerAIConversationTasksWithLocal(context.Background(), http.NewServeMux(), nil, config.AITaskReadConfig{}, nil, true, true, &config.AILocalRuntime{}); err == nil {
		t.Fatal("本地路径缺原创建器未拒绝")
	}
}
