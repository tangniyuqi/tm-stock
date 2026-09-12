package member

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	memberReq "github.com/flipped-aurora/gin-vue-admin/server/model/member/request"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SmsLogApi struct{}

// GetSmsLogList 分页获取短信日志列表
// @Tags MemberSmsLog
// @Summary 分页获取短信日志列表
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param data query memberReq.SmsLogSearch true "分页获取短信日志列表"
// @Success 200 {object} response.Response{data=response.PageResult,msg=string} "获取成功"
// @Router /member/smsLog/getSmsLogList [get]
func (a *SmsLogApi) GetSmsLogList(c *gin.Context) {
	ctx := c.Request.Context()

	var pageInfo memberReq.SmsLogSearch
	if err := c.ShouldBindQuery(&pageInfo); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}

	list, total, err := smsService.GetSmsLogList(ctx, pageInfo)
	if err != nil {
		global.GVA_LOG.Error("获取短信日志列表失败!", zap.Error(err))
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

// DeleteSmsLog 删除短信日志
// @Tags MemberSmsLog
// @Summary 删除短信日志
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param id query string true "日志ID"
// @Success 200 {object} response.Response{msg=string} "删除成功"
// @Router /member/smsLog/deleteSmsLog [delete]
func (a *SmsLogApi) DeleteSmsLog(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		response.FailWithMessage("参数错误", c)
		return
	}
	if err := smsService.DeleteSmsLog(ctx, uint(id)); err != nil {
		global.GVA_LOG.Error("删除短信日志失败!", zap.Error(err))
		response.FailWithMessage("删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("删除成功", c)
}

// DeleteSmsLogByIds 批量删除短信日志
// @Tags MemberSmsLog
// @Summary 批量删除短信日志
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param ids query []string true "日志ID列表"
// @Success 200 {object} response.Response{msg=string} "批量删除成功"
// @Router /member/smsLog/deleteSmsLogByIds [delete]
func (a *SmsLogApi) DeleteSmsLogByIds(c *gin.Context) {
	ctx := c.Request.Context()

	idsStr := c.QueryArray("ids[]")
	var ids []uint
	for _, s := range idsStr {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			response.FailWithMessage("参数错误", c)
			return
		}
		ids = append(ids, uint(v))
	}
	if err := smsService.DeleteSmsLogByIds(ctx, ids); err != nil {
		global.GVA_LOG.Error("批量删除短信日志失败!", zap.Error(err))
		response.FailWithMessage("批量删除失败:"+err.Error(), c)
		return
	}
	response.OkWithMessage("批量删除成功", c)
}
