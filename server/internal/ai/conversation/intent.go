package conversation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

var ErrUnconfirmed = errors.New("会话任务未确认，请核查原意图")

const (
	Prepared    = "prepared"
	Unconfirmed = "unconfirmed"
	Linked      = "linked"
	NotCreated  = "not_created"
)

type IntentInput struct {
	AnalysisType string `json:"analysis_type"`
	SubjectType  string `json:"subject_type"`
	SubjectID    int64  `json:"subject_id"`
	PriceVersion string `json:"price_version"`
}

func (i IntentInput) Valid() bool {
	return (i.AnalysisType == "light" || i.AnalysisType == "deep") && (i.SubjectType == "theme" || i.SubjectType == "topic") && ID(i.SubjectID) && (points.Price{Version: i.PriceVersion, Points: 1}).Validate() == nil
}
func (i IntentInput) Digest(uid, cid int64) (string, error) {
	if !ID(uid) || !ID(cid) || !i.Valid() {
		return "", ErrInvalid
	}
	raw, err := json.Marshal(struct {
		Version                string
		UserID, ConversationID int64
		Input                  IntentInput
	}{"conversation-task-v1", uid, cid, i})
	if err != nil {
		return "", ErrInvalid
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}
func NewTaskKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "conv_" + hex.EncodeToString(b[:]), nil
}
func Rejection(code string) bool {
	return code == "price_changed" || code == "insufficient_points" || code == "creation_disabled" || code == "admission_unavailable" || code == "invalid_request"
}

type Intent struct {
	ID, ConversationID, UserID  int64
	Key, RequestHash, TaskKey   string
	Input                       IntentInput
	PricePoints                 int64
	State, RejectionCode        string
	Version, TaskID, MessageSeq int64
	CreatedAt, UpdatedAt        time.Time
}

func (Intent) String() string     { return "会话任务意图（内部键及身份已隐藏）" }
func (i Intent) GoString() string { return i.String() }
func (i Intent) Valid() bool {
	if !ID(i.ID) || !ID(i.UserID) || !ID(i.ConversationID) || !points.ValidKey(i.Key) || !points.ValidKey(i.TaskKey) || !ID(i.PricePoints) || !Time(i.CreatedAt) || !Time(i.UpdatedAt) || i.UpdatedAt.Before(i.CreatedAt) {
		return false
	}
	h, err := i.Input.Digest(i.UserID, i.ConversationID)
	if err != nil || h != i.RequestHash {
		return false
	}
	switch i.State {
	case Prepared:
		return i.Version == 1 && i.TaskID == 0 && i.MessageSeq == 0 && i.RejectionCode == "" && i.CreatedAt.Equal(i.UpdatedAt)
	case Unconfirmed:
		return i.Version == 2 && i.TaskID == 0 && i.MessageSeq == 0 && i.RejectionCode == ""
	case Linked:
		return i.Version == 3 && ID(i.TaskID) && ID(i.MessageSeq) && i.RejectionCode == ""
	case NotCreated:
		return i.Version == 3 && i.TaskID == 0 && i.MessageSeq == 0 && Rejection(i.RejectionCode)
	}
	return false
}

type TaskIdentity struct {
	ID, UserID       int64
	Key, RequestHash string
	AnalysisType     int
	SubjectType      string
	SubjectID        int64
	PriceVersion     string
	PricePoints      int64
}

func (TaskIdentity) String() string     { return "意图任务身份（内部键已隐藏）" }
func (v TaskIdentity) GoString() string { return v.String() }
func (v TaskIdentity) Matches(i Intent) bool {
	level := 1
	if i.Input.AnalysisType == "deep" {
		level = 2
	}
	if !i.Valid() || !ID(v.ID) || v.UserID != i.UserID || v.Key != i.TaskKey || v.AnalysisType != level || v.SubjectType != i.Input.SubjectType || v.SubjectID != i.Input.SubjectID || v.PriceVersion != i.Input.PriceVersion || v.PricePoints != i.PricePoints {
		return false
	}
	h, err := points.RequestHash("ai_task_create", task.CreateRequest{UserID: v.UserID, Key: v.Key, AnalysisType: v.AnalysisType, SubjectType: v.SubjectType, SubjectID: v.SubjectID, PriceVersion: v.PriceVersion})
	return err == nil && h == v.RequestHash
}

type IntentStore interface {
	WithinIntents(context.Context, bool, func(IntentTx) error) error
}
type IntentTx interface {
	Tx
	IntentByID(context.Context, int64, int64, int64, bool) (Intent, error)
	IntentByKey(context.Context, int64, int64, string) (Intent, error)
	InsertIntent(context.Context, Intent) (Intent, error)
	SetIntent(context.Context, Intent, Intent) error
	Intents(context.Context, int64, int64, int64, int) ([]Intent, error)
	IntentTask(context.Context, int64, string, bool) (TaskIdentity, error)
}
