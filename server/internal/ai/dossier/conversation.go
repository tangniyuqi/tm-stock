package dossier

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

const MaxConversationReferences = 100

var ErrReferenceLimit = errors.New("会话保存的档案引用已达上限")

// Selection只标识用户选定的不可变版本，不包含正文、批准或模型输入。
type Selection struct {
	ThemeID, Version, CompareVersion int64
}

func (s Selection) Valid() bool {
	return conversation.ID(s.ThemeID) && conversation.ID(s.Version) && (s.CompareVersion == 0 || conversation.ID(s.CompareVersion) && s.CompareVersion > s.Version)
}
func (s Selection) Versions() []int64 {
	if s.CompareVersion == 0 {
		return []int64{s.Version}
	}
	return []int64{s.Version, s.CompareVersion}
}

// ConversationReference是本人曾保存的入口，不证明资料现在可读。
type ConversationReference struct {
	ID, UserID, ConversationID int64
	Selection
	CreatedAt time.Time
}

func (ConversationReference) String() string {
	return "会话档案引用（私有身份与选择已隐藏）"
}
func (r ConversationReference) GoString() string { return r.String() }
func (r ConversationReference) Valid() bool {
	return conversation.ID(r.ID) && conversation.ID(r.UserID) && conversation.ID(r.ConversationID) && r.Selection.Valid() && conversation.Time(r.CreatedAt)
}

type ConversationStore interface {
	WithinReferences(context.Context, bool, func(ConversationTx) error) error
}

// 不提供原会话消息/任务/积分写入；Dossiers也仅借用原读取能力。
type ConversationTx interface {
	Dossiers() Tx
	Conversation(context.Context, int64, int64, bool) (conversation.Record, error)
	Reference(context.Context, int64, int64, int64) (ConversationReference, error)
	ReferenceBySelection(context.Context, int64, int64, Selection) (ConversationReference, error)
	References(context.Context, int64, int64, int64, int) ([]ConversationReference, error)
	ReferenceCount(context.Context, int64, int64) (int, error)
	InsertReference(context.Context, ConversationReference) (ConversationReference, error)
}
