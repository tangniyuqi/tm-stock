package task

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const AdmissionVersion = "task-admission-v1"
const MaxAdmissionBytes = 65536

type admissionDocument struct {
	Version string
	Plan    AdmissionPlan
}

func (p AdmissionPlan) Encode() ([]byte, string, error) {
	if p.Validate(p.Scope) != nil {
		return nil, "", points.ErrInvalid
	}
	p = p.Clone()
	for i := range p.Materials {
		p.Materials[i].CollectedAt = p.Materials[i].CollectedAt.UTC()
	}
	b, err := json.Marshal(admissionDocument{AdmissionVersion, p})
	if err != nil || len(b) > MaxAdmissionBytes {
		return nil, "", points.ErrInvalid
	}
	return b, StageDigest(b), nil
}

func DecodeAdmission(b []byte) (AdmissionPlan, error) {
	if len(b) > MaxAdmissionBytes || CheckStageJSON(b) != nil {
		return AdmissionPlan{}, points.ErrInvariant
	}
	var doc admissionDocument
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&doc) != nil || doc.Version != AdmissionVersion {
		return AdmissionPlan{}, points.ErrInvariant
	}
	canonical, _, err := doc.Plan.Encode()
	if err != nil || !bytes.Equal(canonical, b) {
		return AdmissionPlan{}, points.ErrInvariant
	}
	return doc.Plan, nil
}

type AdmissionRecord struct {
	TaskID, UserID              int64
	TaskNo, RequestHash, Digest string
	Plan                        AdmissionPlan
	CreatedAt                   time.Time
}

func (AdmissionRecord) String() string {
	return "持久准入记录（身份、出处与批准已隐藏）"
}
func (r AdmissionRecord) GoString() string { return r.String() }
func (r AdmissionRecord) Bind(v Task) error {
	_, digest, err := r.Plan.Encode()
	if err != nil || r.TaskID != v.ID || v.ID <= 0 || r.UserID != v.UserID || r.TaskNo != v.No || !regexpTaskNo(v.No) || r.RequestHash != v.Hash || !digestPattern.MatchString(v.Hash) || r.Digest != digest || r.CreatedAt.IsZero() || r.CreatedAt.UnixMilli() <= 0 || r.CreatedAt.Nanosecond()%int(time.Millisecond) != 0 {
		return points.ErrInvariant
	}
	p := r.Plan
	if p.Scope != (AdmissionScope{v.UserID, v.AnalysisType, v.SubjectType, v.SubjectID}) || p.Price.Version != v.PriceVersion || p.Price.Points != v.PricePoints || p.Budget != v.Budget || !v.Deadline.Equal(r.CreatedAt.Add(time.Duration(v.Budget.MaxDurationMS)*time.Millisecond)) {
		return points.ErrInvariant
	}
	for _, m := range p.Materials {
		if m.CollectedAt.After(r.CreatedAt) {
			return points.ErrInvariant
		}
	}
	return nil
}
func regexpTaskNo(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

type AdmissionReadTx interface {
	Tx
	ReadAdmission(context.Context, Task) (AdmissionRecord, bool, error)
}
type AdmissionWriteTx interface {
	AdmissionReadTx
	InsertAdmittedTask(context.Context, Task, string) (int64, error)
	InsertAdmission(context.Context, AdmissionRecord) error
}
type AdmissionStore interface {
	WithinAdmissionTx(context.Context, func(AdmissionReadTx) error) error
}
