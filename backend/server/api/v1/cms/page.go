package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type PageApi struct{}

// CreatePage 创建单页
// @Tags Page
// @Summary 创建单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Page true "创建单页"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /page/createPage [post]
func (pageApi *PageApi) CreatePage(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var page cms.Page
	err := c.ShouldBindJSON(&page)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	page.CreatedBy = utils.GetUserID(c)
	err = pageService.CreatePage(ctx, &page)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("创建失败!")
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeletePage 删除单页
// @Tags Page
// @Summary 删除单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Page true "删除单页"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /page/deletePage [delete]
func (pageApi *PageApi) DeletePage(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := pageService.DeletePage(ctx, id, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("删除失败!")
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeletePageByIds 批量删除单页
// @Tags Page
// @Summary 批量删除单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /page/deletePageByIds [delete]
func (pageApi *PageApi) DeletePageByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := pageService.DeletePageByIds(ctx, ids, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("批量删除失败!")
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdatePage 更新单页
// @Tags Page
// @Summary 更新单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Page true "更新单页"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /page/updatePage [put]
func (pageApi *PageApi) UpdatePage(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var page cms.Page
	err := c.ShouldBindJSON(&page)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	page.UpdatedBy = utils.GetUserID(c)
	err = pageService.UpdatePage(ctx, page)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("更新失败!")
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindPage 用id查询单页
// @Tags Page
// @Summary 用id查询单页
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询单页"
// @Success 200 {object} response.Response{data=cms.Page,msg=string} "查询成功"
// @Router /page/findPage [get]
func (pageApi *PageApi) FindPage(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	repage, err := pageService.GetPage(ctx, id)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("查询失败!")
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(repage, c)
}

// GetPageList 分页获取单页列表
// @Tags Page
// @Summary 分页获取单页列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cmsReq.PageSearch true "分页获取单页列表"
// @Success 200 {object} response.Response{data=response.PageResult{list=[]cms.Page},msg=string} "获取成功"
// @Router /page/getPageList [get]
func (pageApi *PageApi) GetPageList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo cmsReq.PageSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := pageService.GetPageInfoList(ctx, pageInfo)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("获取失败!")
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(response.PageResult{
		List:     list,
		Total:    total,
		Page:     pageInfo.Page,
		PageSize: pageInfo.PageSize,
	}, "获取成功", c)
}

// GetPagePublic 不需要鉴权的单页接口（按 name 获取）
// @Tags Page
// @Summary 不需要鉴权的单页接口（按标识名获取，如 about/contact）
// @Accept application/json
// @Produce application/json
// @Param name query string true "单页标识（如 about、contact）"
// @Success 200 {object} response.Response{data=cms.Page,msg=string} "获取成功"
// @Router /page/getPagePublic [get]
func (pageApi *PageApi) GetPagePublic(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Query("name")
	if name == "" {
		response.FailWithMessage("name 参数不能为空", c)
		return
	}
	page, err := pageService.GetPagePublic(ctx, name)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("获取单页失败!")
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithData(page, c)
}
