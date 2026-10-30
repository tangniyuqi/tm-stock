package dossier

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestConversationDossierSelection(t *testing.T) {
	for _, c := range []struct {
		name  string
		s     Selection
		valid bool
	}{
		{"单版", Selection{7, 1, 0}, true}, {"双版", Selection{7, 1, 2}, true},
		{"最大版本", Selection{7, 1, 9007199254740991}, true},
		{"缺题材", Selection{0, 1, 0}, false}, {"缺版本", Selection{7, 0, 0}, false},
		{"同版", Selection{7, 2, 2}, false}, {"逆序", Selection{7, 2, 1}, false},
		{"负对照", Selection{7, 1, -1}, false}, {"超出安全整数", Selection{7, 1, 9007199254740992}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.s.Valid() != c.valid {
				t.Fatal("版本选择校验不符")
			}
			if c.valid {
				ids := c.s.Versions()
				if len(ids) != 1+int(min(c.s.CompareVersion, 1)) || ids[0] != c.s.Version {
					t.Fatal("版本集合不符")
				}
				ids[0] = 0
				if c.s.Versions()[0] != c.s.Version {
					t.Fatal("返回集合修改了原选择")
				}
			}
		})
	}
}

func TestConversationDossierReferencePrivacy(t *testing.T) {
	r := ConversationReference{ID: 98765, UserID: 12345, ConversationID: 43210, Selection: Selection{7, 1, 2}, CreatedAt: time.UnixMilli(1792281600000)}
	if !r.Valid() {
		t.Fatal("合法引用未接受")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		out := fmt.Sprintf(format, r)
		if strings.Contains(out, "98765") || strings.Contains(out, "12345") || strings.Contains(out, "43210") {
			t.Fatal("格式化泄露私有标识")
		}
	}
	r.CreatedAt = r.CreatedAt.Add(time.Nanosecond)
	if r.Valid() {
		t.Fatal("未对齐时点未拒绝")
	}
}
