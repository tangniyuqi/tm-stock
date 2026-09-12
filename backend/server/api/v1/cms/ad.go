package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/cms"
	cmsReq "github.com/flipped-aurora/gin-vue-admin/server/model/cms/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type AdApi struct{}

// CreateAd 创建广告表
// @Tags Ad
// @Summary 创建广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Ad true "创建广告表"
// @Success 200 {object} response.Response{msg=string} "创建成功"
// @Router /ad/createAd [post]
func (adApi *AdApi) CreateAd(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var ad cms.Ad
	err := c.ShouldBindJSON(&ad)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ad.CreatedBy = utils.GetUserID(c)
	err = adService.CreateAd(ctx, &ad)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("创建失败!")
		response.FailWithMessage("创建失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("创建成功", c)
}

// DeleteAd 删除广告表
// @Tags Ad
// @Summary 删除广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Ad true "删除广告表"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /ad/deleteAd [delete]
func (adApi *AdApi) DeleteAd(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	userID := utils.GetUserID(c)
	err := adService.DeleteAd(ctx, id, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("删除失败!")
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteAdByIds 批量删除广告表
// @Tags Ad
// @Summary 批量删除广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /ad/deleteAdByIds [delete]
func (adApi *AdApi) DeleteAdByIds(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	ids := c.QueryArray("ids[]")
	userID := utils.GetUserID(c)
	err := adService.DeleteAdByIds(ctx, ids, userID)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("批量删除失败!")
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}

// UpdateAd 更新广告表
// @Tags Ad
// @Summary 更新广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data body cms.Ad true "更新广告表"
// @Success 200 {object} response.Response{msg=string} "更新成功"
// @Router /ad/updateAd [put]
func (adApi *AdApi) UpdateAd(c *gin.Context) {
	// 从ctx获取标准context进行业务行为
	ctx := c.Request.Context()

	var ad cms.Ad
	err := c.ShouldBindJSON(&ad)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ad.UpdatedBy = utils.GetUserID(c)
	err = adService.UpdateAd(ctx, ad)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("更新失败!")
		response.FailWithMessage("更新失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("更新成功", c)
}

// FindAd 用id查询广告表
// @Tags Ad
// @Summary 用id查询广告表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ID query uint true "用id查询广告表"
// @Success 200 {object} response.Response{data=cms.Ad,msg=string} "查询成功"
// @Router /ad/findAd [get]
func (adApi *AdApi) FindAd(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	id := c.Query("id")
	read, err := adService.GetAd(ctx, id)
	if err != nil {
		logger.WithCtx(ctx).Mod("biz").Err(err).Error("查询失败!")
		response.FailWithMessage("查询失败:"+err.Error(), c)
		return
	}
	response.OkWithData(read, c)
}

// GetAdList 分页获取广告表列表
// @Tags Ad
// @Summary 分页获取广告表列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query cmsReq.AdSearch true "分页获取广告表列表"
// @Success 200 {object} response.Response{data=response.PageResult{list=[]cms.Ad},msg=string} "获取成功"
// @Router /ad/getAdList [get]
func (adApi *AdApi) GetAdList(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	var pageInfo cmsReq.AdSearch
	err := c.ShouldBindQuery(&pageInfo)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	list, total, err := adService.GetAdInfoList(ctx, pageInfo)
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

// GetAdPublic 不需要鉴权的广告表接口
// @Tags Ad
// @Summary 不需要鉴权的广告表接口
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=object,msg=string} "获取成功"
// @Router /ad/getAdPublic [get]
func (adApi *AdApi) GetAdPublic(c *gin.Context) {
	// 创建业务用Context
	ctx := c.Request.Context()

	// 此接口不需要鉴权
	// 示例为返回了一个固定的消息接口，一般本接口用于C端服务，需要自己实现业务逻辑
	adService.GetAdPublic(ctx)
	response.OkWithDetailed(gin.H{
		"info": "不需要鉴权的广告表接口信息",
	}, "获取成功", c)
}
