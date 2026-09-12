package member

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ============ 资产（概况） ============

// GetAssetOverview 资产概况
// @Tags ClientMemberMember
// @Summary 获取当前会员资产概况（当前为积分，type=2）
// @Description 对应前端 api/member/asset.uts：当前可用、冻结、累计获得/消费/赠送
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=member.AssetOverview,msg=string} "获取成功"
// @Router /client/member/asset/detail [get]
func (u *MemberApi) GetAssetOverview(c *gin.Context) {
	ctx := c.Request.Context()
	overview, err := memberService.GetAssetOverview(ctx, getMemberID(c))
	if err != nil {
		global.GVA_LOG.Error("获取资产概况失败!", zap.Error(err))
		response.FailWithMessage("获取失败:"+err.Error(), c)
		return
	}
	response.OkWithDetailed(overview, "获取成功", c)
}
