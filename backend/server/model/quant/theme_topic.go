// 自动生成模板ThemeTopic
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 题材话题 结构体 ThemeTopic
type ThemeTopic struct {
	global.GVA_MODEL_ADDON
	ThemeId     *int64     `json:"theme_id" form:"theme_id" gorm:"default:0;comment:题材ID;column:theme_id;"`             //题材ID
	ThemeName   *string    `json:"theme_name" form:"theme_name" gorm:"comment:题材名称;column:theme_name;size:100;"`        //题材名称
	Title       *string    `json:"title" form:"title" gorm:"comment:标题;column:title;size:200;" binding:"required"`      //标题
	Type        *int8      `json:"type" form:"type" gorm:"default:0;comment:类型;column:type;"`                           //类型
	Summary     *string    `json:"summary" form:"summary" gorm:"comment:摘要;column:summary;size:255;"`                   //摘要
	Content     *string    `json:"content" form:"content" gorm:"comment:内容;column:content;type:text;"`                  //内容
	Sentiment   *int8      `json:"sentiment" form:"sentiment" gorm:"default:0;comment:情绪;column:sentiment;"`            //情绪
	Source      *string    `json:"source" form:"source" gorm:"comment:来源;column:source;size:50;"`                       //来源
	Url         *string    `json:"url" form:"url" gorm:"comment:外链;column:url;size:200;"`                               //外链
	PublishTime *time.Time `json:"publish_time" form:"publish_time" gorm:"comment:发布时间;column:publish_time;"`           //发布时间
	PublishDate *time.Time `json:"publish_date" form:"publish_date" gorm:"type:date;comment:发布日期;column:publish_date;"` //发布日期
	Sort        *int32     `json:"sort" form:"sort" gorm:"default:0;comment:排序;column:sort;"`                           //排序
	Top         *int8      `json:"top" form:"top" gorm:"default:0;comment:置顶;column:top;"`                              //置顶
	Hot         *int8      `json:"hot" form:"hot" gorm:"default:0;comment:热门;column:hot;"`                              //热门
	Heat        *int32     `json:"heat" form:"heat" gorm:"default:0;comment:热度值;column:heat;"`                          //热度值
	View        *int32     `json:"view" form:"view" gorm:"default:0;comment:浏览量;column:view;"`                          //浏览量
	Share       *int32     `json:"share" form:"share" gorm:"default:0;comment:分享数;column:share;"`                       //分享数
	Status      *int8      `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;"`                     //状态
}

// TableName 题材话题 ThemeTopic自定义表名 addon_quant_theme_topic
func (ThemeTopic) TableName() string {
	return "addon_quant_theme_topic"
}
