package task

import "testing"

func TestMemberQueueOptions(t *testing.T) {
	for _, o := range []MemberQueueOptions{{MaxTasks: 1}, {AfterID: 9, MaxTasks: 100}} {
		if err := o.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range []MemberQueueOptions{{}, {AfterID: -1, MaxTasks: 1}, {MaxTasks: 101}} {
		if o.Validate() == nil {
			t.Fatal("无界范围未拒绝")
		}
	}
}
