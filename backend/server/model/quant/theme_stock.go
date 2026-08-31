// 自动生成模板ThemeStock
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 题材股票 结构体  ThemeStock
type ThemeStock struct {
	global.GVA_MODEL_ADDON
	ThemeId   *int32     `json:"theme_id" form:"theme_id" gorm:"comment:题材ID;column:theme_id;"`                  //题材ID
	StockId   *int64     `json:"stock_id" form:"stock_id" gorm:"comment:股票ID ;column:stock_id;"`                 //股票ID
	Reason    *string    `json:"reason" form:"reason" gorm:"comment:入选逻辑;column:reason;type:text;"`              //入选逻辑
	AiReason  *string    `json:"ai_reason" form:"ai_reason" gorm:"comment:AI入选逻辑;column:ai_reason;type:text;"` //AI入选逻辑
	Tier      *int32     `json:"tier" form:"tier" gorm:"default:0;comment:梯队;column:tier;"`                      //梯队           //纳入日期
	Relevance *float64   `json:"relevance" form:"relevance" gorm:"comment:相关度;column:relevance;size:5;"`         //相关度
	InDate    *time.Time `json:"in_date" form:"in_date" gorm:"comment:纳入日期;column:in_date;"`
	Sort      *int32     `json:"sort" form:"sort" gorm:"default:0;comment:排序;column:sort;"` //排序
	Status    *int8      `json:"status" form:"status" gorm:"comment:状态;column:status;"`     //状态
	Theme     *Theme     `json:"theme,omitempty" gorm:"foreignKey:ThemeId"`
	Stock     *BaseStock `json:"stock,omitempty" gorm:"foreignKey:StockId"`
}

// TableName 题材股票 ThemeStock自定义表名 addon_quant_theme_stock
func (ThemeStock) TableName() string {
	return "addon_quant_theme_stock"
}
