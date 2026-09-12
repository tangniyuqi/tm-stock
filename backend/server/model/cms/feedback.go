// 自动生成模板Feedback
package cms

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 反馈 结构体  Feedback
type Feedback struct {
	global.GVA_MODEL_ADDON
	MemberId uint32         `json:"member_id" form:"member_id" gorm:"comment:会员ID;column:member_id;type:int;not null;default:0;"`   //会员ID
	Title    *string        `json:"title" form:"title" gorm:"comment:标题;column:title;size:250;"`                                    //标题
	Type     uint8          `json:"type" form:"type" gorm:"comment:类型;column:type;type:tinyint unsigned;not null;default:0;"`       //类型
	Cover    *string        `json:"cover" form:"cover" gorm:"comment:封面;column:cover;size:250;"`                                    //封面
	Images   datatypes.JSON `json:"images" form:"images" gorm:"comment:图集;column:images;" swaggertype:"object"`                     //图集
	Content  *string        `json:"content" form:"content" gorm:"comment:内容;column:content;type:text;"`                             //内容
	Name     *string        `json:"name" form:"name" gorm:"comment:称呼;column:name;size:100;"`                                       //称呼
	Mobile   *string        `json:"mobile" form:"mobile" gorm:"comment:手机;column:mobile;size:100;"`                                 //手机
	Wechat   *string        `json:"wechat" form:"wechat" gorm:"comment:微信;column:wechat;size:100;"`                                 //微信
	Email    *string        `json:"email" form:"email" gorm:"comment:电子邮箱;column:email;size:100;"`                                  //电子邮箱
	Sort     uint32         `json:"sort" form:"sort" gorm:"comment:排序;column:sort;type:int unsigned;not null;default:0;"`           //排序
	Review   uint8          `json:"review" form:"review" gorm:"comment:审核;column:review;type:tinyint unsigned;not null;default:0;"` //审核
	Remark   *string        `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                                 //备注
	Status   uint8          `json:"status" form:"status" gorm:"comment:状态;column:status;type:tinyint;not null;default:1;"`          //状态
}

// TableName 反馈 Feedback自定义表名 addon_cms_feedback
func (Feedback) TableName() string {
	return "addon_cms_feedback"
}
