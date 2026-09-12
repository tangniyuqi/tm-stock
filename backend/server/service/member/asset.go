package member

import (
	"context"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"gorm.io/gorm"
)

// GetAssetOverview 获取当前会员资产概况（数据源 addon_member_asset）
// 当前仅支持积分资产（type=2）。存量会员可能无资产记录，此时按全 0 返回，不视为错误。
func (s *MemberService) GetAssetOverview(ctx context.Context, memberID uint) (member.AssetOverview, error) {
	var out member.AssetOverview
	var asset member.MemberAsset
	err := global.GVA_DB.WithContext(ctx).Where("member_id = ?", memberID).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			out.AssetType = 2
			return out, nil
		}
		return out, err
	}
	out.AssetType = 2
	out.Credit = asset.Credit
	out.FrozenCredit = asset.FrozenCredit
	out.AccumulateCredit = asset.AccumulateCredit
	out.ConsumeCredit = asset.ConsumeCredit
	out.GiftCredit = asset.GiftCredit
	return out, nil
}
