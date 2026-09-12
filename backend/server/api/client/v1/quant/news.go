package quant

import (
	"encoding/json"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"github.com/flipped-aurora/gin-vue-admin/server/service"
	"github.com/gin-gonic/gin"
)

// newsService C 端快讯接口使用的 service 实例
var newsService = service.ServiceGroupApp.QuantServiceGroup.NewsService

type NewsApi struct{}

// ============ 7x24 快讯 ============

// List 7x24 快讯列表（支持关键字搜索与前端高亮）
// @Tags ClientQuantNews
// @Summary 获取 7x24 快讯列表，支持关键字搜索与前端高亮
// @Description 对应前端 pages/news/index：快讯流 + 搜索关键字高亮；keyword 命中标题/内容，前端按传入 keyword 做高亮；数据源 addon_quant_news，status=1 有效
// @Accept application/json
// @Produce application/json
// @Param keyword query string false "搜索关键字，命中标题/内容"
// @Param page query int false "页码，默认 1"
// @Param pageSize query int false "每页条数，默认 20，最大 100"
// @Success 200 {object} response.Response{data=response.PageResult{list=[]NewsItem},msg=string} "获取成功"
// @Router /client/quant/news/list [get]
func (n *NewsApi) List(c *gin.Context) {
	ctx := c.Request.Context()

	var pageInfo quantReq.NewsSearch
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
	// C 端 source：参数为空字符串时才视为未指定查询全部；
	// 显式传入 0 时按 0 过滤（绑定后无法区分空值与 0，需读原始 query）
	if c.Query("source") == "" {
		pageInfo.Source = nil
	}
	// C 端只返回有效数据（status=1），忽略前端传值避免暴露内部状态
	status := 1
	pageInfo.Status = &status

	list, total, err := newsService.GetNewsInfoList(ctx, pageInfo)
	if err != nil {
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}

	items := make([]NewsItem, 0, len(list))
	for _, news := range list {
		item := NewsItem{
			ID:      strconv.FormatUint(uint64(news.ID), 10),
			Title:   strVal(news.Title),
			Content: strVal(news.Content),
			Source:  int32Ptr(news.Source),
			IsHot:   intVal(news.Level) == 1,
		}
		if news.Ctime != nil {
			item.Ctime = news.Ctime.Format("01-02 15:04")
		}
		items = append(items, item)
	}

	response.OkWithDetailed(response.PageResult{
		List:     items,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// ============ 7x24 快讯详情 ============

// Detail 7x24 快讯详情（数据源 addon_quant_news，仅返回 status=1 有效）
// @Tags ClientQuantNews
// @Summary 获取单条 7x24 快讯详情
// @Description 对应前端 pages/news/detail：标题 + 正文全文 + 标签 + 发布时间；数据源 addon_quant_news，仅返回 status=1 有效数据
// @Accept application/json
// @Produce application/json
// @Param id query string true "快讯ID"
// @Success 200 {object} response.Response{data=NewsDetailResp,msg=string} "获取成功"
// @Router /client/quant/news/detail [get]
func (n *NewsApi) Detail(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Query("id")
	if id == "" {
		response.FailWithMessage("id不能为空", c)
		return
	}

	news, err := newsService.GetNews(ctx, id)
	if err != nil {
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	// C 端仅返回有效数据（status=1），避免暴露内部状态
	if intVal(news.Status) != 1 {
		response.FailWithMessage("获取失败:记录不存在", c)
		return
	}

	resp := NewsDetailResp{
		ID:      strconv.FormatUint(uint64(news.ID), 10),
		Title:   strVal(news.Title),
		Content: strVal(news.Content),
		Author:  strVal(news.Author),
		Source:  int32Ptr(news.Source),
		IsHot:   intVal(news.Level) == 1,
		View:    int32(intVal(news.View)),
		Share:   int32(intVal(news.Share)),
	}
	if len(news.Tags) > 0 {
		_ = json.Unmarshal(news.Tags, &resp.Tags)
	}
	if news.Ctime != nil {
		resp.Ctime = news.Ctime.Format("2006-01-02 15:04")
	}
	response.OkWithDetailed(resp, "获取成功", c)
}

// ============ DTO ============

// NewsItem 7x24 快讯单条
type NewsItem struct {
	ID       string `json:"id"`       // 快讯ID
	Title    string `json:"title"`    // 标题
	Content  string `json:"content"`  // 内容正文（搜索命中时前端按 keyword 高亮）
	Ctime    string `json:"ctime"`    // 发布时间，如 07-20 01:00
	Source   *int32 `json:"source"`   // 来源（字典值，quant_news_source），无来源时为 null
	IsHot    bool   `json:"isHot"`    // 是否热点置顶
	Cover    string `json:"cover"`    // 封面图 URL（可选）
	Expanded bool   `json:"expanded"` // 内容是否已展开（前端阅读状态位）
}

// NewsDetailResp 7x24 快讯详情
type NewsDetailResp struct {
	ID      string   `json:"id"`      // 快讯ID
	Title   string   `json:"title"`   // 标题
	Content string   `json:"content"` // 全文内容
	Tags    []string `json:"tags"`    // 标签列表
	Author  string   `json:"author"`  // 作者
	Source  *int32   `json:"source"`  // 来源（字典映射），无来源时为 null
	Ctime   string   `json:"ctime"`   // 发布时间，如 2006-01-02 15:04
	IsHot   bool     `json:"isHot"`   // 是否热点
	View    int32    `json:"view"`    // 浏览量
	Share   int32    `json:"share"`   // 分享数
}
