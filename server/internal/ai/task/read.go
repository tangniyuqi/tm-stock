package task

import (
	"context"
	"time"
)

// ReadStore 只读事务由服务层选择范围；历史与详情永远带可信会员编号。
type ReadStore interface {
	WithinTaskRead(context.Context, func(ReadTx) error) error
}
type ReadTx interface {
	Task(context.Context, int64, int64) (Task, error)
	History(context.Context, int64, int64, int) ([]Task, error)
	Events(context.Context, int64, int64, int64, int64, int) ([]EventRecord, error)
}

// EventRecord 仅为只读序列补充真实 UTC 创建时点，不改变既有事件写入契约。
type EventRecord struct {
	Event
	CreatedAt time.Time
}
