// 自动生成模板Config
package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 配置 结构体  Config
type Config struct {
	global.GVA_MODEL_ADDON
	MemberId    *int    `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;size:10;"`              //用户ID
	Type        *int    `json:"type" form:"type" gorm:"default:0;comment:类型;column:type;size:1;"`                                //类型
	Name        *string `json:"name" form:"name" gorm:"comment:名称;column:name;size:250;"`                                        //名称
	Host        *string `json:"host" form:"host" gorm:"comment:主机;column:host;size:50;"`                                         //主机
	Port        *string `json:"port" form:"port" gorm:"comment:端口;column:port;size:10;"`                                         //端口
	Key         *string `json:"key" form:"key" gorm:"comment:密钥;column:key;size:250;"`                                           //密钥
	Client      *string `json:"client" form:"client" gorm:"comment:客户端;column:client;size:50;"`                                  //客户端
	WebhookType *int    `json:"webhook_type" form:"webhook_type" gorm:"default:0;comment:Webhook类型;column:webhook_type;size:1;"` //Webhook类型
	WebhookUrl  *string `json:"webhook_url" form:"webhook_url" gorm:"comment:Webhook地址;column:webhook_url;size:250;"`            //Webhook地址
	DataSource  *int    `json:"data_source" form:"data_source" gorm:"default:0;comment:数据源;column:data_source;"`                 //数据源
	DataToken   *string `json:"data_token" form:"data_token" gorm:"comment:数据TOKEN;column:data_token;size:250;"`                 //数据TOKEN
	Remark      *string `json:"remark" form:"remark" gorm:"comment:备注;column:remark;"`                                           //备注
	Status      *int    `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`                          //状态
}

// TableName 配置 Config自定义表名 addon_quant_config
func (Config) TableName() string {
	return "addon_quant_config"
}
