// 自动生成模板Group
package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 群组 结构体  Group
type Group struct {
	global.GVA_MODEL_ADDON
	ClientId  *int    `json:"client_id" form:"client_id" gorm:"default:0;comment:客户端ID;column:client_id;size:10;"` //客户端ID
	Gid       *string `json:"gid" form:"gid" gorm:"comment:GID;column:gid;size:50;"`                               //GID
	Name      *string `json:"name" form:"name" gorm:"comment:名称;column:name;size:50;"`                             //名称
	Industry  *string `json:"industry" form:"industry" gorm:"comment:行业;column:industry;size:250;"`                //行业
	City      *string `json:"city" form:"city" gorm:"comment:城市;column:city;size:50;"`                             //城市
	Remark    *string `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                      //备注
	Status    *int    `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`              //状态
}

// TableName 群组 Group自定义表名 addon_bi_im_group
func (Group) TableName() string {
	return "addon_bi_im_group"
}
