package conversation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func validIntent(t *testing.T) Intent {
	t.Helper()
	in := IntentInput{"light", "theme", 1, "test-v1"}
	hash, err := in.Digest(42, 1)
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewTaskKey()
	if err != nil {
		t.Fatal(err)
	}
	return Intent{ID: 1, ConversationID: 1, UserID: 42, Key: "intent_domain_test", RequestHash: hash, TaskKey: key, Input: in, PricePoints: 7, State: Prepared, Version: 1, CreatedAt: time.UnixMilli(1792281600000), UpdatedAt: time.UnixMilli(1792281600000)}
}
func TestAIConversationIntentValidation(t *testing.T) {
	i := validIntent(t)
	if !i.Valid() || !points.ValidKey(i.TaskKey) {
		t.Fatal("有效意图被拒绝")
	}
	for _, change := range []func(*Intent){func(i *Intent) { i.ID = 0 }, func(i *Intent) { i.Key = "bad" }, func(i *Intent) { i.TaskKey = "bad" }, func(i *Intent) { i.PricePoints = 0 }, func(i *Intent) { i.RequestHash = "" }, func(i *Intent) { i.Input.AnalysisType = "other" }, func(i *Intent) { i.Input.SubjectType = "stock" }, func(i *Intent) { i.Input.SubjectID = 0 }, func(i *Intent) { i.Input.PriceVersion = " " }, func(i *Intent) { i.State = "other" }, func(i *Intent) { i.Version = 2 }, func(i *Intent) { i.TaskID = 1 }, func(i *Intent) { i.RejectionCode = "unknown" }, func(i *Intent) { i.UpdatedAt = i.UpdatedAt.Add(-time.Millisecond) }} {
		bad := i
		change(&bad)
		if bad.Valid() {
			t.Fatal("无效意图被接受")
		}
	}
	for _, state := range []string{Unconfirmed, Linked, NotCreated} {
		v := i
		v.State = state
		v.Version = 2
		if state != Unconfirmed {
			v.Version = 3
		}
		if state == Linked {
			v.TaskID = 40
			v.MessageSeq = 1
		}
		if state == NotCreated {
			v.RejectionCode = "price_changed"
		}
		if !v.Valid() {
			t.Fatal(state)
		}
		v.Version++
		if v.Valid() {
			t.Fatal("错误状态版本")
		}
	}
	if _, err := i.Input.Digest(0, 1); err == nil {
		t.Fatal("零用户")
	}
	if _, err := i.Input.Digest(42, 0); err == nil {
		t.Fatal("零会话")
	}
	for _, v := range []any{i, &i, TaskIdentity{Key: i.TaskKey}} {
		if strings.Contains(fmt.Sprintf("%v %#v", v, v), i.TaskKey) {
			t.Fatal("格式化泄漏任务键")
		}
	}
	next, err := NewTaskKey()
	if err != nil || next == i.TaskKey {
		t.Fatal("随机键重复")
	}
}
func TestAIConversationIntentTaskIdentity(t *testing.T) {
	for _, level := range []int{1, 2} {
		i := validIntent(t)
		if level == 2 {
			i.Input.AnalysisType = "deep"
			i.RequestHash, _ = i.Input.Digest(i.UserID, i.ConversationID)
		}
		v := TaskIdentity{ID: 40, UserID: i.UserID, Key: i.TaskKey, AnalysisType: level, SubjectType: i.Input.SubjectType, SubjectID: i.Input.SubjectID, PriceVersion: i.Input.PriceVersion, PricePoints: i.PricePoints}
		var err error
		v.RequestHash, err = points.RequestHash("ai_task_create", task.CreateRequest{UserID: v.UserID, Key: v.Key, AnalysisType: level, SubjectType: v.SubjectType, SubjectID: v.SubjectID, PriceVersion: v.PriceVersion})
		if err != nil || !v.Matches(i) {
			t.Fatal("有效身份不匹配", err)
		}
		for _, mutate := range []func(*TaskIdentity){func(v *TaskIdentity) { v.ID = 0 }, func(v *TaskIdentity) { v.UserID++ }, func(v *TaskIdentity) { v.Key += "x" }, func(v *TaskIdentity) { v.AnalysisType++ }, func(v *TaskIdentity) { v.SubjectType = "topic" }, func(v *TaskIdentity) { v.SubjectID++ }, func(v *TaskIdentity) { v.PriceVersion += "x" }, func(v *TaskIdentity) { v.PricePoints++ }, func(v *TaskIdentity) { v.RequestHash = "" }} {
			bad := v
			mutate(&bad)
			if bad.Matches(i) {
				t.Fatal("错误身份被接受")
			}
		}
	}
}
