// 自动生成模板Weishi
package addon

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 微视账号 结构体  Weishi
type Weishi struct {
	global.GVA_MODEL_ADDON
	Account      *string    `json:"account" form:"account" gorm:"column:account;comment:账号;size:100;" binding:"required"`                 //账号
	Password     *string    `json:"password" form:"password" gorm:"column:password;comment:密码;size:100;" binding:"required"`              //密码
	IAuthType    *int       `json:"iAuthType" form:"iAuthType" gorm:"column:iAuthType;comment:授权类型;size:1;" binding:"required"`           //授权类型
	Main_login   *string    `json:"main_login" form:"main_login" gorm:"column:main_login;comment:登录类型;size:10;" binding:"required"`       //登录类型
	Openid       *string    `json:"openid" form:"openid" gorm:"column:openid;comment:OPENID;size:100;" binding:"required"`                //OPENID
	SSessionKey  *string    `json:"sSessionKey" form:"sSessionKey" gorm:"column:s_session_key;comment:会话密钥;size:100;" binding:"required"` //会话密钥
	Person_id    *string    `json:"person_id" form:"person_id" gorm:"column:person_id;comment:用户ID;size:100;" binding:"required"`         //用户ID
	Nickname     *string    `json:"nickname" form:"nickname" gorm:"column:nickname;comment:昵称;size:50;"`                                  //昵称
	Cancelled_at *time.Time `json:"cancelled_at" form:"cancelled_at" gorm:"column:cancelled_at;comment:注销时间;size:3;"`                     //注销时间
	Status       *int       `json:"status" form:"status" gorm:"default:0;column:status;comment:状态;size:1;"`                               //状态
}

type WeishiContent struct {
	Content string `json:"content" form:"content" binding:"required"` // Content 字段
}

// TableName 微视账号 Weishi自定义表名 addon_weishi
func (Weishi) TableName() string {
	return "addon_weishi"
}
