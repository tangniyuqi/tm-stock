package dto

// 引用元数据只证明本人曾保存；不含资料标题、原文、可读性或许可。
const AIConversationDossierNotice = "以下仅为本人保存的版本引用；保存记录不代表资料当前可读，打开时重新核验权限。"

type AIConversationDossier struct {
	ID             int64 `json:"id"`
	ConversationID int64 `json:"conversationId"`
	ThemeID        int64 `json:"themeId"`
	Version        int64 `json:"version"`
	CompareVersion int64 `json:"compareVersion"`
	CreatedAt      int64 `json:"createdAt"`
}

type AIConversationDossierPage struct {
	ConversationID int64                   `json:"conversationId"`
	Notice         string                  `json:"notice"`
	Items          []AIConversationDossier `json:"items"`
	NextBeforeID   int64                   `json:"nextBeforeId"`
	HasMore        bool                    `json:"hasMore"`
}

type AIConversationDossierView struct {
	Reference AIConversationDossier `json:"reference"`
	Dossier   AIDossierView         `json:"dossier"`
}
