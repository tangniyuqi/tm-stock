// 自动生成模板Page
package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 单页 结构体  Page
type Page struct {
	global.GVA_MODEL_ADDON
	Title          *string        `json:"title" form:"title" gorm:"comment:标题;column:title;size:250;"`                                  //标题
	Type           uint8          `json:"type" form:"type" gorm:"comment:类型;column:type;type:tinyint unsigned;not null;default:0;"`     //类型
	Author         *string        `json:"author" form:"author" gorm:"comment:作者;column:author;size:250;"`                               //作者
	Link           *string        `json:"link" form:"link" gorm:"comment:链接;column:link;size:250;"`                                     //链接
	Name           *string        `json:"name" form:"name" gorm:"comment:标识;column:name;size:250;"`                                     //标识
	Cover          *string        `json:"cover" form:"cover" gorm:"comment:封面;column:cover;size:250;"`                                  //封面
	Images         datatypes.JSON `json:"images" form:"images" gorm:"comment:图集;column:images;" swaggertype:"object"`                   //图集
	SeoKeywords    *string        `json:"seo_keywords" form:"seo_keywords" gorm:"comment:SEO关键词;column:seo_keywords;size:250;"`         //SEO关键词
	SeoDescription *string        `json:"seo_description" form:"seo_description" gorm:"comment:SEO描述;column:seo_description;size:250;"` //SEO描述
	Description    *string        `json:"description" form:"description" gorm:"comment:简介;column:description;size:250;"`                //简介
	Content        *string        `json:"content" form:"content" gorm:"comment:内容;column:content;type:text;"`                           //内容
	Sort           uint32         `json:"sort" form:"sort" gorm:"comment:排序;column:sort;type:int unsigned;not null;default:0;"`         //排序
	View           uint32         `json:"view" form:"view" gorm:"comment:浏览量;column:view;type:int unsigned;not null;default:0;"`        //浏览量
	Review         uint8          `json:"review" form:"review" gorm:"comment:审核;column:review;type:int unsigned;not null;default:0;"`   //审核
	Status         uint8          `json:"status" form:"status" gorm:"comment:状态;column:status;type:int;not null;default:1;"`            //状态
}

// TableName 单页 Page自定义表名 addon_cms_page
func (Page) TableName() string {
	return "addon_cms_page"
}
