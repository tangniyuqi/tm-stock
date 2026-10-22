package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
)

func TestQAStoredJSONAcceptsCanonicalArraysButRejectsAmbiguity(t *testing.T) {
	for _, raw := range []string{`[]`, `null`, `[{"ID":1,"Hash":"test"}]`} {
		var v []qa.EvidenceProof
		if strictQAJSON([]byte(raw), &v) != nil {
			t.Fatal("合法存证数组被对象根限制拒绝")
		}
	}
	for _, raw := range []string{`[{"ID":1,"ID":2,"Hash":"test"}]`, `[{"ID":1,"id":1,"Hash":"test"}]`, `[{"ID":1,"Hash":"test","extra":1}]`, `[{"ID":1,"Hash":"test"}] []`, strings.Repeat(" ", 16385)} {
		var v []qa.EvidenceProof
		if strictQAJSON([]byte(raw), &v) == nil {
			t.Fatal("歧义/未知/过大存证通过")
		}
	}
	if sameQAJSON(make(chan int), nil) || sameQAJSON(nil, make(chan int)) {
		t.Fatal("编码失败被认成相等")
	}
}
func TestQARepoMissingOrCanceled(t *testing.T) {
	var r *AIQARepo
	if r.WithinQA(context.Background(), false, func(qa.Tx) error { return nil }) == nil {
		t.Fatal("空仓储")
	}
	db, err := sql.Open("mysql", "test@tcp(127.0.0.1:1)/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if NewAIQARepo(db).WithinQA(ctx, false, func(qa.Tx) error { return nil }) == nil {
		t.Fatal("取消仍开启事务")
	}
	if NewAIQARepoWithIsolation(db, sql.LevelSerializable).WithinQA(ctx, false, func(qa.Tx) error { return nil }) == nil {
		t.Fatal("未验隔离级别")
	}
}
