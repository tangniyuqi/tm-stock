// 自动生成模板Msg
package bi

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 消息 结构体  Msg
type Msg struct {
	global.GVA_MODEL_ADDON
	ClientId     *int           `json:"client_id" form:"client_id" gorm:"default:0;comment:客户端ID;column:client_id;size:10;"`   //客户端ID
	SenderId     *string        `json:"sender_id" form:"sender_id" gorm:"comment:发送者ID;column:sender_id;size:50;"`             //发送者ID
	ReceiverId   *string        `json:"receiver_id" form:"receiver_id" gorm:"comment:接收者ID;column:receiver_id;size:50;"`       //接收者ID
	MsgId        *string        `json:"msg_id" form:"msg_id" gorm:"comment:MID;column:msg_id;size:50;""`                       //MID
	Gid          *string        `json:"gid" form:"gid" gorm:"comment:GID;column:gid;size:50;"`                                 //GID
	Group        *string        `json:"group" form:"group" gorm:"comment:群组;column:group;size:50;"`                            //群组
	Type         *string        `json:"type" form:"type" gorm:"comment:类型;column:type;size:10;"`                               //类型
	Sender       *string        `json:"sender" form:"sender" gorm:"comment:发送者;column:sender;size:50;"`                        //发送者
	SenderRemark *string        `json:"sender_remark" form:"sender_remark" gorm:"comment:发送者备注;column:sender_remark;size:50;"` //发送者备注
	Receiver     *string        `json:"receiver" form:"receiver" gorm:"comment:接收者;column:receiver;size:50;"`                  //接收者
	SendTime     *time.Time     `json:"send_time" form:"send_time" gorm:"comment:发送时间;column:send_time;"`                      //发送时间
	Content      *string        `json:"content" form:"content" gorm:"type:text;comment:内容;column:content;"`                    //内容
	ICS          datatypes.JSON `json:"ics" form:"ics" gorm:"column:ics;" swaggertype:"object"`                                //ICS
	Priority     *int           `json:"priority" form:"priority" gorm:"default:0;comment:优先级;column:priority;size:1;"`         //优先级
	Oppty        *int           `json:"oppty" form:"oppty" gorm:"default:0;comment:机会;column:oppty;size:1;"`                   //机会
	Level        *int           `json:"level" form:"level" gorm:"default:0;comment:等级;column:level;size:1;"`                   //状态
	Remark       *string        `json:"remark" form:"remark" gorm:"type:text;comment:备注;column:remark;"`                       //备注
	Status       *int           `json:"status" form:"status" gorm:"default:0;comment:状态;column:status;size:1;"`                //状态
}

// TableName 消息 Msg自定义表名 addon_bi_im_msg
func (Msg) TableName() string {
	return "addon_bi_im_msg"
}
