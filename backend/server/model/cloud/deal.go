// 自动生成模板Deal
package cloud

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"gorm.io/datatypes"
)

// 商机 结构体  Deal
type Deal struct {
	global.GVA_MODEL_ADDON
	MerchantId  *int           `json:"merchant_id" form:"merchant_id" gorm:"default:0;column:merchant_id;comment:商户ID;size:10;"` //商户ID
	MemberId    *int           `json:"member_id" form:"member_id" gorm:"default:0;column:member_id;comment:会员ID;size:10;"`       //会员ID
	Title       *string        `json:"title" form:"title" gorm:"column:title;comment:名称;size:250;" binding:"required"`           //名称
	CustomerId  *int           `json:"customer_id" form:"customer_id" gorm:"default:0;column:customer_id;comment:客户ID;size:10;"` //客户ID
	Amount      *float64       `json:"amount" form:"amount" gorm:"default:0.00;column:amount;comment:金额;size:10;"`               //金额
	Stage       *int           `json:"stage" form:"stage" gorm:"default:1;column:stage;comment:阶段;size:10;"`                     //阶段
	Possibility *int           `json:"possibility" form:"possibility" gorm:"default:0;column:possibility;comment:可能性;size:10;"`  //可能性
	Description *string        `json:"description" form:"description" gorm:"column:description;comment:描述;type:text;"`           //描述
	Records     datatypes.JSON `json:"records" form:"records" gorm:"column:records;comment:记录;type:text;" swaggertype:"object"`  //记录
	Remark      *string        `json:"remark" form:"remark" gorm:"column:remark;comment:备注;type:text;"`                          //备注
	Type        *int           `json:"type" form:"type" gorm:"default:0;column:type;comment:类型;size:1;"`                         //类型
	Source      *int           `json:"source" form:"source" gorm:"default:0;column:source;comment:来源;size:1;"`                   //来源
	LastDate    *time.Time     `json:"last_date" form:"last_date" gorm:"type:date;comment:最近联系日期;column:last_date;"`             //最近联系日期
	NextDate    *time.Time     `json:"next_date" form:"next_date" gorm:"type:date;comment:下次联系日期;column:next_date;"`             //下次联系日期
	CloseDate   *time.Time     `json:"close_date" form:"close_date" gorm:"type:date;comment:成交日期;column:close_date;"`            //成交日期
	Status      *int           `json:"status" form:"status" gorm:"default:1;column:status;comment:状态;size:1;"`                   //状态
}

// TableName 商机 Deal自定义表名 addon_cloud_deal
func (Deal) TableName() string {
	return "addon_cloud_deal"
}

/* func (c *Customer) BeforeSave(tx *gorm.DB) error {
	if c.LastDate == nil {
		tx.Statement.SetColumn("last_date", nil)
	}

	if c.NextDate == nil {
		tx.Statement.SetColumn("next_date", nil)
	}

	return nil
} */
