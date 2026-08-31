package quant

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type NewsService struct{}

const newsListCacheKeyPrefix = "quant_news_list"

func (newsService *NewsService) clearNewsCache(ctx context.Context) {
	if global.GVA_REDIS != nil {
		global.GVA_REDIS.Incr(ctx, newsListCacheKeyPrefix+":version")
		global.GVA_LOG.Info("clearNewsCache", zap.String("key", newsListCacheKeyPrefix+":version"))
	}
}

func (newsService *NewsService) getNewsListCacheKey(ctx context.Context, info quantReq.NewsSearch) string {
	version, _ := global.GVA_REDIS.Get(ctx, newsListCacheKeyPrefix+":version").Result()
	if version == "" {
		version = "0"
	}
	data, _ := json.Marshal(info)
	hash := md5.Sum(data)
	return fmt.Sprintf("%s:v%s:%s", newsListCacheKeyPrefix, version, hex.EncodeToString(hash[:]))
}

// CreateNews 创建快讯记录
// Author [yourname](https://github.com/yourname)
func (newsService *NewsService) CreateNews(ctx context.Context, news *quant.News) (err error) {
	err = global.GVA_DB.Create(news).Error
	if err == nil {
		newsService.clearNewsCache(ctx)
		
		// 异步同步到 Meilisearch
		if global.GVA_MEILISEARCH != nil {
			go func() {
				if meilisearchService, ok := global.GVA_MEILISEARCH.(*MeilisearchService); ok {
					if syncErr := meilisearchService.SyncNewsToMeilisearch(news); syncErr != nil {
						global.GVA_LOG.Error("Failed to sync news to Meilisearch",
							zap.Uint("id", news.ID),
							zap.Error(syncErr))
					}
				}
			}()
		}
	}
	return err
}

// DeleteNews 删除快讯记录
// Author [yourname](https://github.com/yourname)
func (newsService *NewsService) DeleteNews(ctx context.Context, id string, userID uint) (err error) {
	var newsID uint
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 先获取 ID
		var news quant.News
		if err := tx.Where("id = ?", id).First(&news).Error; err != nil {
			return err
		}
		newsID = news.ID
		
		if err := tx.Model(&quant.News{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.News{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		newsService.clearNewsCache(ctx)
		
		// 异步从 Meilisearch 删除
		if global.GVA_MEILISEARCH != nil && newsID > 0 {
			go func() {
				if meilisearchService, ok := global.GVA_MEILISEARCH.(*MeilisearchService); ok {
					if syncErr := meilisearchService.DeleteNewsFromMeilisearch(newsID); syncErr != nil {
						global.GVA_LOG.Error("Failed to delete news from Meilisearch",
							zap.Uint("id", newsID),
							zap.Error(syncErr))
					}
				}
			}()
		}
	}
	return err
}

// DeleteNewsByIds 批量删除快讯记录
// Author [yourname](https://github.com/yourname)
func (newsService *NewsService) DeleteNewsByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	var newsIDs []uint
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		// 先获取所有 ID
		var newsList []quant.News
		if err := tx.Where("id in ?", ids).Find(&newsList).Error; err != nil {
			return err
		}
		for _, news := range newsList {
			newsIDs = append(newsIDs, news.ID)
		}
		
		if err := tx.Model(&quant.News{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.News{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		newsService.clearNewsCache(ctx)
		
		// 异步从 Meilisearch 批量删除
		if global.GVA_MEILISEARCH != nil && len(newsIDs) > 0 {
			go func() {
				if meilisearchService, ok := global.GVA_MEILISEARCH.(*MeilisearchService); ok {
					for _, newsID := range newsIDs {
						if syncErr := meilisearchService.DeleteNewsFromMeilisearch(newsID); syncErr != nil {
							global.GVA_LOG.Error("Failed to delete news from Meilisearch",
								zap.Uint("id", newsID),
								zap.Error(syncErr))
						}
					}
				}
			}()
		}
	}
	return err
}

// UpdateNews 更新快讯记录
// Author [yourname](https://github.com/yourname)
func (newsService *NewsService) UpdateNews(ctx context.Context, news quant.News) (err error) {
	err = global.GVA_DB.Model(&quant.News{}).Where("id = ?", news.ID).Updates(&news).Error
	if err == nil {
		newsService.clearNewsCache(ctx)
		
		// 异步同步到 Meilisearch
		if global.GVA_MEILISEARCH != nil {
			go func() {
				if meilisearchService, ok := global.GVA_MEILISEARCH.(*MeilisearchService); ok {
					if syncErr := meilisearchService.SyncNewsToMeilisearch(&news); syncErr != nil {
						global.GVA_LOG.Error("Failed to sync updated news to Meilisearch",
							zap.Uint("id", news.ID),
							zap.Error(syncErr))
					}
				}
			}()
		}
	}
	return err
}

// GetNews 根据ID获取快讯记录
// Author [yourname](https://github.com/yourname)
func (newsService *NewsService) GetNews(ctx context.Context, id string) (news quant.News, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&news).Error
	return
}

// GetNewsInfoList 分页获取快讯记录
// Author [yourname](https://github.com/yourname)
func (newsService *NewsService) GetNewsInfoList(ctx context.Context, info quantReq.NewsSearch) (list []quant.News, total int64, err error) {
	// 缓存检查
	if global.GVA_REDIS != nil {
		key := newsService.getNewsListCacheKey(ctx, info)
		val, err := global.GVA_REDIS.Get(ctx, key).Result()
		if err == nil {
			var cacheData struct {
				List  []quant.News `json:"list"`
				Total int64        `json:"total"`
			}
			if json.Unmarshal([]byte(val), &cacheData) == nil {
				global.GVA_LOG.Info("GetNewsInfoList cache hit", zap.String("key", key))
				return cacheData.List, cacheData.Total, nil
			}
		}
	}

	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	// 创建db
	db := global.GVA_DB.Model(&quant.News{})
	var newss []quant.News

	// 全文搜索条件
	if info.Keyword != nil && *info.Keyword != "" {
		keywords := strings.Split(*info.Keyword, " ")
		searchQuery := ""
		for _, word := range keywords {
			if word != "" {
				if searchQuery != "" {
					searchQuery += " "
				}
				searchQuery += "+" + word
			}
		}
		db = db.Where("MATCH(title, content) AGAINST(? IN BOOLEAN MODE)", searchQuery)
	}

	if info.Title != nil && *info.Title != "" {
		db = db.Where("title LIKE ?", "%"+*info.Title+"%")
	}

	if info.Content != nil && *info.Content != "" {
		db = db.Where("content LIKE ?", "%"+*info.Content+"%")
	}

	if info.Author != nil && *info.Author != "" {
		db = db.Where("author LIKE ?", "%"+*info.Author+"%")
	}

	if info.Nature != nil {
		db = db.Where("nature = ?", *info.Nature)
	}

	if info.Level != nil {
		db = db.Where("level = ?", *info.Level)
	}

	if info.Bold != nil {
		db = db.Where("bold = ?", *info.Bold)
	}

	if info.Source != nil {
		db = db.Where("source = ?", *info.Source)
	}

	if len(info.CreatedAtRange) == 2 {
		db = db.Where("created_at BETWEEN ? AND ?", info.CreatedAtRange[0], info.CreatedAtRange[1])
	}

	if info.Status != nil {
		db = db.Where("status = ?", *info.Status)
	}

	if info.LastID != 0 {
		db = db.Where("id > ?", info.LastID)
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	var OrderStr string
	orderMap := make(map[string]bool)
	orderMap["nature"] = true
	orderMap["level"] = true
	orderMap["bold"] = true
	orderMap["view"] = true
	orderMap["share"] = true
	orderMap["ctime"] = true
	orderMap["status"] = true

	if orderMap[info.Sort] {
		OrderStr = info.Sort
		if info.Order == "descending" {
			OrderStr = OrderStr + " desc"
		}
		db = db.Order(OrderStr)
	} else if info.LastID > 0 {
		db = db.Order("id asc")
	} else {
		db = db.Order("id desc")
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Find(&newss).Error

	// 缓存设置
	if err == nil && global.GVA_REDIS != nil {
		cacheData := struct {
			List  []quant.News `json:"list"`
			Total int64        `json:"total"`
		}{
			List:  newss,
			Total: total,
		}
		jsonBytes, _ := json.Marshal(cacheData)
		key := newsService.getNewsListCacheKey(ctx, info)
		global.GVA_REDIS.Set(ctx, key, string(jsonBytes), 5*time.Minute)
		global.GVA_LOG.Info("GetNewsInfoList cache set", zap.String("key", key))
	}

	return newss, total, err
}

func (newsService *NewsService) GetNewsPublic(ctx context.Context) {

}

// CreatePublic 创建快讯（不鉴权）
func (newsService *NewsService) CreatePublic(ctx context.Context, news *quant.News) (err error) {
	db := global.GVA_DB.Create(news)
	if db.Error == nil {
		newsService.clearNewsCache(ctx)
		
		// 异步同步到 Meilisearch
		if global.GVA_MEILISEARCH != nil {
			go func() {
				if meilisearchService, ok := global.GVA_MEILISEARCH.(*MeilisearchService); ok {
					if syncErr := meilisearchService.SyncNewsToMeilisearch(news); syncErr != nil {
						global.GVA_LOG.Error("Failed to sync news to Meilisearch",
							zap.Uint("id", news.ID),
							zap.Error(syncErr))
					}
				}
			}()
		}
	}
	return db.Error
}
