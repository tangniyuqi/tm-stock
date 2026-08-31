// 自动生成模板Follow
package bi

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 跟进 结构体  Follow
type Follow struct {
	global.GVA_MODEL_ADDON
	DealId     *int       `json:"deal_id" form:"deal_id" gorm:"default:0;comment:商机ID;column:deal_id;size:10;"`             //商机ID
	MsgId      *int       `json:"msg_id" form:"msg_id" gorm:"default:0;comment:消息ID;column:msg_id;size:10;"`                //消息ID
	MemberId   *int       `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;size:10;"`       //用户ID
	CustomerId *int       `json:"customer_id" form:"customer_id" gorm:"default:0;comment:客户ID;column:customer_id;size:10;"` //客户ID
	FollowTime *time.Time `json:"follow_time" form:"follow_time" gorm:"comment:跟进时间;column:follow_time;size:3;"`            //跟进时间
	Type       *int       `json:"type" form:"type" gorm:"default:0;comment:跟进方式;column:type;size:1;"`                       //跟进方式
	Content    *string    `json:"content" form:"content" gorm:"comment:跟进内容;column:content;" binding:"required"`            //跟进内容
	Plan       *string    `json:"plan" form:"plan" gorm:"comment:下次跟进计划;column:plan;"`                                      //下次跟进计划
	Feedback   *int       `json:"feedback" form:"feedback" gorm:"default:0;comment:客户反馈;column:feedback;size:1;"`           //反馈类型
	Reason     *string    `json:"reason" form:"reason" gorm:"comment:原因;column:reason;"`                                    //原因
}

// TableName 跟进 Follow自定义表名 addon_bi_im_follow
func (Follow) TableName() string {
	return "addon_bi_im_follow"
}
