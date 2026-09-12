package member

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
)

// GetCreditLogs 获取当前会员积分明细流水（分页，数据源 addon_member_asset_log）
// 只取 type=2 的积分资产流水。
func (s *MemberService) GetCreditLogs(ctx context.Context, memberID uint, info request.PageInfo) (list []member.MemberAssetLog, total int64, err error) {
	limit, offset := info.LimitOffset()
	db := global.GVA_DB.WithContext(ctx).Model(&member.MemberAssetLog{}).
		Where("member_id = ? AND type = 2", memberID)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Limit(limit).Offset(offset).Order("id desc").Find(&list).Error
	return list, total, err
}
