// 自动生成模板Account
package cloud

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 财务 结构体  Account
type Account struct {
	global.GVA_MODEL_ADDON
	MerchantId   *int       `json:"merchant_id" form:"merchant_id" gorm:"default:0;comment:商户ID;column:merchant_id;size:10;"`   //商户ID
	MemberId     *int       `json:"member_id" form:"member_id" gorm:"default:0;comment:会员ID;column:member_id;size:10;"`         //会员ID
	Title        *string    `json:"title" form:"title" gorm:"comment:名称;column:title;size:250;" binding:"required"`             //名称
	CustomerId   *int       `json:"customer_id" form:"customer_id" gorm:"default:0;comment:客户ID;column:customer_id;size:10;"`   //客户ID
	Type         *int       `json:"type" form:"type" gorm:"default:0;comment:类型;column:type;size:3;"`                           //类型
	AmountIn     *float64   `json:"amount_in" form:"amount_in" gorm:"default:0.00;comment:收入;column:amount_in;size:10;"`        //收入
	AmountOut    *float64   `json:"amount_out" form:"amount_out" gorm:"default:0.00;comment:支出;column:amount_out;size:10;"`     //支出
	AccountType  *int       `json:"account_type" form:"account_type" gorm:"default:0;comment:账户类型;column:account_type;size:3;"` //账户类型
	PayMode      *int       `json:"pay_mode" form:"pay_mode" gorm:"default:0;comment:支付方式;column:pay_mode;size:3;"`             //支付方式
	Payer        *string    `json:"payer" form:"payer" gorm:"comment:付款人;column:payer;size:250;"`                               //付款人
	Transactor   *string    `json:"transactor" form:"transactor" gorm:"comment:经办人;column:transactor;size:250;"`                //经办人
	HandlingDate *time.Time `json:"handling_date" form:"handling_date" gorm:"type:date;comment:经办日期;column:handling_date;"`     //经办日期
	ProofType    *int       `json:"proof_type" form:"proof_type" gorm:"default:0;comment:凭证类型;column:proof_type;size:3;"`       //凭证类型
	ProofNumber  *string    `json:"proof_number" form:"proof_number" gorm:"comment:凭证编号;column:proof_number;size:250;"`         //凭证编号
	Remark       *string    `json:"remark" form:"remark" gorm:"comment:备注;column:remark;type:text;"`                            //备注
	Balance      *bool      `json:"balance" form:"balance" gorm:"default:0;comment:结平;column:balance;"`                         //结平
	Review       *bool      `json:"review" form:"review" gorm:"default:0;comment:审核;column:review;"`                            //审核
	Status       *int       `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`                     //状态
}

// TableName 财务 Account自定义表名 addon_cloud_account
func (Account) TableName() string {
	return "addon_cloud_account"
}

/* func (c *Customer) BeforeSave(tx *gorm.DB) error {
	if c.HandlingDate == nil {
		tx.Statement.SetColumn("handling_date", nil)
	}

	return nil
} */
