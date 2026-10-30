package dossier

import (
	"reflect"
	"testing"
)

func TestDraftSnapshotSetAndStoredCopy(t *testing.T) {
	d, s := testDraft(t)
	ids, e := d.SnapshotIDs()
	if e != nil || !reflect.DeepEqual(ids, []int64{s[0].ID}) {
		t.Fatal("同一快照多个字段没有去重")
	}
	b, e := BindDraft(d, s)
	if e != nil {
		t.Fatal(e)
	}
	stored := b.Stored()
	if stored.Identity != b.Identity() || stored.Digest != b.Digest() || len(stored.Evidence) != 1 {
		t.Fatal("存储记录丢失身份")
	}
	stored.Canonical[0] = 'x'
	stored.Evidence[0].Hash = "篡改"
	if b.Stored().Canonical[0] != '{' || b.Stored().Evidence[0].Hash != s[0].Hash {
		t.Fatal("存储副本修改了原绑定")
	}
	var none *BoundDraft
	if !reflect.DeepEqual(none.Stored(), StoredVersion{}) {
		t.Fatal("空绑定生成了存证")
	}
	d.Cards[0].Statement.SnapshotID = 0
	if _, e = d.SnapshotIDs(); e == nil {
		t.Fatal("无效编号未拒绝")
	}
	d.Cards = make([]FactCard, MaxCards+1)
	if _, e = d.SnapshotIDs(); e == nil {
		t.Fatal("过量卡片未拒绝")
	}
	d.Cards = nil
	for i := 1; i <= MaxDraftSnapshots+1; i++ {
		d.Cards = append(d.Cards, FactCard{Statement: SentenceRef{SnapshotID: int64(i)}})
	}
	if _, e = d.SnapshotIDs(); e == nil {
		t.Fatal("快照集合超限未拒绝")
	}
}
