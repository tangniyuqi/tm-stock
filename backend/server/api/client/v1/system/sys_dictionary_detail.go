package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type DictionaryDetailApi struct{}

// GetTreeByType 按字典英名读取字典明细树（含启用的子项）
// @Tags      ClientSystemDictionaryDetail
// @Summary   按字典英名读取字典明细树（C 端免登录，层级结构）
// @Description 对应后台「系统设置-数据字典」的字典项树形结构，用于前端下拉/筛选映射
// @Accept    application/json
// @Produce   application/json
// @Param     type  query     string                                            true  "字典英名（如 quant_news_source）"
// @Success   200   {object}  response.Response{data=[]system.SysDictionaryDetail,msg=string}  "查询成功"
// @Router    /client/system/dictionaryDetail/tree [get]
func (s *DictionaryDetailApi) GetTreeByType(c *gin.Context) {
	dictType := c.Query("type")
	if dictType == "" {
		response.FailWithMessage("字典英名不能为空", c)
		return
	}
	list, err := dictionaryDetailService.GetDictionaryTreeListByType(c.Request.Context(), dictType)
	if err != nil {
		logger.WithCtx(c.Request.Context()).Mod("biz").Err(err).Error("获取字典明细失败!")
		response.FailWithMessage("获取失败: "+err.Error(), c)
		return
	}
	if list == nil {
		list = []system.SysDictionaryDetail{}
	}
	response.OkWithDetailed(list, "查询成功", c)
}
