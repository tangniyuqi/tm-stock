package cms

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"gorm.io/gorm"
)

type ArticleService struct{}

// CreateArticle 创建文章记录
// Author [yourname](https://github.com/yourname)
func (articleService *ArticleService) CreateArticle(ctx context.Context, article *cms.Article) (err error) {
	err = global.GVA_DB.WithContext(ctx).Create(article).Error
	return err
}

// DeleteArticle 删除文章记录
// Author [yourname](https://github.com/yourname)
func (articleService *ArticleService) DeleteArticle(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Article{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&cms.Article{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteArticleByIds 批量删除文章记录
// Author [yourname](https://github.com/yourname)
func (articleService *ArticleService) DeleteArticleByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cms.Article{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&cms.Article{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateArticle 更新文章记录
// Author [yourname](https://github.com/yourname)
func (articleService *ArticleService) UpdateArticle(ctx context.Context, article cms.Article) (err error) {
	err = global.GVA_DB.WithContext(ctx).Model(&cms.Article{}).Where("id = ?", article.ID).Updates(&article).Error
	return err
}

// GetArticle 根据ID获取文章记录
// Author [yourname](https://github.com/yourname)
func (articleService *ArticleService) GetArticle(ctx context.Context, id string) (article cms.Article, err error) {
	err = global.GVA_DB.WithContext(ctx).Where("id = ?", id).First(&article).Error
	return
}

// GetArticleInfoList 分页获取文章记录
// Author [yourname](https://github.com/yourname)
func (articleService *ArticleService) GetArticleInfoList(ctx context.Context, info cmsReq.ArticleSearch) (list []cms.Article, total int64, err error) {
	limit, offset := info.LimitOffset()
	// 创建db
	db := global.GVA_DB.WithContext(ctx).Model(&cms.Article{})
	var articles []cms.Article
	// 如果有条件搜索 下方会自动创建搜索语句
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

	err = db.Find(&articles).Error
	return articles, total, err
}
// GetArticleListPublic 按分类获取已发布文章列表（C 端公开接口，可用于"帮助中心"等）
func (articleService *ArticleService) GetArticleListPublic(ctx context.Context, cateID int32) (list []cms.Article, err error) {
	db := global.GVA_DB.WithContext(ctx).Model(&cms.Article{}).Where("status = ?", true)
	if cateID > 0 {
		db = db.Where("cate_id = ?", cateID)
	}
	err = db.Order("sort ASC, id DESC").Find(&list).Error
	return
}
