// 自动生成模板News
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 快讯 结构体  News
type News struct {
	global.GVA_MODEL_ADDON
	NewsId    *int           `json:"news_id" form:"news_id" gorm:"default:0;column:news_id;comment:快讯ID;size:10;"`          //快讯ID
	Title     *string        `json:"title" form:"title" gorm:"column:title;comment:标题;size:250;"`                           //标题
	Content   *string        `json:"content" form:"content" gorm:"column:content;comment:内容;type:text;" binding:"required"` //内容
	Tags      datatypes.JSON `json:"tags" form:"tags" gorm:"column:tags;comment:标签;type:json;" swaggertype:"object"`        //标签
	Author    *string        `json:"author" form:"author" gorm:"column:author;comment:作者;size:50;"`                         //作者
	Source    *int           `json:"source" form:"source" gorm:"default:0;column:source;comment:来源;size:1;"`                //来源
	Nature    *int           `json:"nature" form:"nature" gorm:"default:0;column:nature;comment:性质;size:1;"`                //性质
	Level     *int           `json:"level" form:"level" gorm:"default:0;column:level;comment:等级;size:1;"`                   //等级
	Bold      *int           `json:"bold" form:"bold" gorm:"default:0;column:bold;comment:加粗;size:1;"`                      //加粗
	View      *int           `json:"view" form:"view" gorm:"default:0;column:view;comment:浏览量;size:10;"`                    //浏览量
	Share     *int           `json:"share" form:"share" gorm:"default:0;column:share;comment:分享数;size:10;"`                 //分享数
	Ctime     *time.Time     `json:"ctime" form:"ctime" gorm:"column:ctime;comment:发布时间;type:datetime(0);"`                //发布时间
	Status    *int           `json:"status" form:"status" gorm:"default:1;column:status;comment:状态;size:1;"`                //状态
}

// TableName 快讯 News自定义表名 addon_quant_news
func (News) TableName() string {
	return "addon_quant_news"
}
