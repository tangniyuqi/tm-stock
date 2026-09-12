package member

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	memberReq "github.com/flipped-aurora/gin-vue-admin/server/model/member/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"gorm.io/gorm"
)

// CreateMemberBiz 后台管理 - 创建用户
func (s *MemberService) CreateMemberBiz(ctx context.Context, m *member.Member) error {
	return global.GVA_DB.WithContext(ctx).Create(m).Error
}

// DeleteMember 后台管理 - 删除用户
func (s *MemberService) DeleteMember(ctx context.Context, id string, userID uint) error {
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&member.Member{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&member.Member{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

// DeleteMemberByIds 后台管理 - 批量删除用户
func (s *MemberService) DeleteMemberByIds(ctx context.Context, ids []string, deletedBy uint) error {
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&member.Member{}).Where("id in ?", ids).Update("deleted_by", deletedBy).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&member.Member{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// UpdateMemberBiz 后台管理 - 更新用户
func (s *MemberService) UpdateMemberBiz(ctx context.Context, m member.Member) error {
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).Where("id = ?", m.ID).Updates(&m).Error
}

// GetMemberByID 后台管理 - 根据ID获取用户
func (s *MemberService) GetMemberByID(ctx context.Context, id string) (member.Member, error) {
	var m member.Member
	err := global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&m).Error
	return m, err
}

// ResetMemberPassword 后台管理 - 重置会员密码
func (s *MemberService) ResetMemberPassword(ctx context.Context, id uint, password string) error {
	return global.GVA_DB.WithContext(ctx).Model(&member.Member{}).Where("id = ?", id).Update("password", utils.BcryptHash(password)).Error
}

// GetMemberInfoList 后台管理 - 分页获取用户列表
func (s *MemberService) GetMemberInfoList(ctx context.Context, info memberReq.MemberSearch) (list []member.Member, total int64, err error) {
	limit, offset := info.LimitOffset()

	db := global.GVA_DB.WithContext(ctx).Model(&member.Member{})

	if info.Name != nil && *info.Name != "" {
		db = db.Where("`name` LIKE ?", "%"+*info.Name+"%")
	}
	if info.Nickname != nil && *info.Nickname != "" {
		db = db.Where("`nickname` LIKE ?", "%"+*info.Nickname+"%")
	}
	if info.Mobile != nil && *info.Mobile != "" {
		db = db.Where("`mobile` LIKE ?", "%"+*info.Mobile+"%")
	}
	if info.Email != nil && *info.Email != "" {
		db = db.Where("`email` LIKE ?", "%"+*info.Email+"%")
	}
	if info.Status != nil {
		db = db.Where("`status` = ?", *info.Status)
	}
	if len(info.CreatedAtRange) == 2 {
		db = db.Where("`created_at` BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if err = db.Count(&total).Error; err != nil {
		return
	}
	if limit > 0 {
		db = db.Limit(limit).Offset(offset)
	}
	err = db.Order("id desc").Find(&list).Error
	return
}
