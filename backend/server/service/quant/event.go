package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type EventService struct{}

// CreateEvent 创建事件记录
// Author [yourname](https://github.com/yourname)
func (eventService *EventService) CreateEvent(ctx context.Context, event *quant.Event) (err error) {
	err = global.GVA_DB.Create(event).Error
	return err
}

// DeleteEvent 删除事件记录
// Author [yourname](https://github.com/yourname)
func (eventService *EventService) DeleteEvent(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Event{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.Event{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteEventByIds 批量删除事件记录
// Author [yourname](https://github.com/yourname)
func (eventService *EventService) DeleteEventByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Event{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.Event{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateEvent 更新事件记录
// Author [yourname](https://github.com/yourname)
func (eventService *EventService) UpdateEvent(ctx context.Context, event quant.Event) (err error) {
	err = global.GVA_DB.Model(&quant.Event{}).Where("id = ?", event.ID).Updates(&event).Error
	return err
}

// GetEvent 根据id获取事件记录
// Author [yourname](https://github.com/yourname)
func (eventService *EventService) GetEvent(ctx context.Context, id string) (event quant.Event, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&event).Error
	return
}

// GetEventInfoList 分页获取事件记录
// Author [yourname](https://github.com/yourname)
func (eventService *EventService) GetEventInfoList(ctx context.Context, info quantReq.EventSearch) (list []quant.Event, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&quant.Event{})
	var events []quant.Event

	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}

	if info.Industry != nil && *info.Industry != "" {
		db = db.Where("industry LIKE ?", "%"+*info.Industry+"%")
	}

	if info.Level != nil {
		db = db.Where("level = ?", *info.Level)
	}

	if info.City != nil && *info.City != "" {
		db = db.Where("summary city ?", "%"+*info.City+"%")
	}

	if info.Content != nil && *info.Content != "" {
		db = db.Where("content LIKE ?", "%"+*info.Content+"%")
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+*info.Remark+"%")
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	if len(info.DateRange) == 2 {
		db = db.Where("date BETWEEN ? AND ?", info.DateRange[0], info.DateRange[1])
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

	orderStr := "date desc, id desc"
	if info.OrderKey != "" {
		orderOrder := "ASC"
		if info.OrderDesc {
			orderOrder = "DESC"
		}
		orderStr = info.OrderKey + " " + orderOrder
	}

	err = db.Order(orderStr).Find(&events).Error
	return events, total, err
}
func (eventService *EventService) GetEventPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
