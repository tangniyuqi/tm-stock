package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UpdateApi struct{}

// ClientUpdate 获取客户端更新信息
// @Tags Update
// @Summary 获取客户端更新信息
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Success 200 {object} response.Response{data=map[string]interface{},msg=string} "获取成功"
// @Router /quant/update/client [get]
func (a *UpdateApi) ClientUpdate(c *gin.Context) {
	data, err := updateService.GetClientUpdate()
	if err != nil {
		global.GVA_LOG.Error("获取更新信息失败!", zap.Error(err))
		response.FailWithMessage("获取更新信息失败", c)
		return
	}
	response.OkWithDetailed(data, "获取成功", c)
}
