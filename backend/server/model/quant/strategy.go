// 自动生成模板Strategy
package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 策略 结构体  Strategy
type Strategy struct {
	global.GVA_MODEL_ADDON
	MemberId  *int           `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;size:10;"` //用户ID
	Name      *string        `json:"name" form:"name" gorm:"comment:名称;column:name;size:100;"`                           //名称
	Alias     *string        `json:"alias" form:"alias" gorm:"comment:别名;column:alias;size:100;"`                         //别名
	Options   datatypes.JSON `json:"options" form:"options" gorm:"comment:配置;column:options;type:json;"` //表单配置
	Rules     datatypes.JSON `json:"rules" form:"rules" gorm:"comment:规则;column:rules;type:json;"`       //表单规则
	Remark    *string        `json:"remark" form:"remark" gorm:"comment:备注;column:remark;"`                              //备注
	Sort      *int           `json:"sort" form:"sort" gorm:"default:0;comment:排序;column:sort;size:10;"`                  //排序
	Status    *int           `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`             //状态
}

// TableName 策略 Strategy自定义表名 addon_quant_strategy
func (Strategy) TableName() string {
	return "addon_quant_strategy"
}
