// 自动生成模板Event
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 事件 结构体  Event
type Event struct {
	global.GVA_MODEL_ADDON
	Name      *string    `json:"name" form:"name" gorm:"comment:名称;column:name;size:250;" binding:"required"` //名称
	Industry  *string    `json:"industry" form:"industry" gorm:"comment:行业;column:industry;size:50;"`         //行业
	Date      *time.Time `json:"date" form:"date" gorm:"type:date;comment:日期;column:date;"`                      //日期
	City      *string    `json:"city" form:"city" gorm:"comment:城市;column:city;size:50;"`                     //城市
	Level     *int       `json:"level" form:"level" gorm:"default:0;comment:等级;column:level;size:1;"`         //等级
	Content   *string    `json:"content" form:"content" gorm:"comment:内容;column:content;"`                    //内容
	Remark    *string    `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`              //备注
	Status    *int       `json:"status" form:"status" gorm:"default:0;comment:状态;column:status;size:1;"`      //状态
}

// TableName 事件 Event自定义表名 addon_quant_event
func (Event) TableName() string {
	return "addon_quant_event"
}
