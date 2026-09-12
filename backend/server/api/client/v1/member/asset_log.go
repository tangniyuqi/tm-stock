package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/member"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ============ 资产流水（明细列表） ============

// List 资产流水 - 明细列表
// @Tags ClientMemberMember
// @Summary 获取当前会员资产流水（分页，默认积分）
// @Description 对应前端 api/member/asset_log.uts：各条资产变动数量 + 业务类型 + 备注
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param page query int false "页码，默认 1"
// @Param pageSize query int false "每页条数，默认 20，最大 100"
// @Success 200 {object} response.Response{data=response.PageResult{list=[]member.AssetLogItem},msg=string} "获取成功"
// @Router /client/member/asset_log/list [get]
func (u *MemberApi) List(c *gin.Context) {
	ctx := c.Request.Context()
	var info request.PageInfo
	if err := c.ShouldBindQuery(&info); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if info.Page < 1 {
		info.Page = 1
	}
	if info.PageSize <= 0 || info.PageSize > 100 {
		info.PageSize = 20
	}
	list, total, err := memberService.GetCreditLogs(ctx, getMemberID(c), info)
	if err != nil {
		global.GVA_LOG.Error("获取资产流水明细失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	items := make([]member.AssetLogItem, 0, len(list))
	for _, log := range list {
		item := member.AssetLogItem{
			ID:        log.ID,
			AfterNum:  log.AfterNum,
			FlowType:  log.Flow,
			BizType:   log.BizType,
			Remark:    log.Remark,
			CreatedAt: log.CreatedAt,
		}
		// change_num 为绝对值语义（资产流水表约定），正负由流向 flow 决定
		if log.Flow == 2 {
			item.Change = -log.ChangeNum
		} else {
			item.Change = log.ChangeNum
		}
		items = append(items, item)
	}
	response.OkWithDetailed(response.PageResult{
		List:     items,
		Total:    total,
		Page:     info.Page,
		PageSize: info.PageSize,
	}, "获取成功", c)
}
