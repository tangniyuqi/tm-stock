// 自动生成模板Ic
package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// IC 结构体  Ic
type Ic struct {
	global.GVA_MODEL_ADDON
	MsgId     *string `json:"msg_id" form:"msg_id" gorm:"default:0;comment:MID;column:msg_id;size:50;"`    //MID
	Sender    *string `json:"sender" form:"sender" gorm:"comment:发送者;column:sender;size:50;"`              //发送者
	Type      *int    `json:"type" form:"type" gorm:"default:0;comment:类型;column:type;size:10;"`           //类型
	Name      *string `json:"name" form:"name" gorm:"comment:名称;column:name;size:100;" binding:"required"` //名称
	Times     *int    `json:"times" form:"times" gorm:"default:0;comment:次数;column:times;size:10;"`        //次数
	Status    *int    `json:"status" form:"status" gorm:"default:0;comment:状态;column:status;size:1;"`      //状态
}

// TableName IC Ic自定义表名 addon_bi_im_ic
func (Ic) TableName() string {
	return "addon_bi_im_ic"
}
