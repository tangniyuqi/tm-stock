package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type DictionaryApi struct{}

// FindByType 按字典英名读取字典（含启用明细）
// @Tags      ClientSystemDictionary
// @Summary   按字典英名读取字典与启用明细（C 端免登录）
// @Description 对应后台「系统设置-数据字典」；返回 SysDictionary 含 SysDictionaryDetails（仅启用项）
// @Accept    application/json
// @Produce   application/json
// @Param     type  query     string                         true  "字典英名（如 quant_news_source）"
// @Success   200   {object}  response.Response{data=system.SysDictionary,msg=string}  "查询成功"
// @Router    /client/system/dictionary [get]
func (s *DictionaryApi) FindByType(c *gin.Context) {
	dictType := c.Query("type")
	if dictType == "" {
		response.FailWithMessage("字典英名不能为空", c)
		return
	}
	dict, err := dictionaryService.GetSysDictionary(c.Request.Context(), dictType, 0, nil)
	if err != nil {
		logger.WithCtx(c.Request.Context()).Mod("biz").Err(err).Error("字典未创建或未开启!")
		response.FailWithMessage("字典未创建或未开启", c)
		return
	}
	response.OkWithDetailed(dict, "查询成功", c)
}
