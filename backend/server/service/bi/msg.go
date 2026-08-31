package bi

import (
	"context"
	"fmt"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/bi"
	biReq "github.com/flipped-aurora/gin-vue-admin/server/model/bi/request"
	"gorm.io/gorm"
)

type MsgService struct{}

// CreateMsg 创建消息记录
// Author [yourname](https://github.com/yourname)
func (msgService *MsgService) CreateMsg(ctx context.Context, msg *bi.Msg) (err error) {
	err = global.GVA_DB.Create(msg).Error
	return err
}

// DeleteMsg 删除消息记录
// Author [yourname](https://github.com/yourname)
func (msgService *MsgService) DeleteMsg(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Msg{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&bi.Msg{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteMsgByIds 批量删除消息记录
// Author [yourname](https://github.com/yourname)
func (msgService *MsgService) DeleteMsgByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&bi.Msg{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&bi.Msg{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateMsg 更新消息记录
// Author [yourname](https://github.com/yourname)
func (msgService *MsgService) UpdateMsg(ctx context.Context, msg bi.Msg) (err error) {
	err = global.GVA_DB.Model(&bi.Msg{}).Where("id = ?", msg.ID).Updates(&msg).Error
	return err
}

// GetMsg 根据id获取消息记录
// Author [yourname](https://github.com/yourname)
func (msgService *MsgService) GetMsg(ctx context.Context, id string) (msg bi.Msg, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&msg).Error
	return
}

// GetMsgInfoList 分页获取记录
func (msgService *MsgService) GetMsgInfoList(ctx context.Context, info biReq.MsgSearch) (list []bi.Msg, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&bi.Msg{})
	var msgs []bi.Msg

	if info.ID != nil {
		db = db.Where("id = ?", *info.ID)
	}

	if info.ClientId != nil {
		db = db.Where("client_id = ?", *info.ClientId)
	}

	if info.MsgId != nil {
		db = db.Where("msg_id = ?", *info.MsgId)
	}

	if info.Group != nil && *info.Group != "" {
		db = db.Where("`group` LIKE ?", "%"+*info.Group+"%")
	}

	if info.Sender != nil && *info.Sender != "" {
		sender := *info.Sender
		if len(sender) > 0 && sender[0] == '!' {
			db = db.Where("sender <> ?", sender[1:])
		} else {
			db = db.Where("sender LIKE ?", "%"+sender+"%")
		}
	}

	if info.SenderRemark != nil && *info.SenderRemark != "" {
		db = db.Where("sender_remark LIKE ?", "%"+*info.SenderRemark+"%")
	}

	if info.Content != nil && *info.Content != "" {
		db = db.Where("content LIKE ?", "%"+*info.Content+"%")
	}

	if info.Priority != nil {
		db = db.Where("priority = ?", *info.Priority)
	}

	if info.Oppty != nil {
		db = db.Where("oppty = ?", *info.Oppty)
	}

	if info.Level != nil {
		db = db.Where("level = ?", *info.Level)
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+*info.Remark+"%")
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	if info.StatusStr != nil && *info.StatusStr != "" {
		statusStr := *info.StatusStr
		if len(statusStr) > 0 && statusStr[0] == '!' {
			db = db.Where("status <> ?", statusStr[1:])
		} else {
			db = db.Where("status = ?", statusStr[1:])
		}
	}

	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Order("id desc").Find(&msgs).Error
	return msgs, total, err
}

func (msgService *MsgService) GetListPublic(ctx context.Context, info biReq.MsgSearch) (list []bi.Msg, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&bi.Msg{})

	if info.ID != nil {
		db = db.Where("id > ?", *info.ID)
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

	err = db.Find(&list).Error
	return list, total, err
}

// CreatePublic 创建交易记录（不鉴权）并进行黑名单过滤
func (msgService *MsgService) CreatePublic(ctx context.Context, msg *bi.Msg) (err error) {
	if msg.ClientId == nil {
		return global.GVA_DB.Create(msg).Error
	}

	var client bi.Client
	if err := global.GVA_DB.Where("id = ?", msg.ClientId).First(&client).Error; err != nil {
		return global.GVA_DB.Create(msg).Error
	}

	if *client.Status == 0 {
		return fmt.Errorf("客户端被禁用，不允许创建消息")
	}

	group := ""
	if msg.Group != nil {
		group = strings.TrimSpace(*msg.Group)
	}

	if client.ListType != nil {
		switch *client.ListType {
		case 1:
			if client.AllowList != nil && *client.AllowList != "" {
				allowList := strings.Split(*client.AllowList, "\n")
				allowSet := make(map[string]struct{})
				for _, item := range allowList {
					item = strings.TrimSpace(item)
					if item != "" {
						allowSet[item] = struct{}{}
					}
				}
				if _, inAllow := allowSet[group]; !inAllow {
					return fmt.Errorf("【%s】不在白名单中，不允许创建", group)
				}
			} else {
				return fmt.Errorf("白名单为空，不允许创建")
			}
		case 2:
			if client.BlockList != nil && *client.BlockList != "" {
				blockList := strings.Split(*client.BlockList, "\n")
				blockSet := make(map[string]struct{})
				for _, item := range blockList {
					item = strings.TrimSpace(item)
					if item != "" {
						blockSet[item] = struct{}{}
					}
				}
				if _, inBlock := blockSet[group]; inBlock {
					return fmt.Errorf("【%s】在黑名单中，不允许创建", group)
				}
			}
		default:
		}
	}

	return global.GVA_DB.Create(msg).Error
}

// UpdatePublic 更新消息记录（不鉴权）
func (msgService *MsgService) UpdatePublic(ctx context.Context, msg bi.Msg) (err error) {
	err = global.GVA_DB.Model(&bi.Msg{}).Where("id = ?", msg.ID).Updates(&msg).Error
	return err
}

// GetPageInfoPublic 获取消息总数量、分页大小和总页数（不鉴权）
func (msgService *MsgService) GetPageInfoPublic(ctx context.Context, info biReq.MsgSearch) (total int64, pageSize int, pageCount int64, err error) {
	db := global.GVA_DB.Model(&bi.Msg{})

	if info.ID != nil {
		db = db.Where("id > ?", *info.ID)
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	// 获取总条数
	if err = db.Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}

	// 分页大小
	pageSize = info.PageSize

	// 计算总页数（向上取整）
	if pageSize <= 0 {
		return total, pageSize, 0, nil // 避免除以0
	}
	pageCount = (total + int64(pageSize) - 1) / int64(pageSize)

	return total, pageSize, pageCount, nil
}

// DeleteDuplicate 删除重复的消息记录
func (msgService *MsgService) DeleteDuplicate(ctx context.Context, force bool) (deletedCount int64, err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		subQuery := tx.Table("(?) AS t",
			tx.Model(&bi.Msg{}).
				Select("MIN(id) AS min_id").
				Group("sender, content"),
		).Select("min_id")

		var result *gorm.DB

		if force {
			result = tx.Unscoped().Where("id NOT IN (?)", subQuery).Delete(&bi.Msg{})
		} else {
			result = tx.Where("id NOT IN (?)", subQuery).Delete(&bi.Msg{})
		}

		if result.Error != nil {
			return result.Error
		}

		deletedCount = result.RowsAffected
		return nil
	})

	return deletedCount, err
}

// Match 根据IC匹配
func (msgService *MsgService) Match(ctx context.Context, info biReq.MsgSearch) (total int64, err error) {
	if info.Q == nil || *info.Q == "" || info.MatchType == "" {
		return 0, err
	}

	db := global.GVA_DB.Model(&bi.Msg{})

	if info.Sender != nil && *info.Sender != "" {
		db = db.Where("sender <> ?", "%"+*info.Sender+"%")
	}

	if info.Q != nil && *info.Q != "" && info.MatchType == "content" {
		db = db.Where("content LIKE ?", "%"+*info.Q+"%")
	}

	if info.Q != nil && *info.Q != "" && info.MatchType == "ics" {
		db = db.Where("JSON_CONTAINS(ics, JSON_OBJECT('name', ?), '$')", *info.Q)
		// db = db.Where("JSON_SEARCH(ics, 'one', ?, null, '$[*].name') IS NOT NULL", *info.Q)
	}

	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	if info.StatusStr != nil && *info.StatusStr != "" {
		statusStr := *info.StatusStr
		if len(statusStr) > 0 && statusStr[0] == '!' {
			db = db.Where("status <> ?", statusStr[1:])
		} else {
			db = db.Where("status = ?", statusStr[1:])
		}
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	return total, err
}
