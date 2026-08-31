// 自动生成模板Account
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/datatypes"
)

// 账户 结构体  Account
type Account struct {
	global.GVA_MODEL_ADDON
	MemberId         *uint32        `json:"member_id" form:"member_id" gorm:"default:0;comment:用户ID;column:member_id;size:10;"`                                       //用户ID
	Member           *system.SysUser `json:"member" gorm:"foreignKey:MemberId;references:ID"`                                                                          //关联用户
	Type             *uint8         `json:"type" form:"type" gorm:"default:1;comment:类型;column:type;size:1;"`                                                         //类型
	Broker         *string        `json:"broker" form:"broker" gorm:"comment:开户券商;column:broker;size:20;"`                                                    //开户券商
	Name             *string        `json:"name" form:"name" gorm:"comment:开户名;column:name;size:20;"`                                                                 //开户名
	AccountNo        *string        `json:"account_no" form:"account_no" gorm:"comment:资金账号;column:account_no;size:50;"`                                              //资金账号
	Passcode         *string        `json:"passcode" form:"passcode" gorm:"comment:交易密码;column:passcode;size:6;"`                                                     //交易密码
	Summary          datatypes.JSON `json:"summary" form:"summary" gorm:"comment:账户汇总;column:summary;" swaggertype:"object"`                                          //账户汇总
	Markets          datatypes.JSON `json:"markets" form:"markets" gorm:"comment:交易板块;column:markets;type:text;"`                                                     //交易板块
	Server           datatypes.JSON `json:"server" form:"server" gorm:"comment:服务器;column:server;" swaggertype:"object"`                                              //服务器
	Amount           *float64       `json:"amount" form:"amount" gorm:"default:0.00;comment:资金;column:amount;size:10;"`                                               //资金
	TotalAsset       *float64       `json:"total_asset" form:"total_asset" gorm:"type:decimal(18,2);default:0.00;comment:总资产;column:total_asset;"`                    //总资产
	MarketValue      *float64       `json:"market_value" form:"market_value" gorm:"type:decimal(18,2);default:0.00;comment:总市值;column:market_value;"`                 //总市值
	AvailableCash    *float64       `json:"available_balance" form:"available_balance" gorm:"type:decimal(18,2);default:0.00;comment:可用金额;column:available_balance;"` //可用金额
	WithdrawableCash *float64       `json:"withdrawable_cash" form:"withdrawable_cash" gorm:"type:decimal(18,2);default:0.00;comment:可取金额;column:withdrawable_cash;"` //可取金额
	TotalPnL         *float64       `json:"total_pnl" form:"total_pnl" gorm:"type:decimal(18,2);default:0.00;comment:总盈亏;column:total_pnl;"`                          //总盈亏
	DailyPnL         *float64       `json:"daily_pnl" form:"daily_pnl" gorm:"type:decimal(18,2);default:0.00;comment:当日盈亏;column:daily_pnl;"`                         //当日盈亏
	MaxTask          *uint32        `json:"max_task" form:"max_task" gorm:"default:100;comment:最大任务数;column:max_task;size:10;"`                                       //最大任务数量
	MaxRunningTask   *uint32        `json:"max_running_task" form:"max_running_task" gorm:"default:1;comment:最大运行任务数;column:max_running_task;size:10;"`               //最大同时运行任务数量
	RunningTaskCount int64          `json:"running_task_count" gorm:"-"`
	ExpirationDate   *time.Time     `json:"expiration_date" form:"expiration_date" gorm:"type:date;default:(curdate());comment:到期日期;column:expiration_date;"` //到期日期
	Remark           *string        `json:"remark" form:"remark" gorm:"comment:备注;column:remark;size:200;"`                                                   //备注
	Status           *int8          `json:"status" form:"status" gorm:"default:1;comment:状态;column:status;size:1;"`                                           //状态
}

// TableName 账户 Account自定义表名 addon_quant_account
func (Account) TableName() string {
	return "addon_quant_account"
}
