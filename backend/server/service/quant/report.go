package quant

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

type ReportService struct{}

// CreateReport 创建研报记录
// Author [yourname](https://github.com/yourname)
func (reportService *ReportService) CreateReport(ctx context.Context, report *quant.Report) (err error) {
	err = global.GVA_DB.Create(report).Error
	return err
}

// DeleteReport 删除研报记录
// Author [yourname](https://github.com/yourname)
func (reportService *ReportService) DeleteReport(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Report{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.Report{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteReportByIds 批量删除研报记录
// Author [yourname](https://github.com/yourname)
func (reportService *ReportService) DeleteReportByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.Report{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.Report{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateReport 更新研报记录
// Author [yourname](https://github.com/yourname)
func (reportService *ReportService) UpdateReport(ctx context.Context, report quant.Report) (err error) {
	err = global.GVA_DB.Model(&quant.Report{}).Where("id = ?", report.ID).Updates(&report).Error
	return err
}

// GetReport 根据id获取研报记录
// Author [yourname](https://github.com/yourname)
func (reportService *ReportService) GetReport(ctx context.Context, id string) (report quant.Report, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&report).Error
	return
}

// GetReportInfoList 分页获取研报记录
// Author [yourname](https://github.com/yourname)
func (reportService *ReportService) GetReportInfoList(ctx context.Context, info quantReq.ReportSearch) (list []quant.Report, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.Report{})
	var reports []quant.Report

	if info.Title != nil && *info.Title != "" {
		db = db.Where("title LIKE ?", "%"+*info.Title+"%")
	}

	if info.Industry != nil && *info.Industry != "" {
		db = db.Where("industry LIKE ?", "%"+*info.Industry+"%")
	}

	if info.Type != nil {
		db = db.Where("type = ?", *info.Type)
	}

	if info.Level != nil {
		db = db.Where("level = ?", *info.Level)
	}

	if info.Institution != nil && *info.Institution != "" {
		db = db.Where("institution LIKE ?", "%"+*info.Institution+"%")
	}

	if info.Analyst != nil && *info.Analyst != "" {
		db = db.Where("analyst LIKE ?", "%"+*info.Analyst+"%")
	}

	if info.Summary != nil && *info.Summary != "" {
		db = db.Where("summary LIKE ?", "%"+*info.Summary+"%")
	}

	if info.FileExt != nil {
		db = db.Where("file_ext = ?", *info.FileExt)
	}

	if info.Remark != nil && *info.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+*info.Remark+"%")
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	if len(info.PublishDateRange) == 2 {
		db = db.Where("publish_date BETWEEN ? AND ?", info.PublishDateRange[0], info.PublishDateRange[1])
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

    err = db.Order("publish_date desc, id desc").Find(&reports).Error
	return reports, total, err
}
func (reportService *ReportService) GetReportPublic(ctx context.Context) {
	// 此方法为获取数据源定义的数据
	// 请自行实现
}
