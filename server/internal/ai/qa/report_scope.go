package qa

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 报告追问只使用本人当前可读报告内已终检的引文；它是独立证据边界，不回退到全资料库。
const (
	ReportMaxQuotes    = 16
	reportScopeTTL     = 2 * time.Minute
	reportLicenseTag   = "报告终检引文"
	reportApprovalBase = "report:"
)

// IsReportApproval 识别报告追问的稳定批准引用；资料问答不得伪造该前缀获得额外证据上限。
func IsReportApproval(ref string) bool { return strings.HasPrefix(ref, reportApprovalBase) }

// ParseReportApproval 只解析受信任格式的 report:<taskID>:<digest>，不接受其他前缀或多余字段。
func ParseReportApproval(ref string) (int64, string, bool) {
	if !IsReportApproval(ref) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(ref, reportApprovalBase)
	rawTask, rawDigest, ok := strings.Cut(rest, ":")
	if !ok || !digest(rawDigest) {
		return 0, "", false
	}
	taskID, err := strconv.ParseInt(rawTask, 10, 64)
	if err != nil || !conversation.ID(taskID) || strconv.FormatInt(taskID, 10) != rawTask {
		return 0, "", false
	}
	return taskID, rawDigest, true
}

// ReportQuote 是报告内一段可公开引文及其出处；不携带快照正文或内部判定。
type ReportQuote struct {
	Section     string
	Text        string
	SourceType  string
	SourceURL   string
	CollectedAt time.Time
}

func (ReportQuote) String() string     { return "报告引文（正文已隐藏）" }
func (q ReportQuote) GoString() string { return q.String() }

// ReportEvidence 由当前可读报告构造；Digest 必须是已提交报告内容的稳定摘要。
type ReportEvidence struct {
	TaskID      int64
	SubjectID   int64
	SubjectType string
	SubjectName string
	GeneratedAt time.Time
	Digest      string
	Quotes      []ReportQuote
}

func (ReportEvidence) String() string     { return "报告追问证据（正文与出处已隐藏）" }
func (e ReportEvidence) GoString() string { return e.String() }

// reportText 允许报告引文内的换行/制表符（页面以 pre-wrap 展示），但拒绝其他控制符与首尾空白。
func reportText(value string) bool {
	if !utf8.ValidString(value) || value == "" || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > 1000 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}

func reportCategory(label string) (string, bool) {
	switch label {
	case "公告":
		return sources.CategoryAnnouncement, true
	case "年报":
		return sources.CategoryAnnualReport, true
	case "招股书":
		return sources.CategoryProspectus, true
	case "官方产业目录":
		return sources.CategoryOfficialList, true
	case "互动易问答":
		return sources.CategoryInteractive, true
	}
	return "", false
}

// BuildReportScope 把报告引文转换为逐条快照，供现有问答核做选句、回取与判定。
// 它不读取资料库、不接受客户端快照编号；每条引文都要求 HTTPS 出处与有效时点。
func BuildReportScope(ev ReportEvidence, now time.Time) (Scope, error) {
	return BuildReportScopeUntil(ev, now, now.Add(reportScopeTTL))
}

// BuildReportScopeUntil 用调用方给定的批准时点重建范围；授权复核必须在原批准时限内复现同一范围。
func BuildReportScopeUntil(ev ReportEvidence, now, validUntil time.Time) (Scope, error) {
	if !validUntil.After(now) || validUntil.After(now.Add(reportScopeTTL)) {
		return Scope{}, ErrInvalid
	}
	if !conversation.ID(ev.TaskID) || !conversation.ID(ev.SubjectID) || ev.SubjectType != "theme" || !meta(ev.SubjectName) || !digest(ev.Digest) || !conversation.Time(ev.GeneratedAt) || ev.GeneratedAt.After(now) {
		return Scope{}, ErrInvalid
	}
	if len(ev.Quotes) < 1 || len(ev.Quotes) > ReportMaxQuotes {
		return Scope{}, ErrInvalid
	}
	seen := map[string]bool{}
	snapshots := make([]sources.Snapshot, 0, len(ev.Quotes))
	domains := []string{}
	for _, quote := range ev.Quotes {
		if !meta(quote.Section) || !reportText(quote.Text) || !conversation.Time(quote.CollectedAt) || quote.CollectedAt.After(now) || quote.CollectedAt.After(ev.GeneratedAt) {
			return Scope{}, ErrInvalid
		}
		category, ok := reportCategory(quote.SourceType)
		if !ok {
			return Scope{}, ErrInvalid
		}
		u, err := url.Parse(quote.SourceURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return Scope{}, ErrInvalid
		}
		canonical, err := sources.ValidateURL(u.Hostname(), quote.SourceURL)
		if err != nil || canonical != quote.SourceURL {
			return Scope{}, ErrInvalid
		}
		key := quote.SourceURL + "\x00" + quote.Text
		// 同一出处同一原文重复引用只保留一条证据；不丢失任何唯一引文。
		if seen[key] {
			continue
		}
		seen[key] = true
		source := sources.Source{ID: int64(len(snapshots) + 1), Domain: u.Hostname(), Category: category, LicenseTag: reportLicenseTag, LicenseRef: reportApprovalBase + fmt.Sprintf("%d:%s", ev.TaskID, ev.Digest), Enabled: true}
		snapshot, err := sources.BuildSnapshot(source, quote.SourceURL, sources.MediaPlain, []byte(quote.Text), quote.CollectedAt, 64<<10)
		if err != nil {
			return Scope{}, ErrInvalid
		}
		snapshot.ID = int64(len(snapshots) + 1)
		snapshot.SourceID = int64(len(snapshots) + 1)
		// 规范化后的正文必须与报告引文逐字一致；任何改写都会让“报告内引文”失去可追溯性。
		if snapshot.Body != quote.Text || snapshot.Validate() != nil {
			return Scope{}, ErrInvalid
		}
		if !slices.Contains(domains, u.Hostname()) {
			domains = append(domains, u.Hostname())
		}
		snapshots = append(snapshots, snapshot)
	}
	if len(domains) < 1 || len(domains) > 16 {
		return Scope{}, ErrInvalid
	}
	slices.Sort(domains)
	return Scope{SubjectType: ev.SubjectType, SubjectID: ev.SubjectID, SubjectName: ev.SubjectName, ApprovalRef: reportApprovalBase + fmt.Sprintf("%d:%s", ev.TaskID, ev.Digest), ValidUntil: validUntil, Snapshots: snapshots}, nil
}

// NewReportPlan 只允许 report_qa 使用报告引文范围；任务编号必须与原报告一致，不能借其他报告充数。
func NewReportPlan(id Identity, in conversation.MessageInput, ev ReportEvidence, c Config, now time.Time) (*Plan, error) {
	if in.TaskID != ev.TaskID {
		return nil, ErrInvalid
	}
	scope, err := BuildReportScope(ev, now)
	if err != nil {
		return nil, err
	}
	return NewReportPlanScoped(id, in, scope, c, now)
}

// NewReportPlanScoped 用已重建的报告范围构造计划，供读取时按原批准时点复现同一计划。
func NewReportPlanScoped(id Identity, in conversation.MessageInput, scope Scope, c Config, now time.Time) (*Plan, error) {
	taskID, _, ok := ParseReportApproval(scope.ApprovalRef)
	if !ok || taskID != in.TaskID || in.Skill != "report_qa" || len(scope.Snapshots) < 1 || len(scope.Snapshots) > ReportMaxQuotes {
		return nil, ErrInvalid
	}
	domains := make([]string, 0, len(scope.Snapshots))
	for _, snapshot := range scope.Snapshots {
		u, err := url.Parse(snapshot.URL)
		if err != nil {
			return nil, ErrInvalid
		}
		if !slices.Contains(domains, u.Hostname()) {
			domains = append(domains, u.Hostname())
		}
	}
	slices.Sort(domains)
	// 报告范围不继承资料库域名白名单；只使用本报告引文实际出处，并整体替换而不是合并。
	c.AllowedDomains = domains
	c.Limits.MaxSources = len(scope.Snapshots)
	return newPlan(id, in, scope, c, now, "report_qa", ReportMaxQuotes)
}

// ReportReference 返回报告追问的稳定批准引用（任务编号+报告摘要），供持久层复核原报告。
func (p *Plan) ReportReference() (string, bool) {
	if !p.Valid() || p.input.Skill != "report_qa" {
		return "", false
	}
	return p.scope.ApprovalRef, true
}
