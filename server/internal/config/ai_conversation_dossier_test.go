package config

import (
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

func TestConversationDossierConfig(t *testing.T) {
	good := AITaskReadConfig{Enabled: true, Auth: memberauth.Config{SigningKey: strings.Repeat("k", 40), Issuer: "GVA", Audience: "GVA"}}
	for _, c := range []struct {
		name, flag, conversation string
		dossiers                 bool
		read                     AITaskReadConfig
		want, bad                bool
	}{
		{"默认关闭", "", "", false, AITaskReadConfig{}, false, false}, {"显式关闭", "false", "", false, AITaskReadConfig{}, false, false},
		{"独立开启", "true", "true", true, good, true, false}, {"非法值", "yes", "true", true, good, false, true},
		{"缺档案", "true", "true", false, good, false, true}, {"缺会话", "true", "false", true, good, false, true},
		{"缺会员读取", "true", "true", true, AITaskReadConfig{}, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TM_AI_LOCAL_CONVERSATION_DOSSIER_ENABLED", c.flag)
			t.Setenv("TM_AI_CONVERSATIONS_ENABLED", c.conversation)
			got, e := LoadAILocalConversationDossiers(c.dossiers, c.read)
			if got != c.want || (e != nil) != c.bad {
				t.Fatal("引用配置边界不符", got, e)
			}
		})
	}
}
