package task

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var (
	ErrCreationDisabled     = fmt.Errorf("会员任务创建已关闭")
	ErrAdmissionUnavailable = fmt.Errorf("任务创建准入不可用")
)

const (
	LightAnalysisType = 1
	DeepAnalysisType  = 2
	DeepPricePoints   = 30
	DeepMinQuotes     = 6
	DeepMaxPerUser    = 1
	DeepMaxGlobal     = 5
)

type AdmissionScope struct {
	UserID       int64
	AnalysisType int
	SubjectType  string
	SubjectID    int64
}

func (s AdmissionScope) Validate() error {
	if s.UserID <= 0 || (s.AnalysisType != 1 && s.AnalysisType != 2) || (s.SubjectType != Theme && s.SubjectType != Topic) || s.SubjectID <= 0 {
		return points.ErrInvalid
	}
	return nil
}

// AdmissionMaterial 仅为服务端当前批准的范围元数据，不接用户URL或正文。
type AdmissionMaterial struct {
	SnapshotID                    int64
	Digest, Domain, Category, URL string
	CollectedAt                   time.Time
}

// AdmissionPlan 的格式正确不代表批准真实；必须由控制器查证，不存在默认放行。
type AdmissionPlan struct {
	Scope                                                            AdmissionScope
	Price                                                            points.Price
	Budget                                                           Budget
	SubjectName, ModelName, FilingNo, ProfileDigest, ExecutionDigest string
	ApprovalRef, QualityRef, HardBudgetRef, PriceRef, SourceRef      string
	Materials                                                        []AdmissionMaterial
	CompanyDigest                                                    string `json:",omitempty"` // 非空时显式绑定完整审核公司集合，空值保留旧编码。
}

func (AdmissionPlan) String() string {
	return "任务准入计划（身份、出处与批准引用已隐藏）"
}
func (p AdmissionPlan) GoString() string { return p.String() }

func (p AdmissionPlan) Validate(scope AdmissionScope) error {
	if p.CompanyDigest != "" && !digestPattern.MatchString(p.CompanyDigest) {
		return points.ErrInvalid
	}
	if scope.Validate() != nil || p.Scope != scope || scope.SubjectType != Theme || p.Price.Validate() != nil || p.Budget.Validate() != nil || p.Budget.MaxCalls < 5 || !digestPattern.MatchString(p.ProfileDigest) || !digestPattern.MatchString(p.ExecutionDigest) {
		return points.ErrInvalid
	}
	for _, ref := range []string{p.ApprovalRef, p.QualityRef, p.HardBudgetRef, p.PriceRef, p.SourceRef} {
		if len(ref) < 1 || len(ref) > 128 || !identifier.MatchString(ref) {
			return points.ErrInvalid
		}
	}
	for i, name := range []string{p.SubjectName, p.ModelName, p.FilingNo} {
		if name == "" || len(name) > 128 || !utf8.ValidString(name) || guard.Normalize(name) != name || guard.Blocked(name) || strings.ContainsAny(name, "\r\n") || (i == 0 && utf8.RuneCountInString(name) > 32) {
			return points.ErrInvalid
		}
	}
	if len(p.Materials) < 1 || len(p.Materials) > 32 {
		return points.ErrInvalid
	}
	var previous int64
	for _, m := range p.Materials {
		if m.SnapshotID <= previous || !digestPattern.MatchString(m.Digest) || m.CollectedAt.UnixMilli() <= 0 || m.CollectedAt.Year() > 9999 || m.CollectedAt.Nanosecond()%int(time.Millisecond) != 0 {
			return points.ErrInvalid
		}
		canonical, err := sources.ValidateURL(m.Domain, m.URL)
		if err != nil || canonical != m.URL {
			return points.ErrInvalid
		}
		switch m.Category {
		case sources.CategoryAnnouncement, sources.CategoryAnnualReport, sources.CategoryProspectus, sources.CategoryOfficialList, sources.CategoryInteractive:
		default:
			return points.ErrInvalid
		}
		previous = m.SnapshotID
	}
	return nil
}

func (p AdmissionPlan) Clone() AdmissionPlan {
	p.Materials = append([]AdmissionMaterial(nil), p.Materials...)
	return p
}
