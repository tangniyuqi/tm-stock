package service

import (
	"slices"
	"strconv"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

const dossierSourceNotice = "仅核对所选档案已引用资料的存档关系；相同链接可能有不同采集版本，相同正文不证明转载方向，同一登记来源不等于同一原始发布者。未分组不表示来源独立，未核实不表示不存在。"

// 只接收经原单/双版本当前授权、正文绑定和公开引文投影后的出处。
// 内部来源编号及正文摘要只用于相等分组，不进入公开响应。
func dossierSourceRelations(origins map[int64]dossier.Origin) dto.AIDossierSourceRelations {
	out := dto.AIDossierSourceRelations{Notice: dossierSourceNotice, Independence: "unverified", ReprintChain: "unverified", ExplicitCorrection: "unverified", Groups: []dto.AIDossierSourceGroup{}}
	ids := make([]int64, 0, len(origins))
	for id := range origins {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, axis := range []struct {
		kind string
		key  func(dossier.Origin) string
	}{
		{"same_url", func(o dossier.Origin) string { return o.URL }},
		{"duplicate_body", func(o dossier.Origin) string { return o.Hash }},
		{"same_registered_source", func(o dossier.Origin) string { return strconv.FormatInt(o.SourceID, 10) }},
	} {
		buckets := map[string][]int64{}
		for _, id := range ids {
			key := axis.key(origins[id])
			buckets[key] = append(buckets[key], id)
		}
		groups := []dto.AIDossierSourceGroup{}
		for _, members := range buckets {
			if len(members) > 1 {
				groups = append(groups, dto.AIDossierSourceGroup{Kind: axis.kind, SnapshotIDs: members})
			}
		}
		slices.SortFunc(groups, func(a, b dto.AIDossierSourceGroup) int {
			if a.SnapshotIDs[0] < b.SnapshotIDs[0] {
				return -1
			}
			if a.SnapshotIDs[0] > b.SnapshotIDs[0] {
				return 1
			}
			return 0
		})
		out.Groups = append(out.Groups, groups...)
	}
	return out
}
