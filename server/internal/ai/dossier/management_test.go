package dossier

import (
	"encoding/json"
	"fmt"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"reflect"
	"strings"
	"testing"
	"time"
)

func managementTestRequest() ManagementRequest {
	return ManagementRequest{RequestID: "test_management_request_001", Object: ManagementObject{Kind: "publication", ThemeID: 7, Version: 1}, VersionDigest: strings.Repeat("a", 64), Enabled: true, ValidUntil: 1793703600000, ReviewReference: "test-fixture:private-review"}
}
func managementTestEvent() ManagementEvent {
	r := managementTestRequest()
	return ManagementEvent{Protocol: ManagementProtocol, ActorID: 7, Request: r, After: ManagementState{Digest: r.VersionDigest, Reference: ManagedApprovalPrefix + r.RequestID, Enabled: true, ValidUntil: r.ValidUntil}, Revision: 1, CreatedAt: r.ValidUntil - 60000}
}
func TestDossierManagementRequestProtocol(t *testing.T) {
	valid := managementTestRequest()
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseManagementRequest(raw)
	if err != nil || got != valid {
		t.Fatal("合法管理请求被改写")
	}
	for name, mutate := range map[string]func(*ManagementRequest){
		"request_id":        func(r *ManagementRequest) { r.RequestID = "short" },
		"kind":              func(r *ManagementRequest) { r.Object.Kind = "PUBLICATION" },
		"publication_user":  func(r *ManagementRequest) { r.Object.UserID = 1 },
		"member_user":       func(r *ManagementRequest) { r.Object.Kind = "member_read" },
		"theme":             func(r *ManagementRequest) { r.Object.ThemeID = 0 },
		"version":           func(r *ManagementRequest) { r.Object.Version = 9007199254740992 },
		"revision":          func(r *ManagementRequest) { r.ExpectedRevision = -1 },
		"revision_overflow": func(r *ManagementRequest) { r.ExpectedRevision = 9007199254740991 },
		"digest":            func(r *ManagementRequest) { r.VersionDigest = strings.Repeat("A", 64) },
		"until_zero":        func(r *ManagementRequest) { r.ValidUntil = 0 },
		"until_overflow":    func(r *ManagementRequest) { r.ValidUntil = ManagementMaxTime + 1 },
		"disable_until":     func(r *ManagementRequest) { r.Enabled = false },
		"reference":         func(r *ManagementRequest) { r.ReviewReference = " " },
	} {
		t.Run(name, func(t *testing.T) {
			r := valid
			mutate(&r)
			b, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = ParseManagementRequest(b); e == nil {
				t.Fatal("非法管理请求被接受")
			}
		})
	}
	for name, body := range map[string]string{
		"missing_enabled": strings.Replace(string(raw), ",\"enabled\":true", "", 1),
		"null_enabled":    strings.Replace(string(raw), "\"enabled\":true", "\"enabled\":null", 1),
		"string_enabled":  strings.Replace(string(raw), "\"enabled\":true", "\"enabled\":\"true\"", 1),
		"null_user":       strings.Replace(string(raw), "\"userId\":0", "\"userId\":null", 1),
		"string_user":     strings.Replace(string(raw), "\"userId\":0", "\"userId\":\"0\"", 1),
		"float_version":   strings.Replace(string(raw), "\"version\":1", "\"version\":1.5", 1),
		"duplicate":       strings.TrimSuffix(string(raw), "}") + ",\"enabled\":false}",
		"case_duplicate":  strings.TrimSuffix(string(raw), "}") + ",\"Enabled\":false}",
		"actor_spoof":     strings.TrimSuffix(string(raw), "}") + ",\"actorId\":8}",
		"target_extra":    strings.Replace(string(raw), "\"kind\":\"publication\"", "\"kind\":\"publication\",\"scope\":\"all\"", 1),
		"root_array":      "[]",
		"trailing":        string(raw) + "{}",
		"oversized":       strings.Repeat(" ", 8193),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := ParseManagementRequest([]byte(body)); e == nil {
				t.Fatal("畸形JSON被当作完整管理请求")
			}
		})
	}
	t.Run("member_and_disable", func(t *testing.T) {
		r := valid
		r.Object.Kind = "member_read"
		r.Object.UserID = 2
		r.ValidUntil = ManagementMaxTime
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ParseManagementRequest(b); e != nil {
			t.Fatal(e)
		}
		r.Enabled = false
		r.ValidUntil = 0
		if r.Validate() != nil {
			t.Fatal("明确撤回被拒绝")
		}
	})
	t.Run("invalid_utf8", func(t *testing.T) {
		if _, e := ParseManagementRequest(append([]byte{'{', '"'}, byte(255))); e == nil {
			t.Fatal("非法UTF8被接受")
		}
	})
}
func TestDossierManagementEventProtocol(t *testing.T) {
	valid := managementTestEvent()
	raw, hash, err := valid.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreManagementEvent(raw, hash)
	if err != nil || !reflect.DeepEqual(restored, valid) {
		t.Fatal("合法管理事件不能规范回取")
	}
	if !valid.Matches(7, valid.Request) || valid.Matches(8, valid.Request) {
		t.Fatal("原请求身份绑定错误")
	}
	if strings.Contains(fmt.Sprintf("%v %#v %v %#v", valid, valid, valid.Request, valid.Request), valid.Request.ReviewReference) {
		t.Fatal("管理结构日志暴露内部审核引用")
	}
	for name, mutate := range map[string]func(*ManagementEvent){
		"protocol":               func(e *ManagementEvent) { e.Protocol = "other" },
		"actor":                  func(e *ManagementEvent) { e.ActorID = 0 },
		"revision":               func(e *ManagementEvent) { e.Revision++ },
		"time":                   func(e *ManagementEvent) { e.CreatedAt = 0 },
		"reference":              func(e *ManagementEvent) { e.After.Reference = "legacy" },
		"digest":                 func(e *ManagementEvent) { e.After.Digest = strings.Repeat("b", 64) },
		"state":                  func(e *ManagementEvent) { e.After.Enabled = false },
		"expiry":                 func(e *ManagementEvent) { e.CreatedAt = e.After.ValidUntil },
		"missing_before":         func(e *ManagementEvent) { e.Request.ExpectedRevision = 1; e.Revision = 2 },
		"bad_before":             func(e *ManagementEvent) { e.Before = &ManagementState{} },
		"disable_without_before": func(e *ManagementEvent) { e.Request.Enabled = false; e.Request.ValidUntil = 0; e.After.Enabled = false },
	} {
		t.Run(name, func(t *testing.T) {
			e := valid
			mutate(&e)
			if _, _, err := e.Canonical(); err == nil {
				t.Fatal("矛盾管理事件可归档")
			}
		})
	}
	t.Run("disable_preserves_history", func(t *testing.T) {
		e := valid
		before := e.After
		e.Before = &before
		e.Request.RequestID = "test_management_request_002"
		e.Request.ExpectedRevision = 1
		e.Request.Enabled = false
		e.Request.ValidUntil = 0
		e.After.Reference = ManagedApprovalPrefix + e.Request.RequestID
		e.After.Enabled = false
		e.Revision = 2
		e.CreatedAt = e.After.ValidUntil + 1
		raw, digest, err := e.Canonical()
		if err != nil {
			t.Fatal("已过期许可不能留痕撤回")
		}
		if _, err = RestoreManagementEvent(raw, digest); err != nil {
			t.Fatal(err)
		}
		e.After.ValidUntil--
		if _, _, err = e.Canonical(); err == nil {
			t.Fatal("撤回伪造旧期限未拒绝")
		}
	})
	for name, body := range map[string][]byte{
		"unknown":     []byte(strings.TrimSuffix(string(raw), "}") + ",\"extra\":true}"),
		"case_alias":  []byte(strings.Replace(string(raw), "\"actorId\"", "\"ActorId\"", 1)),
		"null_number": []byte(strings.Replace(string(raw), "\"userId\":0", "\"userId\":null", 1)),
		"whitespace":  append([]byte(" "), raw...),
		"duplicate":   []byte(strings.TrimSuffix(string(raw), "}") + ",\"protocol\":\"dossier-management-v1\"}"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := RestoreManagementEvent(body, task.StageDigest(body)); e == nil {
				t.Fatal("非规范管理事件被接受")
			}
		})
	}
	t.Run("approval_projection", func(t *testing.T) {
		a := Approval{Digest: valid.After.Digest, Reference: valid.After.Reference, Enabled: true, Until: time.UnixMilli(valid.After.ValidUntil)}
		if !valid.MatchesApproval(valid.Request.Object, a) || !IsManagedApproval(a.Reference) || IsManagedApproval("legacy") {
			t.Fatal("当前许可后像未绑定")
		}
		a.Enabled = false
		if valid.MatchesApproval(valid.Request.Object, a) {
			t.Fatal("撤回状态被旧事件接受")
		}
	})
	t.Run("digest_mismatch", func(t *testing.T) {
		if _, e := RestoreManagementEvent(raw, strings.Repeat("b", 64)); e == nil {
			t.Fatal("管理事件摘要不符未拒绝")
		}
	})
}
