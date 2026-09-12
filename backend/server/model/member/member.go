// C 端（题材宝典）用户 Member
package member

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 用户 结构体  Member（用户数据存储）
type Member struct {
	global.GVA_MODEL_ADDON
	Name                 string     `json:"name" form:"name" gorm:"comment:用户名;column:name;size:50;"`                               //用户名
	Nickname             string     `json:"nickname" form:"nickname" gorm:"comment:昵称;column:nickname;size:50;"`                    //昵称
	Mobile               string     `json:"mobile" form:"mobile" gorm:"uniqueIndex;comment:手机号;column:mobile;size:20;"`             //手机号（登录凭证）
	Password             string     `json:"password" form:"password" gorm:"comment:密码;column:password;size:250;"`                   //密码哈希，未设置密码时为空
	AuthKey              string     `json:"-" form:"-" gorm:"comment:持久登录凭证;column:auth_key;size:64;"`                              //持久登录凭证
	AuthKeyExpires       *time.Time `json:"-" form:"-" gorm:"comment:持久登录凭证过期时间;column:auth_key_expires;"`                          //持久登录凭证过期时间
	PasswordResetToken   string     `json:"-" form:"-" gorm:"comment:密码重置凭证(哈希);column:password_reset_token;size:64;"`              //密码重置凭证（存哈希）
	PasswordResetExpires *time.Time `json:"-" form:"-" gorm:"comment:密码重置凭证过期时间;column:password_reset_expires;"`                    //密码重置凭证过期时间
	MobileResetToken     string     `json:"-" form:"-" gorm:"comment:手机号重置凭证(哈希);column:mobile_reset_token;size:64;"`               //手机号重置凭证（存哈希）
	MobileResetExpires   *time.Time `json:"-" form:"-" gorm:"comment:手机号重置凭证过期时间;column:mobile_reset_expires;"`                     //手机号重置凭证过期时间
	Avatar               string     `json:"avatar" form:"avatar" gorm:"comment:头像;column:avatar;size:250;"`                         //头像 URL
	Gender               int8       `json:"gender" form:"gender" gorm:"default:0;comment:性别;column:gender;"`                        //性别
	Birthday             *time.Time `json:"birthday" form:"birthday" gorm:"comment:生日;column:birthday;"`                            //生日
	Email                string     `json:"email" form:"email" gorm:"comment:电子邮箱;column:email;size:100;"`                          //电子邮箱
	Wechat               string     `json:"wechat" form:"wechat" gorm:"comment:微信;column:wechat;size:50;"`                          //微信
	QQ                   string     `json:"qq" form:"qq" gorm:"comment:QQ;column:qq;size:50;"`                                      //QQ
	Bio                  string     `json:"bio" form:"bio" gorm:"comment:个人简介;column:bio;size:250;"`                                //个人简介
	Cover                string     `json:"cover" form:"cover" gorm:"comment:背景图;column:cover;size:250;"`                           //背景图
	ProvinceId           int32      `json:"province_id" form:"province_id" gorm:"default:0;comment:省份ID;column:province_id;"`       //省份ID
	CityId               int32      `json:"city_id" form:"city_id" gorm:"default:0;comment:城市ID;column:city_id;"`                   //城市ID
	AreaId               int32      `json:"area_id" form:"area_id" gorm:"default:0;comment:区县ID;column:area_id;"`                   //区县ID
	Address              string     `json:"address" form:"address" gorm:"comment:详细地址;column:address;size:250;"`                    //详细地址
	Level                int8       `json:"level" form:"level" gorm:"default:0;comment:等级;column:level;"`                           //等级
	ExpireAt             *time.Time `json:"expire_at" form:"expire_at" gorm:"comment:到期时间;column:expire_at;"`                       //到期时间
	VisitCount           int32      `json:"visit_count" form:"visit_count" gorm:"default:0;comment:访问次数;column:visit_count;"`       //访问次数
	InviteCode           string     `json:"invite_code" form:"invite_code" gorm:"comment:邀请码;column:invite_code;size:32;"`          //邀请码
	ParentId             uint       `json:"parent_id" form:"parent_id" gorm:"default:0;comment:上级ID(邀请人);column:parent_id;"`        //上级ID
	Realname             string     `json:"realname" form:"realname" gorm:"comment:真实姓名;column:realname;size:50;"`                  //真实姓名
	IdCard               string     `json:"id_card" form:"id_card" gorm:"comment:证件号;column:id_card;size:50;"`                      //实名认证证件号
	LastLoginAt          *time.Time `json:"last_login_at" form:"last_login_at" gorm:"comment:最近登录时间;column:last_login_at;"`         //最近登录时间
	LastLoginIp          string     `json:"last_login_ip" form:"last_login_ip" gorm:"comment:最近登录IP;column:last_login_ip;size:50;"` //最近登录IP
	Status               int8       `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;"`                        //状态：1正常 / 0禁用
}

// TableName 用户 Member自定义表名 addon_member
func (Member) TableName() string {
	return "addon_member"
}
