package quant

import (
	"context"
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

type MeilisearchService struct {
	client meilisearch.ServiceManager
}

// InitializeMeilisearch 初始化 Meilisearch 客户端
func (s *MeilisearchService) InitializeMeilisearch() error {
	config := global.GVA_CONFIG.Meilisearch
	
	// 验证配置
	if config.Host == "" {
		return fmt.Errorf("meilisearch host is empty")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("meilisearch port is invalid: %d", config.Port)
	}

	// 构建 Meilisearch URL
	url := fmt.Sprintf("http://%s:%d", config.Host, config.Port)
	
	// 创建客户端
	s.client = meilisearch.New(url, meilisearch.WithAPIKey(config.APIKey))
	
	// 验证连接
	if _, err := s.client.ListIndexes(nil); err != nil {
		global.GVA_LOG.Error("Failed to connect to Meilisearch", 
			zap.String("url", url),
			zap.Error(err))
		return fmt.Errorf("failed to connect to meilisearch: %w", err)
	}
	
	global.GVA_LOG.Info("Meilisearch client initialized successfully", 
		zap.String("url", url))
	
	return nil
}


// CreateNewsIndex 创建并配置新闻索引
func (s *MeilisearchService) CreateNewsIndex() error {
	if s.client == nil {
		return fmt.Errorf("meilisearch client is not initialized")
	}

	indexName := "news"
	
	// 获取或创建索引
	index := s.client.Index(indexName)
	
	// 配置可搜索字段
	searchableAttributes := []string{"title", "content", "author"}
	if _, err := index.UpdateSearchableAttributes(&searchableAttributes); err != nil {
		global.GVA_LOG.Error("Failed to update searchable attributes", zap.Error(err))
		return fmt.Errorf("failed to update searchable attributes: %w", err)
	}
	
	// 配置可筛选字段 - 注意新版本需要 []interface{}
	filterableAttributesStr := []string{"source", "level", "bold", "nature", "status", "created_at"}
	filterableAttributes := make([]interface{}, len(filterableAttributesStr))
	for i, v := range filterableAttributesStr {
		filterableAttributes[i] = v
	}
	if _, err := index.UpdateFilterableAttributes(&filterableAttributes); err != nil {
		global.GVA_LOG.Error("Failed to update filterable attributes", zap.Error(err))
		return fmt.Errorf("failed to update filterable attributes: %w", err)
	}
	
	// 配置可排序字段
	sortableAttributes := []string{"id", "ctime", "created_at", "level", "bold", "view", "share"}
	if _, err := index.UpdateSortableAttributes(&sortableAttributes); err != nil {
		global.GVA_LOG.Error("Failed to update sortable attributes", zap.Error(err))
		return fmt.Errorf("failed to update sortable attributes: %w", err)
	}
	
	global.GVA_LOG.Info("News index created and configured successfully", 
		zap.String("index", indexName))
	
	return nil
}


// NewsDocument Meilisearch 新闻文档结构
type NewsDocument struct {
	ID        uint   `json:"id"`
	NewsID    int    `json:"news_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	Source    int    `json:"source"`
	Nature    int    `json:"nature"`
	Level     int    `json:"level"`
	Bold      int    `json:"bold"`
	View      int    `json:"view"`
	Share     int    `json:"share"`
	Ctime     int64  `json:"ctime"`      // Unix 时间戳
	Status    int    `json:"status"`
	CreatedAt int64  `json:"created_at"` // Unix 时间戳
}

// convertNewsToDocument 将 News 模型转换为 NewsDocument
func convertNewsToDocument(news *quant.News) NewsDocument {
	doc := NewsDocument{
		ID:        news.ID,
		NewsID:    0,
		Title:     "",
		Content:   "",
		Author:    "",
		Source:    0,
		Nature:    0,
		Level:     0,
		Bold:      0,
		View:      0,
		Share:     0,
		Ctime:     0,
		Status:    1,
		CreatedAt: news.CreatedAt.Unix(),
	}
	
	if news.NewsId != nil {
		doc.NewsID = *news.NewsId
	}
	if news.Title != nil {
		doc.Title = *news.Title
	}
	if news.Content != nil {
		doc.Content = *news.Content
	}
	if news.Author != nil {
		doc.Author = *news.Author
	}
	if news.Source != nil {
		doc.Source = *news.Source
	}
	if news.Nature != nil {
		doc.Nature = *news.Nature
	}
	if news.Level != nil {
		doc.Level = *news.Level
	}
	if news.Bold != nil {
		doc.Bold = *news.Bold
	}
	if news.View != nil {
		doc.View = *news.View
	}
	if news.Share != nil {
		doc.Share = *news.Share
	}
	if news.Ctime != nil {
		doc.Ctime = news.Ctime.Unix()
	}
	if news.Status != nil {
		doc.Status = *news.Status
	}
	
	return doc
}

// SyncNewsToMeilisearch 同步单条新闻到 Meilisearch
func (s *MeilisearchService) SyncNewsToMeilisearch(news *quant.News) error {
	if s.client == nil {
		return fmt.Errorf("meilisearch client is not initialized")
	}
	
	if news.ID == 0 {
		return fmt.Errorf("news ID is invalid")
	}
	
	// 转换为 Meilisearch 文档格式
	document := convertNewsToDocument(news)
	
	// 添加或更新文档到 Meilisearch
	index := s.client.Index("news")
	primaryKey := "id"
	taskInfo, err := index.AddDocuments([]NewsDocument{document}, &meilisearch.DocumentOptions{
		PrimaryKey: &primaryKey,
	})
	if err != nil {
		global.GVA_LOG.Error("Failed to sync news to Meilisearch",
			zap.Uint("id", news.ID),
			zap.Error(err))
		return fmt.Errorf("failed to sync news to meilisearch: %w", err)
	}
	
	// 等待任务完成
	if err := s.waitForTask(taskInfo.TaskUID); err != nil {
		global.GVA_LOG.Error("Failed to wait for sync task",
			zap.Uint("id", news.ID),
			zap.Int64("taskUID", taskInfo.TaskUID),
			zap.Error(err))
		return fmt.Errorf("failed to wait for sync task: %w", err)
	}
	
	global.GVA_LOG.Info("News synced to Meilisearch successfully",
		zap.Uint("id", news.ID),
		zap.Int64("taskUID", taskInfo.TaskUID))
	
	return nil
}


// DeleteNewsFromMeilisearch 从 Meilisearch 删除新闻
func (s *MeilisearchService) DeleteNewsFromMeilisearch(id uint) error {
	if s.client == nil {
		return fmt.Errorf("meilisearch client is not initialized")
	}
	
	if id == 0 {
		return fmt.Errorf("news ID is invalid")
	}
	
	// 从 Meilisearch 索引中删除文档
	index := s.client.Index("news")
	taskInfo, err := index.DeleteDocument(fmt.Sprintf("%d", id), nil)
	if err != nil {
		global.GVA_LOG.Error("Failed to delete news from Meilisearch",
			zap.Uint("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to delete news from meilisearch: %w", err)
	}
	
	// 等待任务完成
	if err := s.waitForTask(taskInfo.TaskUID); err != nil {
		global.GVA_LOG.Error("Failed to wait for delete task",
			zap.Uint("id", id),
			zap.Int64("taskUID", taskInfo.TaskUID),
			zap.Error(err))
		return fmt.Errorf("failed to wait for delete task: %w", err)
	}
	
	global.GVA_LOG.Info("News deleted from Meilisearch successfully",
		zap.Uint("id", id),
		zap.Int64("taskUID", taskInfo.TaskUID))
	
	return nil
}


// waitForTask 等待 Meilisearch 任务完成
func (s *MeilisearchService) waitForTask(taskUID int64) error {
	if s.client == nil {
		return fmt.Errorf("meilisearch client is not initialized")
	}
	
	maxRetries := 30 // 最多等待 30 次
	retryInterval := 200 * time.Millisecond // 每次等待 200ms
	
	for i := 0; i < maxRetries; i++ {
		task, err := s.client.GetTask(taskUID)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}
		
		switch task.Status {
		case "succeeded":
			global.GVA_LOG.Debug("Task completed successfully",
				zap.Int64("taskUID", taskUID),
				zap.String("status", string(task.Status)))
			return nil
			
		case "failed":
			global.GVA_LOG.Error("Task failed",
				zap.Int64("taskUID", taskUID),
				zap.String("status", string(task.Status)),
				zap.Any("error", task.Error))
			return fmt.Errorf("task failed: %v", task.Error)
			
		case "enqueued", "processing":
			// 任务还在处理中，继续等待
			global.GVA_LOG.Debug("Task still processing",
				zap.Int64("taskUID", taskUID),
				zap.String("status", string(task.Status)),
				zap.Int("retry", i+1))
			time.Sleep(retryInterval)
			
		default:
			global.GVA_LOG.Warn("Unknown task status",
				zap.Int64("taskUID", taskUID),
				zap.String("status", string(task.Status)))
			time.Sleep(retryInterval)
		}
	}
	
	return fmt.Errorf("task timeout after %d retries", maxRetries)
}


// SyncResult 批量同步结果
type SyncResult struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failure int `json:"failure"`
}

// BatchSyncNewsToMeilisearch 批量同步新闻到 Meilisearch
func (s *MeilisearchService) BatchSyncNewsToMeilisearch(newsList []quant.News) SyncResult {
	result := SyncResult{
		Total:   len(newsList),
		Success: 0,
		Failure: 0,
	}
	
	if s.client == nil {
		global.GVA_LOG.Error("Meilisearch client is not initialized")
		result.Failure = result.Total
		return result
	}
	
	if len(newsList) == 0 {
		return result
	}
	
	batchSize := 1000
	index := s.client.Index("news")
	
	// 分批处理
	for i := 0; i < len(newsList); i += batchSize {
		endIndex := i + batchSize
		if endIndex > len(newsList) {
			endIndex = len(newsList)
		}
		
		batch := newsList[i:endIndex]
		
		// 转换为文档数组
		documents := make([]NewsDocument, 0, len(batch))
		for _, news := range batch {
			doc := convertNewsToDocument(&news)
			documents = append(documents, doc)
		}
		
		// 批量添加到 Meilisearch
		primaryKey := "id"
		taskInfo, err := index.AddDocuments(documents, &meilisearch.DocumentOptions{
			PrimaryKey: &primaryKey,
		})
		if err != nil {
			result.Failure += len(documents)
			global.GVA_LOG.Error("Batch sync failed",
				zap.Int("count", len(documents)),
				zap.Error(err))
			continue
		}
		
		// 等待任务完成
		if err := s.waitForTask(taskInfo.TaskUID); err != nil {
			result.Failure += len(documents)
			global.GVA_LOG.Error("Batch sync task failed",
				zap.Int("count", len(documents)),
				zap.Int64("taskUID", taskInfo.TaskUID),
				zap.Error(err))
		} else {
			result.Success += len(documents)
			global.GVA_LOG.Info("Batch synced successfully",
				zap.Int("count", len(documents)),
				zap.Int64("taskUID", taskInfo.TaskUID))
		}
	}
	
	return result
}


// buildMeilisearchQuery 构建 Meilisearch 查询
func (s *MeilisearchService) buildMeilisearchQuery(req quantReq.NewsSearch) *meilisearch.SearchRequest {
	searchReq := &meilisearch.SearchRequest{}
	
	// 设置搜索关键词
	if req.Keyword != nil && *req.Keyword != "" {
		searchReq.Query = *req.Keyword
	}
	
	// 构建筛选条件
	filters := make([]string, 0)
	
	if req.Level != nil {
		filters = append(filters, fmt.Sprintf("level = %d", *req.Level))
	}
	
	if req.Bold != nil {
		filters = append(filters, fmt.Sprintf("bold = %d", *req.Bold))
	}
	
	if req.Source != nil {
		filters = append(filters, fmt.Sprintf("source = %d", *req.Source))
	}
	
	if req.Nature != nil {
		filters = append(filters, fmt.Sprintf("nature = %d", *req.Nature))
	}
	
	if req.Status != nil {
		filters = append(filters, fmt.Sprintf("status = %d", *req.Status))
	}
	
	// 日期范围筛选
	if len(req.CreatedAtRange) == 2 {
		startTimestamp := req.CreatedAtRange[0].Unix()
		endTimestamp := req.CreatedAtRange[1].Unix()
		filters = append(filters, fmt.Sprintf("created_at >= %d", startTimestamp))
		filters = append(filters, fmt.Sprintf("created_at <= %d", endTimestamp))
	}
	
	// 组合筛选条件（AND 逻辑）
	if len(filters) > 0 {
		filterStr := ""
		for i, filter := range filters {
			if i > 0 {
				filterStr += " AND "
			}
			filterStr += filter
		}
		searchReq.Filter = filterStr
	}
	
	// 设置排序
	sortFields := make([]string, 0)
	if req.Sort != "" {
		sortDirection := "asc"
		if req.Order == "descending" {
			sortDirection = "desc"
		}
		sortFields = append(sortFields, fmt.Sprintf("%s:%s", req.Sort, sortDirection))
	} else {
		sortFields = append(sortFields, "id:desc") // 默认按 ID 降序
	}
	searchReq.Sort = sortFields
	
	// 设置分页
	limit := int64(req.PageSize)
	offset := int64((req.Page - 1) * req.PageSize)
	searchReq.Limit = limit
	searchReq.Offset = offset
	
	return searchReq
}


// SearchResponse 搜索响应结构
type SearchResponse struct {
	List     []NewsDocument `json:"list"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Engine   string         `json:"engine"` // "meilisearch" 或 "mysql"
}

// searchWithMeilisearch 使用 Meilisearch 执行搜索
func (s *MeilisearchService) searchWithMeilisearch(req quantReq.NewsSearch) (*SearchResponse, error) {
	if s.client == nil {
		return nil, fmt.Errorf("meilisearch client is not initialized")
	}
	
	// 构建查询
	searchReq := s.buildMeilisearchQuery(req)
	
	// 执行搜索
	index := s.client.Index("news")
	searchResult, err := index.Search(searchReq.Query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search failed: %w", err)
	}
	
	// 转换结果 - 使用 Decode 方法
	newsList := make([]NewsDocument, 0)
	if err := searchResult.Hits.Decode(&newsList); err != nil {
		global.GVA_LOG.Error("Failed to decode search results", zap.Error(err))
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}
	
	return &SearchResponse{
		List:     newsList,
		Total:    searchResult.EstimatedTotalHits,
		Page:     req.Page,
		PageSize: req.PageSize,
		Engine:   "meilisearch",
	}, nil
}


// searchWithMySQL 使用 MySQL 执行搜索（降级方案）
func (s *MeilisearchService) searchWithMySQL(req quantReq.NewsSearch) (*SearchResponse, error) {
	// 使用现有的 NewsService 进行 MySQL 搜索
	newsService := &NewsService{}
	// 使用 context.Background() 而不是 nil
	ctx := context.Background()
	list, total, err := newsService.GetNewsInfoList(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mysql search failed: %w", err)
	}
	
	// 转换为统一格式
	docList := make([]NewsDocument, 0, len(list))
	for _, news := range list {
		doc := convertNewsToDocument(&news)
		docList = append(docList, doc)
	}
	
	return &SearchResponse{
		List:     docList,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Engine:   "mysql",
	}, nil
}


// SearchNews 主搜索函数（含缓存和降级逻辑）
func (s *MeilisearchService) SearchNews(req quantReq.NewsSearch) (*SearchResponse, error) {
	// 参数验证
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}
	
	// 步骤 1: 检查 Redis 缓存（已禁用以保证快报数据实时性）
	// var cacheKey string
	// if global.GVA_REDIS != nil {
	// 	newsService := &NewsService{}
	// 	cacheKey = newsService.getNewsListCacheKey(ctx, req)
	// 	
	// 	cachedData, err := global.GVA_REDIS.Get(ctx, cacheKey).Result()
	// 	if err == nil && cachedData != "" {
	// 		var response SearchResponse
	// 		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
	// 			global.GVA_LOG.Info("Search cache hit", zap.String("key", cacheKey))
	// 			return &response, nil
	// 		}
	// 	}
	// }
	
	// 步骤 2: 尝试使用 Meilisearch 搜索
	startTime := time.Now()
	result, err := s.searchWithMeilisearch(req)
	
	if err != nil {
		// 步骤 3: 降级到 MySQL 搜索
		global.GVA_LOG.Warn("Meilisearch search failed, falling back to MySQL",
			zap.Error(err))
		
		result, err = s.searchWithMySQL(req)
		if err != nil {
			return nil, fmt.Errorf("both meilisearch and mysql search failed: %w", err)
		}
	}
	
	// 记录性能指标
	elapsedTime := time.Since(startTime)
	global.GVA_LOG.Info("Search completed",
		zap.String("engine", result.Engine),
		zap.Int64("total", result.Total),
		zap.Duration("elapsed", elapsedTime))
	
	if elapsedTime > 500*time.Millisecond {
		global.GVA_LOG.Warn("Search response time exceeded 500ms",
			zap.Duration("elapsed", elapsedTime))
	}
	
	// 步骤 4: 缓存结果（已禁用以保证快报数据实时性）
	// if global.GVA_REDIS != nil && cacheKey != "" {
	// 	jsonBytes, _ := json.Marshal(result)
	// 	global.GVA_REDIS.Set(ctx, cacheKey, string(jsonBytes), 5*time.Minute)
	// 	global.GVA_LOG.Info("Search result cached", zap.String("key", cacheKey))
	// }
	
	return result, nil
}


// ConsistencyCheckResult 一致性检查结果
type ConsistencyCheckResult struct {
	MySQLCount       int64 `json:"mysql_count"`
	MeilisearchCount int64 `json:"meilisearch_count"`
	IsConsistent     bool  `json:"is_consistent"`
	Difference       int64 `json:"difference"`
}

// CheckDataConsistency 检查 MySQL 和 Meilisearch 的数据一致性
func (s *MeilisearchService) CheckDataConsistency() (*ConsistencyCheckResult, error) {
	result := &ConsistencyCheckResult{}
	
	// 获取 MySQL 记录数
	var mysqlCount int64
	if err := global.GVA_DB.Model(&quant.News{}).Count(&mysqlCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count mysql records: %w", err)
	}
	result.MySQLCount = mysqlCount
	
	// 获取 Meilisearch 记录数
	if s.client == nil {
		return nil, fmt.Errorf("meilisearch client is not initialized")
	}
	
	index := s.client.Index("news")
	stats, err := index.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get meilisearch stats: %w", err)
	}
	
	result.MeilisearchCount = stats.NumberOfDocuments
	result.Difference = mysqlCount - stats.NumberOfDocuments
	result.IsConsistent = (result.Difference == 0)
	
	if !result.IsConsistent {
		global.GVA_LOG.Warn("Data inconsistency detected",
			zap.Int64("mysql_count", result.MySQLCount),
			zap.Int64("meilisearch_count", result.MeilisearchCount),
			zap.Int64("difference", result.Difference))
	}
	
	return result, nil
}


// FullSyncToMeilisearch 全量同步 MySQL 数据到 Meilisearch（异步执行）
func (s *MeilisearchService) FullSyncToMeilisearch() (*SyncResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("meilisearch client is not initialized")
	}
	
	global.GVA_LOG.Info("Starting full sync to Meilisearch (async)")
	
	// 先获取总数
	var totalCount int64
	if err := global.GVA_DB.Model(&quant.News{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count records: %w", err)
	}
	
	result := &SyncResult{
		Total:   int(totalCount),
		Success: 0,
		Failure: 0,
	}
	
	// 启动后台 goroutine 执行同步
	go func() {
		batchSize := 1000
		offset := 0
		successCount := 0
		failureCount := 0
		
		global.GVA_LOG.Info("Background sync started", zap.Int64("total", totalCount))
		
		// 分批处理，边读边同步
		for {
			var batch []quant.News
			err := global.GVA_DB.
				Limit(batchSize).
				Offset(offset).
				Find(&batch).Error
			
			if err != nil {
				global.GVA_LOG.Error("Failed to read batch from MySQL",
					zap.Int("offset", offset),
					zap.Error(err))
				failureCount += batchSize
				offset += batchSize
				continue
			}
			
			if len(batch) == 0 {
				break
			}
			
			// 立即同步这一批数据
			batchResult := s.BatchSyncNewsToMeilisearch(batch)
			successCount += batchResult.Success
			failureCount += batchResult.Failure
			
			offset += batchSize
			
			// 记录进度
			progress := float64(offset) / float64(totalCount) * 100
			global.GVA_LOG.Info("Sync progress",
				zap.Int("processed", offset),
				zap.Int64("total", totalCount),
				zap.Float64("progress", progress),
				zap.Int("success", successCount),
				zap.Int("failure", failureCount))
		}
		
		global.GVA_LOG.Info("Background sync completed",
			zap.Int64("total", totalCount),
			zap.Int("success", successCount),
			zap.Int("failure", failureCount))
	}()
	
	// 立即返回，告知用户同步已启动
	global.GVA_LOG.Info("Full sync task started in background", zap.Int("total", result.Total))
	
	return result, nil
}
