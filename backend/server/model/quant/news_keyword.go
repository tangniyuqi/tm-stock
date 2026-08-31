// 自动生成模板NewsKeyword
package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 关键词 结构体  NewsKeyword
type NewsKeyword struct {
	global.GVA_MODEL_ADDON
	Name      *string `json:"name" form:"name" gorm:"comment:关键词;column:name;size:250;" binding:"required"` //关键词
	Times     *int    `json:"times" form:"times" gorm:"default:0;comment:次数;column:times;size:10;"`         //次数
	Status    *int    `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`       //状态
}

// TableName 关键词 NewsKeyword自定义表名 addon_quant_news_keyword
func (NewsKeyword) TableName() string {
	return "addon_quant_news_keyword"
}
