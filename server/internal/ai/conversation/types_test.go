package conversation

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func testRecord() Record {
	now := time.UnixMilli(1792281600000)
	return Record{ID: 1, UserID: 42, Key: "conversation_test_key", RequestHash: CreationHash("测试会话"), Title: "测试会话", Version: 1, CreatedAt: now, UpdatedAt: now}
}
func TestConversationTextAndRecord(t *testing.T) {
	for _, tc := range []struct {
		text  string
		limit int
		want  bool
	}{{"测试会话", 40, true}, {strings.Repeat("🙂", 40), 40, true}, {strings.Repeat("🙂", 41), 40, false}, {"", 40, false}, {" 前置空格", 40, false}, {"后置空格\u3000", 40, false}, {"\xff", 40, false}, {"换\n行", 40, false}, {"控制\x00字", 40, false}, {"资料？", 1000, true}} {
		if got := Text(tc.text, tc.limit); got != tc.want {
			t.Fatal("文本规则不符")
		}
	}
	r := testRecord()
	if !r.Valid() {
		t.Fatal("正常记录无效")
	}
	for _, change := range []func(*Record){func(r *Record) { r.ID = 0 }, func(r *Record) { r.UserID = MaxID + 1 }, func(r *Record) { r.Title = "" }, func(r *Record) { r.RequestHash = "bad" }, func(r *Record) { r.Key = "short" }, func(r *Record) { r.LastSeq = 1 }, func(r *Record) { r.Archived = true }, func(r *Record) { r.UpdatedAt = r.CreatedAt.Add(-time.Millisecond) }, func(r *Record) { r.CreatedAt = r.CreatedAt.Add(time.Nanosecond) }} {
		r := testRecord()
		change(&r)
		if r.Valid() {
			t.Fatal("接受坏记录")
		}
	}
	if CreationHash("测试会话") != CreationHash("测试会话") || CreationHash("测试会话") == CreationHash("测试会话2") {
		t.Fatal("创建摘要不稳定")
	}
	if fmt.Sprintf("%v %#v", r, r) != "私有会话（输入及身份已隐藏） 私有会话（输入及身份已隐藏）" {
		t.Fatal("诊断输出泄漏")
	}
}

func TestConversationMessageClosedShape(t *testing.T) {
	base := MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "material_qa", Question: "测试资料有什么出处？"}
	for _, change := range []func(*MessageInput){func(m *MessageInput) { m.Kind = "assistant" }, func(m *MessageInput) { m.SubjectType = "stock" }, func(m *MessageInput) { m.SubjectID = 0 }, func(m *MessageInput) { m.Skill = "unregistered" }, func(m *MessageInput) { m.Question = "" }, func(m *MessageInput) { m.TaskID = 9 }, func(m *MessageInput) { m.Question = strings.Repeat("中", 1001) }} {
		v := base
		change(&v)
		if v.Valid() {
			t.Fatal("接受越界消息")
		}
	}
	for _, m := range []MessageInput{base, {Kind: "question", SubjectType: "topic", SubjectID: 7, Skill: "report_qa", Question: "测试报告出处？", TaskID: 9}, {Kind: "task", SubjectType: "theme", SubjectID: 7, Skill: "light", TaskID: 9}, {Kind: "task", SubjectType: "theme", SubjectID: 7, Skill: "deep", TaskID: 9}} {
		if !m.Valid() {
			t.Fatal("正常消息被拒")
		}
		hash, err := m.Digest(42, 1)
		if err != nil {
			t.Fatal(err)
		}
		v := Message{ConversationID: 1, UserID: 42, Seq: 1, Key: "conversation_message_test", RequestHash: hash, Input: m, CreatedAt: time.UnixMilli(1792281600000)}
		if !v.Valid() {
			t.Fatal("正常消息无效")
		}
		v.RequestHash = strings.Repeat("0", 64)
		if v.Valid() {
			t.Fatal("接受摘要错绑")
		}
		other, err := m.Digest(43, 1)
		if err != nil || other == hash {
			t.Fatal("摘要不绑定用户")
		}
		if strings.Contains(fmt.Sprintf("%#v %#v", v, m), "测试报告出处") {
			t.Fatal("输入泄漏")
		}
	}
	if _, err := base.Digest(0, 1); err == nil {
		t.Fatal("非法摘要身份")
	}
	invalid := base
	invalid.Kind = "raw"
	if _, err := invalid.Digest(42, 1); err == nil {
		t.Fatal("坏消息摘要")
	}
}
