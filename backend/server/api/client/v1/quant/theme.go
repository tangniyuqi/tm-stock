package quant

import (
	"context" // required for Treasure/buildFieldGroup
	"fmt"     // required for formatChange
	"sort"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/gin-gonic/gin"
)

var (
	themeService      = service.ServiceGroupApp.QuantServiceGroup.ThemeService
	themeStockService = service.ServiceGroupApp.QuantServiceGroup.ThemeStockService
)

// ============ 指针解引用辅助（本包共用） ============

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intVal(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// int32Ptr 将 *int 转为 *int32 指针
func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	r := int32(*v)
	return &r
}

func int32Val(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func int64Val(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func floatVal(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func formatChange(v float64) string {
	if v > 0 {
		return fmt.Sprintf("+%.2f%%", v)
	}
	return fmt.Sprintf("%.2f%%", v)
}

type ThemeApi struct{}

// ============ 题材宝典概览 ============

// Treasure 题材宝典概览（今日热点 + 题材全景表）
// @Tags ClientQuantTheme
// @Summary 题材宝典首页概览数据
// @Description 对应前端 pages/theme/treasure：KPI 今日热点题材 + 题材全景表；type 对应 全部题材/领涨题材/热度最高/新题材
// @Accept application/json
// @Produce application/json
// @Param type query string false "筛选：all/领涨/热度/新题材"
// @Success 200 {object} response.Response{data=quant.TreasureResp,msg=string} "获取成功"
// @Router /client/theme/treasure [get]
func (t *ThemeApi) Treasure(c *gin.Context) {
	ctx := c.Request.Context()
	filter := c.Query("type")

	themes, err := themeService.GetThemeListWithoutChildren(ctx, 0)
	if err != nil {
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	// 今日热点题材：按热度（题材股票数）取前 5
	byHeat := make([]*quant.Theme, len(themes))
	copy(byHeat, themes)
	sort.SliceStable(byHeat, func(i, j int) bool {
		return int32Val(byHeat[i].StockCount) > int32Val(byHeat[j].StockCount)
	})
	top := make([]TopThemeItem, 0, 5)
	for i, th := range byHeat {
		if i >= 5 {
			break
		}
		top = append(top, TopThemeItem{
			Name:   strVal(th.Name),
			Change: floatVal(th.ChangePct),
			Heat:   int(int32Val(th.StockCount)),
		})
	}

	// 题材全景表：按 type 排序
	list := make([]*quant.Theme, len(themes))
	copy(list, themes)
	switch filter {
	case "领涨":
		sort.SliceStable(list, func(i, j int) bool {
			return floatVal(list[i].ChangePct) > floatVal(list[j].ChangePct)
		})
	case "热度":
		sort.SliceStable(list, func(i, j int) bool {
			return int32Val(list[i].StockCount) > int32Val(list[j].StockCount)
		})
	case "新题材":
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].CreatedAt.After(list[j].CreatedAt)
		})
	}
	// 限制行数，规避领涨股 N+1 查询放大
	if len(list) > 100 {
		list = list[:100]
	}

	rows := make([]ThemeRowItem, 0, len(list))
	var latest time.Time
	for _, th := range list {
		rows = append(rows, ThemeRowItem{
			Id:      strconv.FormatUint(uint64(th.ID), 10),
			Name:    strVal(th.Name),
			Change:  floatVal(th.ChangePct),
			Heat:    int(int32Val(th.StockCount)),
			Leaders: t.loadThemeLeaders(ctx, int32(th.ID), 3),
		})
		if th.UpdatedAt.After(latest) {
			latest = th.UpdatedAt
		}
	}

	updateTime := ""
	if !latest.IsZero() {
		updateTime = latest.Format("15:04")
	}
	response.OkWithDetailed(TreasureResp{
		UpdateTime: updateTime,
		TopThemes:  top,
		ThemeList:  rows,
	}, "获取成功", c)
}

// loadThemeLeaders 取题材的领涨股名称（tier=1 龙头）
func (t *ThemeApi) loadThemeLeaders(ctx context.Context, themeID int32, limit int) []string {
	info := quantReq.ThemeStockSearch{}
	info.Page = 1
	info.PageSize = limit
	tier := int32(1)
	info.Tier = &tier
	info.ThemeId = &themeID
	list, _, err := themeStockService.GetThemeStockInfoList(ctx, info)
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(list))
	for _, ts := range list {
		if ts.Stock != nil && ts.Stock.Name != nil {
			names = append(names, *ts.Stock.Name)
		}
	}
	return names
}

// ============ 题材详情 ============

// Detail 题材详情（描述 + 细分领域 + 入选标的）
// @Tags ClientQuantTheme
// @Summary 获取题材详情及其细分领域与入选标的
// @Description 对应前端 pages/theme/index：OCS 简介、细分领域折叠面板、个股入选逻辑（精选逻辑 + AI逻辑）、题材更新耗时
// @Accept application/json
// @Produce application/json
// @Param id query string true "题材ID"
// @Success 200 {object} response.Response{data=quant.ThemeDetailResp,msg=string} "获取成功"
// @Router /client/theme/detail [get]
func (t *ThemeApi) Detail(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Query("id")
	if id == "" {
		response.FailWithMessage("id不能为空", c)
		return
	}

	theme, err := themeService.GetThemeWithChildren(ctx, id)
	if err != nil {
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	resp := ThemeDetailResp{
		Id:         id,
		Name:       strVal(theme.Name),
		Desc:       strVal(theme.Description),
		UpdateTime: theme.UpdatedAt.Format("15:04:05"),
		Fields:     []ThemeFieldGroup{},
	}
	for _, child := range theme.Children {
		resp.Fields = append(resp.Fields, t.buildFieldGroup(ctx, child))
	}
	response.OkWithDetailed(resp, "获取成功", c)
}

// buildFieldGroup 将题材子节点（细分领域）及其入选标的映射为详情分组
func (t *ThemeApi) buildFieldGroup(ctx context.Context, child *quant.Theme) ThemeFieldGroup {
	group := ThemeFieldGroup{
		Segment: SegmentInfo{
			Id:   int64(child.ID),
			Name: strVal(child.Name),
		},
		Desc:    strVal(child.Description),
		Symbols: []ThemeStockItem{},
	}
	themeID := int32(child.ID)
	info := quantReq.ThemeStockSearch{}
	info.Page = 1
	info.PageSize = 100
	info.ThemeId = &themeID
	list, _, err := themeStockService.GetThemeStockInfoList(ctx, info)
	if err != nil {
		return group
	}
	for _, ts := range list {
		group.Symbols = append(group.Symbols, t.mapThemeStock(ts))
	}
	return group
}

// mapThemeStock 题材入选标的映射
func (t *ThemeApi) mapThemeStock(ts quant.ThemeStock) ThemeStockItem {
	item := ThemeStockItem{
		Key:          strconv.FormatUint(uint64(ts.ID), 10),
		Id:           int64Val(ts.StockId),
		Reason:       strVal(ts.Reason),
		ManualReason: strVal(ts.Reason),
		AiReason:     strVal(ts.AiReason),
	}
	if ts.Tier != nil && *ts.Tier == 1 {
		item.Leader = 1
	}
	if ts.Stock != nil {
		item.Name = strVal(ts.Stock.Name)
		item.Pct = floatVal(ts.Stock.ChangePct)
	}
	if ts.InDate != nil {
		item.UpdateAt = ts.InDate.Format("2006-01-02 15:04")
	}
	return item
}

// ============ DTO ============

// TopThemeItem 今日热点题材 KPI
type TopThemeItem struct {
	Name   string  `json:"name"`   // 题材名称
	Change float64 `json:"change"` // 涨跌幅（%）
	Heat   int     `json:"heat"`   // 热度值
}

// ThemeRowItem 题材全景表行
type ThemeRowItem struct {
	Id      string   `json:"id"`      // 题材ID
	Name    string   `json:"name"`    // 题材名称
	Change  float64  `json:"change"`  // 涨跌幅（%）
	Heat    int      `json:"heat"`    // 热度
	Leaders []string `json:"leaders"` // 领涨股名称列表
}

// TreasureResp 题材宝典概览
type TreasureResp struct {
	UpdateTime string         `json:"updateTime"` // 数据更新时间，如 09:45
	TopThemes  []TopThemeItem `json:"topThemes"`  // 今日热点题材（KPI）
	ThemeList  []ThemeRowItem `json:"themeList"`  // 题材全景表
}

// SegmentInfo 细分领域基础信息
type SegmentInfo struct {
	Id   int64  `json:"id"`   // 细分领域ID
	Name string `json:"name"` // 细分领域名称
}

// ThemeStockItem 入选标的
type ThemeStockItem struct {
	Key          string  `json:"key"`          // 行标识
	Id           int64   `json:"id"`           // 标的ID
	Name         string  `json:"name"`         // 股票名称
	Leader       int     `json:"leader"`       // 是否龙头：1是 / 0否
	Pct          float64 `json:"pct"`          // 涨跌幅（%）
	Reason       string  `json:"reason"`       // 入选原因（列表展示）
	ManualReason string  `json:"manualReason"` // 精选逻辑（人工，抽屉展示）
	AiReason     string  `json:"aiReason"`     // AI 入选逻辑（抽屉展示）
	UpdateAt     string  `json:"updateAt"`     // 入选逻辑更新时间
}

// ThemeFieldGroup 题材下的一个细分领域分组
type ThemeFieldGroup struct {
	Segment SegmentInfo      `json:"segment"` // 细分领域
	Desc    string           `json:"desc"`    // 细分领域描述
	Symbols []ThemeStockItem `json:"symbols"` // 该领域下入选标的
}

// ThemeDetailResp 题材详情
type ThemeDetailResp struct {
	Id         string            `json:"id"`         // 题材ID
	Name       string            `json:"name"`       // 题材名称
	Desc       string            `json:"desc"`       // 题材简介（OCS）
	Fields     []ThemeFieldGroup `json:"fields"`     // 细分领域分组
	UpdateTime string            `json:"updateTime"` // 题材更新耗时
}
