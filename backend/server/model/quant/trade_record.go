// 自动生成模板TradeRecord
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 交易记录 结构体  TradeRecord
type TradeRecord struct {
	global.GVA_MODEL_ADDON
	MemberId   *uint32       `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;size:10;"`       //用户ID
	AccountId  *uint32       `json:"account_id" form:"account_id" gorm:"default:0;comment:账户ID;column:account_id;size:10;"`    //账户ID
	TaskId     *uint32       `json:"task_id" form:"task_id" gorm:"default:0;comment:任务ID;column:task_id;size:10;"`                //任务ID
	Name       *string    `json:"name" form:"name" gorm:"comment:名称;column:name;size:50;"`                          //名称
	Symbol     *string    `json:"symbol" form:"symbol" gorm:"comment:标识;column:symbol;size:50;"`
	Price      *float64   `json:"price" form:"price" gorm:"type:decimal(18,2);default:0.00;comment:价格;column:price;"`    //价格
	Quantity   *float64   `json:"quantity" form:"quantity" gorm:"default:0;comment:数量;column:quantity;"` //数量
	Amount     *float64   `json:"amount" form:"amount" gorm:"type:decimal(18,2);default:0.00;comment:金额;column:amount;"` //金额
	Action     *string    `json:"action" form:"action" gorm:"comment:动作;column:action;size:10;"`                   //动作
	Reason     *string    `json:"reason" form:"reason" gorm:"comment:原因;column:reason;size:20;"`                                  //原因
	TradedAt   *time.Time `json:"traded_at" form:"traded_at" gorm:"comment:交易时间;column:traded_at;"`                         //交易时间
	TradeTask  *TradeTask       `json:"trade_task" gorm:"foreignKey:TaskId"` //关联交易任务
}

// TableName 交易记录 TradeRecord自定义表名 addon_quant_trade_record
func (TradeRecord) TableName() string {
	return "addon_quant_trade_record"
}
