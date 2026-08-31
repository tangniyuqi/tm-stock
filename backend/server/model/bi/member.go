// 自动生成模板Member
package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 用户 结构体  Member
type Member struct {
	global.GVA_MODEL_ADDON
	ClientId  *int    `json:"client_id" form:"client_id" gorm:"default:0;comment:客户端ID;column:client_id;size:10;"` //客户端ID
	GroupId   *int    `json:"group_id" form:"group_id" gorm:"default:0;comment:群组ID;column:group_id;size:10;"`     //群组ID
	MsgId     *int    `json:"msg_id" form:"msg_id" gorm:"default:0;comment:消息ID;column:msg_id;size:10;"`           //消息ID
	Gid       *string `json:"gid" form:"gid" gorm:"comment:GID;column:gid;size:50;"`                               //GID
	Uid       *string `json:"uid" form:"uid" gorm:"comment:UID;column:uid;size:50;"`                               //UID
	Avatar    *string `json:"avatar" form:"avatar" gorm:"comment:头像;column:avatar;size:250;"`                      //头像
	Nickname  *string `json:"nickname" form:"nickname" gorm:"comment:昵称;column:nickname;size:250;"`                //昵称
	Name      *string `json:"name" form:"name" gorm:"comment:姓名;column:name;size:250;"`                            //姓名
	Business  *string `json:"business" form:"business" gorm:"comment:业务;column:business;size:250;"`                //业务
	Mobile    *string `json:"mobile" form:"mobile" gorm:"comment:电话;column:mobile;size:11;default:null"`           //电话
	Wechat    *string `json:"wechat" form:"wechat" gorm:"comment:微信;column:wechat;size:50;default:null"`           //微信
	Qq        *string `json:"qq" form:"qq" gorm:"comment:QQ;column:qq;size:50;"`                                   //QQ
	Douyin    *string `json:"douyin" form:"douyin" gorm:"comment:抖音;column:douyin;size:50;"`                       //抖音
	Tiktok    *string `json:"tiktok" form:"tiktok" gorm:"comment:TikTok;column:tiktok;size:50;"`                   //TikTok
	City      *string `json:"city" form:"city" gorm:"comment:城市;column:city;size:50;"`                             //城市
	Remark    *string `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                      //备注
	Status    *int    `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`              //状态
}

// TableName 用户 Member自定义表名 addon_bi_im_member
func (Member) TableName() string {
	return "addon_bi_im_member"
}
