package config

import (
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"strings"
	"testing"
)

func TestAIConversationTaskConfigClosed(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, tc := range []struct {
		raw                      string
		read                     AITaskReadConfig
		conversations, want, bad bool
	}{{"", good, false, false, false}, {"false", AITaskReadConfig{}, false, false, false}, {"true", good, true, true, false}, {"true", good, false, false, true}, {"yes", good, true, false, true}, {"true", AITaskReadConfig{}, true, false, true}, {"true", AITaskReadConfig{Enabled: true}, true, false, true}} {
		t.Setenv("TM_AI_CONVERSATION_TASKS_ENABLED", tc.raw)
		t.Setenv("TM_AI_TASK_CREATE_ENABLED", "false")
		got, err := LoadAIConversationTasks(tc.read, tc.conversations)
		if got != tc.want || (err != nil) != tc.bad {
			t.Fatal(got, err)
		}
	}
}
