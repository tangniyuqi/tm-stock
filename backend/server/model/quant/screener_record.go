// 自动生成模板ScreenerRecord
package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// AI选股器查询记录 结构体  ScreenerRecord
type ScreenerRecord struct {
	global.GVA_MODEL_ADDON
	MemberId  *uint32 `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;size:10;"` //用户ID
	Prompt    *string `json:"prompt" form:"prompt" gorm:"comment:提示词;column:prompt;size:250;"`                    //提示词
	Times     *int    `json:"times" form:"times" gorm:"default:1;comment:次数;column:times;"`                       //次数
	Status    *int8   `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`             //状态
}

// TableName AI选股器查询记录 ScreenerRecord自定义表名 addon_quant_screener_record
func (ScreenerRecord) TableName() string {
	return "addon_quant_screener_record"
}
