package quant

import (
	"context"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/gin-gonic/gin"
)

var themeTopicService = service.ServiceGroupApp.QuantServiceGroup.ThemeTopicService

type ThemeTopicApi struct{}

// ============ 题材动态列表 ============

// List 题材动态列表（数据源 addon_quant_theme_topic type=2）
// @Tags ClientQuantTheme
// @Summary 获取题材动态列表
// @Description 对应前端 news/index 与首页「题材动态」tab：标题 + 题材标签 + 涨跌幅 + 发布时间
// @Accept application/json
// @Produce application/json
// @Param publish_date query string false "发布日 YYYY-MM-DD，默认当天（仅回显）"
// @Param page query int false "页码，默认 1"
// @Param pageSize query int false "每页条数，默认 20，最大 100"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /client/quant/themeTopic/list [get]
func (t *ThemeTopicApi) List(c *gin.Context) {
	ctx := c.Request.Context()
	var pageInfo quantReq.ThemeTopicSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	if pageInfo.PageSize <= 0 || pageInfo.PageSize > 100 {
		pageInfo.PageSize = 20
	}
	// 本接口固定展示「题材动态」资讯流，无需按 type 过滤
	status := 1
	pageInfo.Status = &status

	list, total, err := themeTopicService.GetThemeTopicInfoList(ctx, pageInfo)
	if err != nil {
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	response.OkWithDetailed(response.PageResult{
		List:     t.mapThemeTopics(ctx, list),
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// ============ 题材动态详情 ============

// Detail 获取单条题材动态详情（数据源 addon_quant_theme_topic）
// @Tags ClientQuantTheme
// @Summary 获取单条题材动态详情
// @Description 对应前端题材动态列表点击：标题 + 题材标签 + 情绪 + 摘要 + 全文 + 发布时间；数据源 addon_quant_theme_topic
// @Accept application/json
// @Produce application/json
// @Param id query string true "题材动态ID"
// @Success 200 {object} response.Response{data=TopicDetailResp,msg=string} "获取成功"
// @Router /client/quant/themeTopic/detail [get]
func (t *ThemeTopicApi) Detail(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Query("id")
	if id == "" {
		response.FailWithMessage("id不能为空", c)
		return
	}

	topic, err := themeTopicService.GetThemeTopic(ctx, id)
	if err != nil {
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	resp := TopicDetailResp{
		Id:        int64(topic.ID),
		ThemeId:   int64Val(topic.ThemeId),
		ThemeName: strVal(topic.ThemeName),
		Title:     strVal(topic.Title),
		Summary:   strVal(topic.Summary),
		Content:   strVal(topic.Content),
		Source:    strVal(topic.Source),
		Url:       strVal(topic.Url),
		Hot:       topic.Hot != nil && *topic.Hot == 1,
		View:      int32Val(topic.View),
		Share:     int32Val(topic.Share),
		Top:       topic.Top != nil && *topic.Top == 1,
	}
	if topic.Sentiment != nil {
		resp.Sentiment = *topic.Sentiment
	}
	if topic.Type != nil {
		resp.Type = *topic.Type
	}
	if topic.PublishTime != nil {
		resp.PublishTime = topic.PublishTime.Format("2006-01-02 15:04")
	}
	response.OkWithDetailed(resp, "获取成功", c)
}

// ============ 题材动态映射辅助 ============

// mapThemeTopics 题材话题（热点/动态/轮动）映射为资讯流条目，并关联题材涨跌幅
func (t *ThemeTopicApi) mapThemeTopics(ctx context.Context, list []quant.ThemeTopic) []ThemeHotItem {
	items := make([]ThemeHotItem, 0, len(list))
	for _, tp := range list {
		item := ThemeHotItem{
			ID:     strconv.FormatUint(uint64(tp.ID), 10),
			Title:  strVal(tp.Title),
			Tag:    strVal(tp.ThemeName),
			Change: t.loadThemeChange(ctx, tp.ThemeId),
			IsHot:  tp.Hot != nil && *tp.Hot == 1,
		}
		if tp.PublishTime != nil {
			item.Datetime = tp.PublishTime.Format("01-02 15:04")
		}
		items = append(items, item)
	}
	return items
}

// loadThemeChange 根据题材ID取涨跌幅展示串
func (t *ThemeTopicApi) loadThemeChange(ctx context.Context, themeID *int64) string {
	if themeID == nil {
		return ""
	}
	th, err := themeService.GetTheme(ctx, strconv.FormatInt(*themeID, 10))
	if err != nil {
		return ""
	}
	return formatChange(floatVal(th.ChangePct))
}

// ============ DTO ============

// ThemeHotItem 题材动态单条资讯
type ThemeHotItem struct {
	ID       string `json:"id"`       // 题材动态ID（用于详情跳转）
	Title    string `json:"title"`    // 标题
	Tag      string `json:"tag"`      // 题材标签
	Change   string `json:"change"`   // 涨跌幅展示串，如 +3.25% / -0.85%
	Datetime string `json:"datetime"` // 发布时间，如 07-20 01:00
	IsHot    bool   `json:"isHot"`    // 是否热点置顶
	Cover    string `json:"cover"`    // 封面图 URL（可选）
}

// TopicDetailResp 题材动态详情
type TopicDetailResp struct {
	Id          int64  `json:"id"`          // 动态ID
	ThemeId     int64  `json:"themeId"`     // 题材ID
	ThemeName   string `json:"themeName"`   // 题材名称
	Title       string `json:"title"`       // 标题
	Summary     string `json:"summary"`     // 摘要
	Content     string `json:"content"`     // 全文内容
	Type        int8   `json:"type"`        // 类型（1题材热点/2题材动态/3题材轮动）
	Sentiment   int8   `json:"sentiment"`   // 情绪
	Source      string `json:"source"`      // 来源
	Url         string `json:"url"`         // 外链
	Hot         bool   `json:"hot"`         // 是否热门
	Top         bool   `json:"top"`         // 是否置顶
	View        int32  `json:"view"`        // 浏览量
	Share       int32  `json:"share"`       // 分享数
	PublishTime string `json:"publishTime"` // 发布时间
}
