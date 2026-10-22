// Package feedback 定义报告反馈闭环；不调用模型，不提供咨询或改账能力。
package feedback

import (
	"context"
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalid   = errors.New("反馈参数无效")
	ErrNotFound  = errors.New("反馈或任务不存在")
	ErrNoReport  = errors.New("该任务没有可反馈的成功报告")
	ErrConflict  = errors.New("反馈版本或操作冲突")
	ErrInvariant = errors.New("反馈记录或处理历史不一致")
)

const MaxID int64 = 9007199254740991

func ID(id int64) bool       { return id > 0 && id <= MaxID }
func Category(s string) bool { return s == "fact" || s == "source" || s == "display" || s == "other" }
func State(status, resolution string, version int64) bool {
	switch status {
	case "received":
		return version == 1 && resolution == ""
	case "processing":
		return version == 2 && resolution == ""
	case "resolved":
		return version == 3 && (resolution == "reviewed" || resolution == "referred")
	case "dismissed":
		return version == 3 && (resolution == "unconfirmed" || resolution == "not_applicable")
	}
	return false
}
func Transition(old, next string) bool {
	return old == "received" && next == "processing" || old == "processing" && (next == "resolved" || next == "dismissed")
}

var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Record struct {
	ID, TaskID, UserID                         int64
	Category, Status, Resolution, ReportDigest string
	Version                                    int64
	CreatedAt, UpdatedAt                       time.Time
}

func (r Record) String() string   { return "报告反馈（身份及内部信息已隐藏）" }
func (r Record) GoString() string { return r.String() }
func (r Record) Valid() bool {
	return ID(r.ID) && ID(r.TaskID) && r.UserID > 0 && Category(r.Category) && State(r.Status, r.Resolution, r.Version) && digest.MatchString(r.ReportDigest) && validTime(r.CreatedAt) && validTime(r.UpdatedAt) && !r.UpdatedAt.Before(r.CreatedAt)
}
func validTime(t time.Time) bool {
	return t.UnixMilli() > 0 && t.UnixMilli() <= 8640000000000000 && t.Nanosecond()%int(time.Millisecond) == 0
}

type Event struct {
	FeedbackID, Version, ActorID int64
	Status, Resolution           string
	At                           time.Time
}

func (e Event) Valid() bool {
	return ID(e.FeedbackID) && State(e.Status, e.Resolution, e.Version) && validTime(e.At) && (e.Version == 1 && e.ActorID == 0 || e.Version > 1 && e.ActorID > 0)
}
func ValidateHistory(r Record, events []Event) error {
	if !r.Valid() || int64(len(events)) != r.Version {
		return ErrInvariant
	}
	for i, e := range events {
		if !e.Valid() || e.FeedbackID != r.ID || e.Version != int64(i+1) || i == 0 && !e.At.Equal(r.CreatedAt) || i > 0 && (!Transition(events[i-1].Status, e.Status) || e.At.Before(events[i-1].At)) {
			return ErrInvariant
		}
	}
	last := events[len(events)-1]
	if last.Status != r.Status || last.Resolution != r.Resolution || !last.At.Equal(r.UpdatedAt) {
		return ErrInvariant
	}
	return nil
}

type Decision struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Status          string `json:"status"`
	Resolution      string `json:"resolution"`
}

func (d Decision) Valid() bool {
	return d.ExpectedVersion >= 1 && d.ExpectedVersion <= 2 && State(d.Status, d.Resolution, d.ExpectedVersion+1)
}

type Store interface {
	Within(context.Context, func(Tx) error) error
}
type Tx interface {
	Now(context.Context) (time.Time, error)
	OwnedReport(context.Context, int64, int64) (string, error)
	ByCategory(context.Context, int64, int64, string) (Record, error)
	ByID(context.Context, int64) (Record, error)
	Insert(context.Context, Record) (Record, error)
	Update(context.Context, Record, Record) error
	Append(context.Context, Event) error
	History(context.Context, int64) ([]Event, error)
	ForReport(context.Context, int64, int64) ([]Record, error)
	List(context.Context, int64, int) ([]Record, error)
}
