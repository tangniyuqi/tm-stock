// 自动生成模板TradeTask
package quant

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/datatypes"
)

// 交易任务 结构体  TradeTask
type TradeTask struct {
	global.GVA_MODEL_ADDON
	MemberId          *uint32         `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;"`                                        //用户ID
	AccountId         *uint32         `json:"account_id" form:"account_id" gorm:"default:0;comment:账户ID;column:account_id;"`                                     //账户ID
	StrategyId        *uint32         `json:"strategy_id" form:"strategy_id" gorm:"default:0;comment:策略ID;column:strategy_id;"`                                  //策略ID
	Name              *string         `json:"name" form:"name" gorm:"comment:名称;column:name;size:100;"`                                                          //名称
	Stock             datatypes.JSON  `json:"stock" form:"stock" gorm:"comment:股票;column:stock;" swaggertype:"object"`                                           //股票
	Config            datatypes.JSON  `json:"config" form:"config" gorm:"comment:配置;column:config;" swaggertype:"object"`                                        //配置
	Positions         datatypes.JSON  `json:"positions" form:"positions" gorm:"comment:持仓;column:positions;" swaggertype:"object"`                                //持仓
	Remark            *string         `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:250;"`                                                    //备注
	Status            *int8           `json:"status" form:"status" gorm:"default:0;comment:状态;column:status;"`
	Member            *system.SysUser `json:"member" gorm:"foreignKey:MemberId"`     //关联用户
	Account           *Account        `json:"account" gorm:"foreignKey:AccountId"`   //关联账户
	Strategy          *Strategy       `json:"strategy" gorm:"foreignKey:StrategyId"` //关联策略
}

// TableName 交易任务 TradeTask自定义表名 addon_quant_trade_task
func (TradeTask) TableName() string {
	return "addon_quant_trade_task"
}
