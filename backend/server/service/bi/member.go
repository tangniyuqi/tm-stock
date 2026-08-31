package bi

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type MemberService struct{}

// CreateMember 创建用户记录
// Author [yourname](https://github.com/yourname)
func (memberService *MemberService) CreateMember(ctx context.Context, member *bi.Member) (err error) {
	err = global.GVA_DB.Create(member).Error
	return err
}

// DeleteMember 删除用户记录
// Author [yourname](https://github.com/yourname)
func (memberService *MemberService) DeleteMember(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Member{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Member{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteMemberByIds 批量删除用户记录
// Author [yourname](https://github.com/yourname)
func (memberService *MemberService) DeleteMemberByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Member{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Member{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateMember 更新用户记录
// Author [yourname](https://github.com/yourname)
func (memberService *MemberService) UpdateMember(ctx context.Context, member bi.Member) (err error) {
	err = global.GVA_DB.Model(&bi.Member{}).Where("id = ?", member.ID).Updates(&member).Error
	return err
}

// GetMember 根据ID获取用户记录
// Author [yourname](https://github.com/yourname)
func (memberService *MemberService) GetMember(ctx context.Context, id string) (member bi.Member, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&member).Error
	return
}

// GetMemberInfoList 分页获取用户记录
// Author [yourname](https://github.com/yourname)
func (memberService *MemberService) GetMemberInfoList(ctx context.Context, info biReq.MemberSearch) (list []bi.Member, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&bi.Member{})
	var members []bi.Member

	if info.ClientId != nil {
		db = db.Where("client_id = ?", *info.ClientId)
	}

	if info.MsgId != nil {
		db = db.Where("msg_id = ?", *info.MsgId)
	}

	if info.Name != nil && *info.Name != "" {
		db = db.Where("`name` LIKE ?", "%"+*info.Name+"%")
	}

	if info.Nickname != nil && *info.Nickname != "" {
		db = db.Where("`nickname` LIKE ?", "%"+*info.Nickname+"%")
	}

	if info.Mobile != nil && *info.Mobile != "" {
		db = db.Where("`mobile` LIKE ?", "%"+*info.Mobile+"%")
	}

	if info.Wechat != nil && *info.Wechat != "" {
		db = db.Where("`wechat` LIKE ?", "%"+*info.Wechat+"%")
	}

	if info.Business != nil && *info.Business != "" {
		db = db.Where("`business` LIKE ?", "%"+*info.Business+"%")
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("`remark` LIKE ?", "%"+*info.Remark+"%")
	}

	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Order("id desc").Find(&members).Error
	return members, total, err
}

func (memberService *MemberService) GetMemberPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}

func (memberService *MemberService) CheckMobileExists(ctx context.Context, mobile string, excludeID uint) (bool, error) {
	var exists bool
	query := global.GVA_DB.Model(&bi.Member{}).Select("1").Where("mobile = ?", mobile)

	// 仅在 excludeID 不为 0 时添加排除条件
	if excludeID != 0 {
		query = query.Where("id != ?", excludeID)
	}

	err := query.Limit(1).Pluck("1", &exists).Error
	return exists, err
}

func (memberService *MemberService) CheckWechatExists(ctx context.Context, wechat string, excludeID uint) (bool, error) {
	var exists bool
	query := global.GVA_DB.Model(&bi.Member{}).Select("1").Where("wechat = ?", wechat)

	// 仅在 excludeID 不为 0 时添加排除条件
	if excludeID != 0 {
		query = query.Where("id != ?", excludeID)
	}

	err := query.Limit(1).Pluck("1", &exists).Error
	return exists, err
}
