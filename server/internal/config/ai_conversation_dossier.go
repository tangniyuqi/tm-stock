package config

import (
	"os"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

// 引用保存独立默认关闭，必须已有档案读取与本人会话，不开启模型或任务。
func LoadAILocalConversationDossiers(dossiersEnabled bool, read AITaskReadConfig) (bool, error) {
	raw := os.Getenv("TM_AI_LOCAL_CONVERSATION_DOSSIER_ENABLED")
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, memberauth.ErrInvalidConfig
	}
	if !enabled {
		return false, nil
	}
	conversations, err := LoadAIConversations(read)
	if err != nil || !dossiersEnabled || !conversations {
		return false, memberauth.ErrInvalidConfig
	}
	return true, nil
}
