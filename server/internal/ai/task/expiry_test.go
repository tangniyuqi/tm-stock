package task

import "testing"

func TestExpiryOptions(t *testing.T) {
	for _, o := range []ExpiryOptions{{PageSize: 1, MaxTasks: 1}, {AfterID: 9, PageSize: 1000, MaxTasks: 100000}} {
		if o.Validate() != nil {
			t.Fatal(o)
		}
	}
	for _, o := range []ExpiryOptions{{}, {AfterID: -1, PageSize: 1, MaxTasks: 1}, {PageSize: 1001, MaxTasks: 1}, {PageSize: 1, MaxTasks: 0}, {PageSize: 1, MaxTasks: 100001}} {
		if o.Validate() == nil {
			t.Fatal(o)
		}
	}
}
