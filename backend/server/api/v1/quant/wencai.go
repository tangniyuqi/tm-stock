package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type WencaiApi struct{}

// QueryWencai 问财选股查询
// @Tags Wencai
// @Summary 问财选股查询
// @Security ApiKeyAuth
// @Accept application/json
// @Produce application/json
// @Param query query string true "查询条件"
// @Success 200 {object} response.Response{data=object,msg=string} "查询成功"
// @Router /wencai/query [get]
func (wencaiApi *WencaiApi) QueryWencai(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		response.FailWithMessage("查询条件不能为空", c)
		return
	}

	// 使用 gin 的 context，支持请求取消
	data, err := wencaiService.QueryWithContext(c.Request.Context(), query, false, "")
	if err != nil {
		// 检查是否是上下文取消
		if c.Request.Context().Err() != nil {
			response.FailWithMessage("查询已取消", c)
			return
		}
		global.GVA_LOG.Error("查询失败!", zap.Error(err))
		response.FailWithMessage("查询失败: "+err.Error(), c)
		return
	}

	response.OkWithDetailed(data, "查询成功", c)
}
