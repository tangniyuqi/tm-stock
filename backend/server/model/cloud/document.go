// 自动生成模板Document
package cloud

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 文档 结构体  Document
type Document struct {
	global.GVA_MODEL_ADDON
	MerchantId *int           `json:"merchant_id" form:"merchant_id" gorm:"default:0;column:merchant_id;comment:商户ID;size:10;"` //商户ID
	Title      *string        `json:"title" form:"title" gorm:"column:title;comment:标题;size:250;" binding:"required"`           //标题
	Type       *int           `json:"type" form:"type" gorm:"default:0;column:type;comment:类型;size:10;"`                        //分类ID
	Url        *string        `json:"url" form:"url" gorm:"column:url;comment:外链地址;size:250;"`                                  //外链地址
	Link       *string        `json:"link" form:"link" gorm:"column:link;comment:跳转地址;size:250;"`                               //跳转地址
	Summary    *string        `json:"summary" form:"summary" gorm:"column:summary;comment:摘要;size:250;"`                        //摘要
	Content    *string        `json:"content" form:"content" gorm:"column:content;comment:内容;type:text;"`                       //内容
	Tags       datatypes.JSON `json:"tags" form:"tags" gorm:"column:tags;comment:标签;type:text;" swaggertype:"array,object"`     //标签
	Sort       *int           `json:"sort" form:"sort" gorm:"default:0;column:sort;comment:排序;size:10;"`                        //排序
	View       *int           `json:"view" form:"view" gorm:"default:0;column:view;comment:浏览量;size:10;"`                       //浏览量
	Top        *int           `json:"top" form:"top" gorm:"default:0;column:top;comment:顶置;size:1;"`                            //顶置
	Digest     *int           `json:"digest" form:"digest" gorm:"default:0;column:digest;comment:精华;size:1;"`                   //精华
	Status     *int           `json:"status" form:"status" gorm:"default:1;column:status;comment:状态;size:1;"`                   //状态
}

// TableName 文档 Document自定义表名 addon_cloud_document
func (Document) TableName() string {
	return "addon_cloud_document"
}
