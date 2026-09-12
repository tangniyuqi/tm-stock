// 自动生成模板Ad
package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 广告表 结构体  Ad
type Ad struct {
	global.GVA_MODEL_ADDON
	Title      *string `json:"title" form:"title" gorm:"comment:标题;column:title;size:250;"`                              //标题
	Type       uint8   `json:"type" form:"type" gorm:"comment:类型;column:type;type:tinyint unsigned;not null;default:0;"` //类型
	Cover      *string `json:"cover" form:"cover" gorm:"comment:封面;column:cover;size:250;"`                              //封面
	LocationId *int32  `json:"location_id" form:"location_id" gorm:"comment:广告位ID;column:location_id;"`                  //广告位ID
	SilderText *string `json:"silder_text" form:"silder_text" gorm:"comment:文本描述;column:silder_text;size:250;"`          //文本描述
	LinkType   *int32  `json:"link_type" form:"link_type" gorm:"comment:链接类型;column:link_type;"`                         //链接类型
	LinkId     *int32  `json:"link_id" form:"link_id" gorm:"comment:链接ID;column:link_id;"`                               //链接ID
	StartTime  *int32  `json:"start_time" form:"start_time" gorm:"comment:开始时间;column:start_time;"`                      //开始时间
	EndTime    *int32  `json:"end_time" form:"end_time" gorm:"comment:结束时间;column:end_time;"`                            //结束时间
	Sort       uint32  `json:"sort" form:"sort" gorm:"comment:优先级;column:sort;type:int unsigned;not null;default:0;"`    //优先级
	Status     uint8   `json:"status" form:"status" gorm:"comment:状态;column:status;type:tinyint;not null;default:1;"`    //状态
}

// TableName 广告表 Ad自定义表名 addon_cms_ad
func (Ad) TableName() string {
	return "addon_cms_ad"
}
