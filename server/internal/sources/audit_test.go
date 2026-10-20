package sources

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSourceAuditValidationAndHistoricalAuthorization(t *testing.T) {
	expired := time.UnixMilli(1700000000123).UTC()
	source := Source{ID: 1, Domain: "example.test", Category: CategoryAnnualReport, LicenseTag: "test", LicenseRef: "test:written", LicenseExpires: &expired, MinInterval: time.Second}
	register := SourceAudit{ActorID: 7, Operation: "register", After: source}
	enable := register
	enable.Operation = "enable"
	before := source
	enable.Before = &before
	enable.After.Enabled = true
	disable := register
	disable.Operation = "disable"
	enabledBefore := source
	enabledBefore.Enabled = true
	disable.Before = &enabledBefore
	imported := enable
	imported.Operation = "import"
	imported.Before = nil
	imported.SnapshotID = 2
	imported.ContentHash = strings.Repeat("a", 64)
	imported.URL = "https://example.test/report"
	fetched := imported
	fetched.Operation = "fetch"
	for _, a := range []SourceAudit{register, enable, disable, imported, fetched} {
		if err := a.Validate(); err != nil {
			t.Fatalf("历史授权过期不应阻断读取: %v", err)
		}
	}
	for name, change := range map[string]func(*SourceAudit){
		"无身份":    func(a *SourceAudit) { a.ActorID = 0 },
		"无来源":    func(a *SourceAudit) { a.After.ID = 0 },
		"不规范域名":  func(a *SourceAudit) { a.After.Domain = "EXAMPLE.TEST" },
		"未知类别":   func(a *SourceAudit) { a.After.Category = "other" },
		"访问太快":   func(a *SourceAudit) { a.After.MinInterval = 0 },
		"访问间隔超限": func(a *SourceAudit) { a.After.MinInterval = 25 * time.Hour },
		"无快照":    func(a *SourceAudit) { a.SnapshotID = 0 },
		"坏哈希长度":  func(a *SourceAudit) { a.ContentHash = "bad" },
		"坏哈希字符":  func(a *SourceAudit) { a.ContentHash = strings.Repeat("z", 64) },
		"跨域链接":   func(a *SourceAudit) { a.URL = "https://other.test/report" },
		"不规范链接":  func(a *SourceAudit) { a.URL += "#fragment" },
		"采集停用源":  func(a *SourceAudit) { a.After.Enabled = false },
		"采集带变更前": func(a *SourceAudit) { a.Before = &before },
		"未知操作":   func(a *SourceAudit) { a.Operation = "delete" },
	} {
		t.Run(name, func(t *testing.T) {
			a := imported
			change(&a)
			if !errors.Is(a.Validate(), ErrInvalidSourceAudit) {
				t.Fatal("无效审计被接受")
			}
		})
	}
	for _, a := range []SourceAudit{
		{ActorID: 7, Operation: "register", After: enable.After},
		{ActorID: 7, Operation: "register", Before: &before, After: source},
		{ActorID: 7, Operation: "enable", After: source},
		{ActorID: 7, Operation: "enable", Before: &before, After: source},
		{ActorID: 7, Operation: "register", After: source, SnapshotID: 2},
	} {
		if a.Validate() == nil {
			t.Fatal("操作状态字段未校验")
		}
	}
	mismatched := before
	mismatched.ID = 2
	enable.Before = &mismatched
	if enable.Validate() == nil {
		t.Fatal("变更前来源错绑未拒绝")
	}
	mismatched.ID = 1
	mismatched.LicenseRef = "test:other"
	if enable.Validate() == nil {
		t.Fatal("启停夹带授权变更未拒绝")
	}
}
