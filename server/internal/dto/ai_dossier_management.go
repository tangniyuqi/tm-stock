package dto

import "github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"

const AIDossierManagementNotice = "内部候选与管理状态，仅用于有权操作人核对；不代表已发布、AI用途已批准或语义已核验。"

type AIDossierPermissionState struct {
	Enabled    bool  `json:"enabled"`
	ValidUntil int64 `json:"validUntil"`
}
type AIDossierManagementInspect struct {
	Notice        string                    `json:"notice"`
	Object        dossier.ManagementObject  `json:"object"`
	VersionDigest string                    `json:"versionDigest"`
	Revision      int64                     `json:"revision"`
	Managed       bool                      `json:"managed"`
	Permission    *AIDossierPermissionState `json:"permission"`
	SourceState   string                    `json:"sourceState"`
	Preview       *AIDossierView            `json:"preview"`
}
type AIDossierManagementReceipt struct {
	RequestID  string                   `json:"requestId"`
	Object     dossier.ManagementObject `json:"object"`
	ActorID    int64                    `json:"actorId"`
	Revision   int64                    `json:"revision"`
	Enabled    bool                     `json:"enabled"`
	RecordedAt int64                    `json:"recordedAt"`
}

func (AIDossierManagementInspect) String() string {
	return "专题管理预览（候选及许可已隐藏）"
}
func (v AIDossierManagementInspect) GoString() string { return v.String() }
