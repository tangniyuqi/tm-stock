package quant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BaseStockService struct{}

// CreateBaseStock 创建基础股票记录
func (baseStockService *BaseStockService) CreateBaseStock(ctx context.Context, baseStock *quant.BaseStock) (err error) {
	err = global.GVA_DB.Create(baseStock).Error
	return err
}

// DeleteBaseStock 删除基础股票记录
func (baseStockService *BaseStockService) DeleteBaseStock(ctx context.Context, id string, userID uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.BaseStock{}).Where("id = ?", id).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err = tx.Delete(&quant.BaseStock{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// DeleteBaseStockByIds 批量删除基础股票记录
func (baseStockService *BaseStockService) DeleteBaseStockByIds(ctx context.Context, ids []string, deleted_by uint) (err error) {
	err = global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&quant.BaseStock{}).Where("id in ?", ids).Update("deleted_by", deleted_by).Error; err != nil {
			return err
		}
		if err := tx.Where("id in ?", ids).Delete(&quant.BaseStock{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// UpdateBaseStock 更新基础股票记录
func (baseStockService *BaseStockService) UpdateBaseStock(ctx context.Context, baseStock quant.BaseStock) (err error) {
	err = global.GVA_DB.Model(&quant.BaseStock{}).Where("id = ?", baseStock.ID).Updates(&baseStock).Error
	return err
}

// GetBaseStock 根据id获取基础股票记录
func (baseStockService *BaseStockService) GetBaseStock(ctx context.Context, id string) (baseStock quant.BaseStock, err error) {
	err = global.GVA_DB.Where("id = ?", id).First(&baseStock).Error
	return
}

// GetBaseStockInfoList 分页获取基础股票记录
func (baseStockService *BaseStockService) GetBaseStockInfoList(ctx context.Context, info quantReq.BaseStockSearch) (list []quant.BaseStock, total int64, err error) {
	limit := info.PageSize
	offset := info.PageSize * (info.Page - 1)
	db := global.GVA_DB.Model(&quant.BaseStock{})
	var baseStocks []quant.BaseStock

	// 全文搜索条件
	if info.Q != nil && *info.Q != "" {
		q := *info.Q
		q = strings.ReplaceAll(q, "\"", "")
		searchQuery := "+" + q + "*"

		db = db.Select("*, MATCH(symbol, name, cnspell) AGAINST(? IN BOOLEAN MODE) AS score", searchQuery).
			Where("MATCH(symbol, name, cnspell) AGAINST(? IN BOOLEAN MODE)", searchQuery).
			Order("score DESC")
	}

	if info.TsCode != nil && *info.TsCode != "" {
		db = db.Where("ts_code LIKE ?", "%"+*info.TsCode+"%")
	}
	if info.Symbol != nil && *info.Symbol != "" {
		db = db.Where("symbol LIKE ?", "%"+*info.Symbol+"%")
	}
	if info.Name != nil && *info.Name != "" {
		db = db.Where("name LIKE ?", "%"+*info.Name+"%")
	}
	if info.Area != nil && *info.Area != "" {
		db = db.Where("area LIKE ?", "%"+*info.Area+"%")
	}
	if info.Industry != nil && *info.Industry != "" {
		db = db.Where("industry LIKE ?", "%"+*info.Industry+"%")
	}
	if info.Fullname != nil && *info.Fullname != "" {
		db = db.Where("fullname LIKE ?", "%"+*info.Fullname+"%")
	}
	if info.Enname != nil && *info.Enname != "" {
		db = db.Where("enname LIKE ?", "%"+*info.Enname+"%")
	}
	if info.Cnspell != nil && *info.Cnspell != "" {
		db = db.Where("cnspell LIKE ?", "%"+*info.Cnspell+"%")
	}
	if info.Market != nil && *info.Market != "" {
		db = db.Where("market = ?", *info.Market)
	}
	if info.Exchange != nil && *info.Exchange != "" {
		db = db.Where("exchange = ?", *info.Exchange)
	}
	if info.ListStatus != nil && *info.ListStatus != "" {
		db = db.Where("list_status = ?", *info.ListStatus)
	}
	if info.IsHs != nil && *info.IsHs != "" {
		db = db.Where("is_hs = ?", *info.IsHs)
	}
	if info.ActName != nil && *info.ActName != "" {
		db = db.Where("act_name LIKE ?", "%"+*info.ActName+"%")
	}

	err = db.Count(&total).Error
	if err != nil {
		return
	}

	if limit != 0 {
		db = db.Limit(limit).Offset(offset)
	}

	err = db.Find(&baseStocks).Error
	return baseStocks, total, err
}

// GetBaseStockPublic 公开获取基础股票数据，复用GetBaseStockInfoList的逻辑
func (baseStockService *BaseStockService) GetBaseStockPublic(ctx context.Context, info quantReq.BaseStockSearch) (list []quant.BaseStock, total int64, err error) {
	return baseStockService.GetBaseStockInfoList(ctx, info)
}

// TushareResponse 定义了 Tushare API 响应的整体结构
type TushareResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Fields  []string        `json:"fields"`
		Items   [][]interface{} `json:"items"`
		HasMore *bool           `json:"has_more"`
		Count   int             `json:"count"`
	} `json:"data"`
	RequestID string `json:"request_id"`
}

// 工具函数：string → *string
func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// 工具函数：日期字符串 → *time.Time
func parseDateToPtr(dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, nil
	}

	layout := "20060102"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		layout2 := "2006-01-02"
		t, err = time.Parse(layout2, dateStr)
		if err != nil {
			return nil, fmt.Errorf("date parse failed: %s, err: %w", dateStr, err)
		}
	}
	return &t, nil
}

// 从 Tushare API 获取股票基本数据
func (baseStockService *BaseStockService) fetchTushareStockBasic(ctx context.Context, token, apiUrl string, limit, offset int) (*TushareResponse, error) {
	requestPayload := map[string]interface{}{
		"api_name": "stock_basic",
		"token":    token,
		"params": map[string]interface{}{
			"list_status": "L", // L-上市 D-退市 P-暂停上市
			"limit":       limit,
			"offset":      offset,
		},
		"fields": "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,enname,exchange,curr_type,list_status,delist_date,is_hs",
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Tushare 官方接口统一 POST 到 apiUrl 根路径，接口名（api_name）在请求体 JSON 中
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 调试：打印 Tushare API 的原始响应
	global.GVA_LOG.Debug("Tushare API raw response", zap.String("body", string(body)))

	var tushareResp TushareResponse
	if err := json.Unmarshal(body, &tushareResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tushare response: %w", err)
	}

	if tushareResp.Code != 0 {
		return nil, fmt.Errorf("tushare API error: %s", tushareResp.Msg)
	}

	return &tushareResp, nil
}

// parseTushareItems 将 Tushare API 返回的数据项转换为 quant.BaseStock 切片
func (baseStockService *BaseStockService) parseTushareItems(tushareResp *TushareResponse) ([]quant.BaseStock, error) {
	fieldMap := make(map[string]int)
	for i, field := range tushareResp.Data.Fields {
		fieldMap[field] = i
	}

	// 检查必需的字段是否存在
	requiredFields := []string{"ts_code", "symbol", "name", "list_date"}
	for _, f := range requiredFields {
		if _, ok := fieldMap[f]; !ok {
			return nil, fmt.Errorf("missing required field in tushare response: %s", f)
		}
	}

	var stocks []quant.BaseStock
	for _, item := range tushareResp.Data.Items {
		getString := func(fieldName string) string {
			if idx, ok := fieldMap[fieldName]; ok && idx < len(item) {
				if val, ok := item[idx].(string); ok {
					return val
				}
			}
			return ""
		}

		listDateStr := getString("list_date")
		listDatePtr, err := parseDateToPtr(listDateStr)
		if err != nil {
			global.GVA_LOG.Warn("parse list_date failed, skipping item", zap.String("date", listDateStr), zap.Error(err))
			continue
		}

		delistDateStr := getString("delist_date")
		delistDatePtr, err := parseDateToPtr(delistDateStr)
		if err != nil {
			global.GVA_LOG.Warn("parse delist_date failed, skipping item", zap.String("date", delistDateStr), zap.Error(err))
			continue
		}

		stock := quant.BaseStock{
			TsCode:     strToPtr(getString("ts_code")),
			Symbol:     strToPtr(getString("symbol")),
			Name:       strToPtr(getString("name")),
			Area:       strToPtr(getString("area")),
			Industry:   strToPtr(getString("industry")),
			Fullname:   strToPtr(getString("fullname")),
			Enname:     strToPtr(getString("enname")),
			Cnspell:    strToPtr(getString("cnspell")),
			Market:     strToPtr(getString("market")),
			Exchange:   strToPtr(getString("exchange")),
			CurrType:   strToPtr(getString("curr_type")),
			ListStatus: strToPtr(getString("list_status")),
			ListDate:   listDatePtr,
			DelistDate: delistDatePtr,
			IsHs:       strToPtr(getString("is_hs")),
			ActName:    strToPtr(getString("act_name")),
			ActEntType: strToPtr(getString("act_ent_type")),
		}
		stocks = append(stocks, stock)
	}
	return stocks, nil
}

// Sync 同步基础股票数据
func (baseStockService *BaseStockService) Sync(ctx context.Context) (err error) {
	token := global.GVA_CONFIG.Tushare.Token
	apiUrl := global.GVA_CONFIG.Tushare.ApiUrl
	if token == "" || apiUrl == "" {
		return fmt.Errorf("tushare token or api_url is not configured")
	}

	var allStocks []quant.BaseStock
	limit := 10000 // Tushare Pro API 单次最大可获取 10000 条
	offset := 0

	for {
		// 1. 从 Tushare API 分页获取数据
		tushareResp, err := baseStockService.fetchTushareStockBasic(ctx, token, apiUrl, limit, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch data from tushare (offset: %d): %w", offset, err)
		}

		// 2. 解析和转换数据
		stocks, err := baseStockService.parseTushareItems(tushareResp)
		if err != nil {
			return fmt.Errorf("failed to parse tushare items: %w", err)
		}

		if len(stocks) == 0 {
			// 没有更多数据了，退出循环
			break
		}

		allStocks = append(allStocks, stocks...)

		// 如果返回的数据量小于 limit，说明已经是最后一页
		if len(stocks) < limit {
			break
		}

		// 准备下一次请求的 offset
		offset += limit
	}

	if len(allStocks) == 0 {
		global.GVA_LOG.Info("no stock data to sync from tushare")
		return nil
	}

	// 3. 查询本地已有股票，构建 ts_code -> id 映射，用于区分新增与更新
	var localStocks []quant.BaseStock
	if err := global.GVA_DB.WithContext(ctx).Select("id", "ts_code").Find(&localStocks).Error; err != nil {
		return fmt.Errorf("failed to query local stocks: %w", err)
	}
	tsCodeToID := make(map[string]uint, len(localStocks))
	for i := range localStocks {
		if localStocks[i].TsCode != nil && *localStocks[i].TsCode != "" {
			tsCodeToID[*localStocks[i].TsCode] = localStocks[i].ID
		}
	}

	// 需要更新的基础信息字段列。
	// 不含 change_pct 及 AI 分析相关字段（fundamentals/financial/realization/momentum/risk/ai_analyzed_at），
	// 避免增量同步时覆盖本地已有的行情或分析结果。
	updateColumns := []string{
		"ts_code", "symbol", "name", "area", "industry", "fullname", "enname", "cnspell",
		"market", "exchange", "curr_type", "list_status", "list_date", "delist_date", "is_hs",
		"act_name", "act_ent_type", "updated_at",
	}

	// 4. 在事务中增量写入：本地已存在的股票按主键更新基础字段，其余执行新增
	return global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var toCreate []quant.BaseStock
		var toUpdate []quant.BaseStock
		for i := range allStocks {
			stock := allStocks[i]
			if stock.TsCode == nil || *stock.TsCode == "" {
				continue
			}
			if id, ok := tsCodeToID[*stock.TsCode]; ok {
				stock.GVA_MODEL_ADDON = global.GVA_MODEL_ADDON{ID: id}
				toUpdate = append(toUpdate, stock)
			} else {
				toCreate = append(toCreate, stock)
			}
		}

		if len(toCreate) > 0 {
			if err := tx.CreateInBatches(toCreate, 100).Error; err != nil {
				return fmt.Errorf("failed to batch insert new stocks: %w", err)
			}
		}

		if len(toUpdate) > 0 {
			// 基于主键冲突批量更新基础字段，保留本地 change_pct 与 AI 分析结果
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns(updateColumns),
			}).CreateInBatches(toUpdate, 100).Error; err != nil {
				return fmt.Errorf("failed to batch update stocks: %w", err)
			}
		}

		global.GVA_LOG.Info("successfully synced stocks",
			zap.Int("total", len(allStocks)),
			zap.Int("created", len(toCreate)),
			zap.Int("updated", len(toUpdate)))
		return nil
	})
}

// Clear 清除全部基础股票数据
func (baseStockService *BaseStockService) Clear(ctx context.Context) (err error) {
	// 使用 Unscoped() 来永久删除所有记录，而不是软删除
	err = global.GVA_DB.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&quant.BaseStock{}).Error
	return err
}

// callTushare 通用调用 Tushare API
// Tushare 官方接口统一 POST 到 apiUrl 根路径，接口名（api_name）放在请求体 JSON 中
func (baseStockService *BaseStockService) callTushare(ctx context.Context, token, apiUrl, apiName string, params map[string]interface{}, fields string) (*TushareResponse, error) {
	requestPayload := map[string]interface{}{
		"api_name": apiName,
		"token":    token,
		"params":   params,
		"fields":   fields,
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	global.GVA_LOG.Debug("Tushare API raw response", zap.String("api", apiName), zap.String("body", string(body)))

	var tushareResp TushareResponse
	if err := json.Unmarshal(body, &tushareResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tushare response: %w", err)
	}

	if tushareResp.Code != 0 {
		return nil, fmt.Errorf("tushare API error: %s", tushareResp.Msg)
	}

	return &tushareResp, nil
}

// fetchChangePctByTradeDate 按交易日分页拉取全市场涨跌幅，返回 ts_code -> pct_chg（百分比）映射
func (baseStockService *BaseStockService) fetchChangePctByTradeDate(ctx context.Context, token, apiUrl, tradeDate string) (map[string]float64, error) {
	changePctMap := make(map[string]float64)
	limit := 6000 // Tushare daily 单次最大返回条数
	offset := 0

	for {
		tushareResp, err := baseStockService.callTushare(ctx, token, apiUrl, "daily", map[string]interface{}{
			"trade_date": tradeDate,
			"limit":      limit,
			"offset":     offset,
		}, "ts_code,pct_chg")
		if err != nil {
			return nil, err
		}

		// 如果 fields 为空，说明当天非交易日或无数据，返回空 map 而非错误
		if len(tushareResp.Data.Fields) == 0 {
			return changePctMap, nil
		}
		fieldMap := make(map[string]int)
		for i, field := range tushareResp.Data.Fields {
			fieldMap[field] = i
		}
		tsIdx, okTs := fieldMap["ts_code"]
		pctIdx, okPct := fieldMap["pct_chg"]
		if !okTs || !okPct {
			return nil, fmt.Errorf("daily response missing required fields (ts_code/pct_chg)")
		}

		items := tushareResp.Data.Items
		for _, item := range items {
			if tsIdx >= len(item) || pctIdx >= len(item) {
				continue
			}
			tsCode, _ := item[tsIdx].(string)
			pctVal, ok := toFloat64(item[pctIdx])
			if !ok || tsCode == "" {
				continue
			}
			changePctMap[tsCode] = pctVal
		}

		// 优先以接口返回的 has_more 判断是否还有下一页，否则按返回条数判断
		if tushareResp.Data.HasMore != nil {
			if !*tushareResp.Data.HasMore {
				break
			}
		} else if len(items) < limit {
			break
		}
		offset += limit
	}
	return changePctMap, nil
}

// toFloat64 将 Tushare 返回的数值（float64 或字符串数字）转为 float64
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// UpdateAllChangePct 一键更新全部股票涨跌幅
// 通过 Tushare daily 接口获取全市场行情，将 pct_chg 批量写入 change_pct，返回成功更新的股票数量
// 从今天起向前最多 7 天逐日查询，取第一个有数据的交易日（避免依赖 trade_cal 接口的频率限制）
func (baseStockService *BaseStockService) UpdateAllChangePct(ctx context.Context) (updatedCount int, err error) {
	token := global.GVA_CONFIG.Tushare.Token
	apiUrl := global.GVA_CONFIG.Tushare.ApiUrl
	if token == "" || apiUrl == "" {
		return 0, fmt.Errorf("tushare token or api_url is not configured")
	}

	// 1. 从今天起向前最多 7 天，逐日拉取全市场涨跌幅，取首个有数据的交易日
	var changePctMap map[string]float64
	tradeDate := ""
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("20060102")
		changePctMap, err = baseStockService.fetchChangePctByTradeDate(ctx, token, apiUrl, date)
		if err != nil {
			return 0, fmt.Errorf("获取行情数据失败: %w", err)
		}
		if len(changePctMap) > 0 {
			tradeDate = date
			break
		}
	}
	if len(changePctMap) == 0 {
		return 0, fmt.Errorf("最近 7 天内未获取到有效的交易日行情数据")
	}

	// 2. 读取本地股票 ts_code -> id 映射
	var stocks []quant.BaseStock
	if err := global.GVA_DB.WithContext(ctx).Select("id", "ts_code").Find(&stocks).Error; err != nil {
		return 0, fmt.Errorf("查询本地股票失败: %w", err)
	}
	tsCodeToID := make(map[string]uint, len(stocks))
	for i := range stocks {
		if stocks[i].TsCode != nil && *stocks[i].TsCode != "" {
			tsCodeToID[*stocks[i].TsCode] = stocks[i].ID
		}
	}

	// 3. 将行情映射组装为带主键的批量更新对象
	updates := make([]quant.BaseStock, 0, len(changePctMap))
	for tsCode, pct := range changePctMap {
		id, ok := tsCodeToID[tsCode]
		if !ok {
			continue
		}
		p := pct
		updates = append(updates, quant.BaseStock{
			GVA_MODEL_ADDON: global.GVA_MODEL_ADDON{ID: id},
			ChangePct:       &p,
		})
	}

	if len(updates) == 0 {
		global.GVA_LOG.Warn("no local stock matched change_pct data")
		return 0, nil
	}

	// 4. 基于主键冲突更新 change_pct，批量写入替代逐条 UPDATE，减少数据库往返次数
	err = global.GVA_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"change_pct", "updated_at"}),
		}).CreateInBatches(updates, 100).Error; err != nil {
			return fmt.Errorf("批量更新股票涨跌幅失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	global.GVA_LOG.Info("successfully updated all change_pct", zap.String("trade_date", tradeDate), zap.Int("updated", len(updates)))
	return len(updates), nil
}
