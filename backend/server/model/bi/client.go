// 自动生成模板Client
package bi

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 客户端 结构体  Client
type Client struct {
	global.GVA_MODEL_ADDON
	Name      *string `json:"name" form:"name" gorm:"comment:名称;column:name;size:50;" binding:"required"`    //名称
	Server    *string `json:"server" form:"server" gorm:"comment:服务器;column:server;size:50;"`                //服务器
	ListType  *int    `json:"list_type" form:"list_type" gorm:"comment:名单类型;column:list_type;size:1;"`       //名单类型
	AllowList *string `json:"allow_list" form:"allow_list" gorm:"type:text;comment:群白名单;column:allow_list;"` //群白名单
	BlockList *string `json:"block_list" form:"block_list" gorm:"type:text;comment:群黑名单;column:block_list;"` //群黑名单
	Remark    *string `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                //备注
	Status    *int    `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`        //状态
}

// TableName 客户端 Client自定义表名 addon_bi_im_client
func (Client) TableName() string {
	return "addon_bi_im_client"
}
