// Package conversation 定义私有会话与不可变历史，不执行模型或账务。
package conversation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const MaxID int64 = 9007199254740991

var (
	ErrInvalid   = errors.New("会话输入无效")
	ErrNotFound  = errors.New("会话不存在")
	ErrConflict  = errors.New("会话版本或原键冲突")
	ErrInvariant = errors.New("会话或消息结构不一致")
)

func ID(id int64) bool { return id > 0 && id <= MaxID }
func Text(value string, max int) bool {
	if !utf8.ValidString(value) || value == "" || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > max {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func Time(t time.Time) bool {
	return t.UnixMilli() > 0 && t.UnixMilli() <= 8640000000000000 && t.Nanosecond()%int(time.Millisecond) == 0
}

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func CreationHash(title string) string {
	// 仅一个严格校验的标题，不做隐式裁剪或Unicode重写；与可变标题分开保留。
	h := sha256.Sum256([]byte("conversation-create-v1\x00" + title))
	return hex.EncodeToString(h[:])
}

type Record struct {
	ID, UserID              int64
	Key, RequestHash, Title string
	Archived                bool
	Version, LastSeq        int64
	CreatedAt, UpdatedAt    time.Time
}

func (Record) String() string     { return "私有会话（输入及身份已隐藏）" }
func (r Record) GoString() string { return r.String() }
func (r Record) Valid() bool {
	return ID(r.ID) && ID(r.UserID) && points.ValidKey(r.Key) && hashPattern.MatchString(r.RequestHash) && Text(r.Title, 40) && ID(r.Version) && r.LastSeq >= 0 && r.LastSeq < MaxID && r.Version > r.LastSeq && (!r.Archived || r.Version >= 2) && Time(r.CreatedAt) && Time(r.UpdatedAt) && !r.UpdatedAt.Before(r.CreatedAt)
}

// MessageInput 没有任意assistant正文、模型或来源URL；问题只是私有用户输入。
type MessageInput struct {
	Kind        string `json:"kind"`
	SubjectType string `json:"subjectType"`
	SubjectID   int64  `json:"subjectId"`
	Skill       string `json:"skill"`
	Question    string `json:"question"`
	TaskID      int64  `json:"taskId"`
}

func (MessageInput) String() string     { return "会话消息输入（正文已隐藏）" }
func (m MessageInput) GoString() string { return m.String() }
func (m MessageInput) Valid() bool {
	if (m.SubjectType != "theme" && m.SubjectType != "topic") || !ID(m.SubjectID) {
		return false
	}
	switch m.Kind {
	case "question":
		return Text(m.Question, 1000) && len(m.Question) <= 4000 && (m.Skill == "material_qa" && m.TaskID == 0 || m.Skill == "report_qa" && ID(m.TaskID))
	case "task":
		return m.Question == "" && ID(m.TaskID) && (m.Skill == "light" || m.Skill == "deep")
	}
	return false
}
func (m MessageInput) Digest(uid, cid int64) (string, error) {
	if !ID(uid) || !ID(cid) || !m.Valid() {
		return "", ErrInvalid
	}
	raw, err := json.Marshal(struct {
		Version                string
		UserID, ConversationID int64
		Input                  MessageInput
	}{"conversation-message-v1", uid, cid, m})
	if err != nil {
		return "", ErrInvalid
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}

type Message struct {
	ConversationID, UserID, Seq int64
	Key, RequestHash            string
	Input                       MessageInput
	CreatedAt                   time.Time
}

func (Message) String() string     { return "私有会话消息（输入及身份已隐藏）" }
func (m Message) GoString() string { return m.String() }
func (m Message) Valid() bool {
	if !ID(m.ConversationID) || !ID(m.UserID) || !ID(m.Seq) || !points.ValidKey(m.Key) || !Time(m.CreatedAt) {
		return false
	}
	h, err := m.Input.Digest(m.UserID, m.ConversationID)
	return err == nil && h == m.RequestHash
}

type Store interface {
	Within(context.Context, bool, func(Tx) error) error
}
type Tx interface {
	Now(context.Context) (time.Time, error)
	CreateOrGet(context.Context, Record) (Record, error)
	ByID(context.Context, int64, int64, bool) (Record, error)
	ByKey(context.Context, int64, string) (Record, error)
	List(context.Context, int64, int64, bool, int) ([]Record, error)
	Update(context.Context, Record, Record) error
	Messages(context.Context, int64, int64, int64, int64, int) ([]Message, error)
	// Append仅供未来受控问答/意图服务在同一会话锁事务使用；没有公共写历史入口。
	Append(context.Context, Message) error
}
