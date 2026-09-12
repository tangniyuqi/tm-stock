package member

import "github.com/flipped-aurora/gin-vue-admin/server/global"

// MemberAsset 会员资产（对应 addon_member_asset 表）
// 资产与会员是 1:1 关联（member_id），注册会员时初始化一条记录。
// 资产表由独立建表语句维护，不注册到 AutoMigrate。
type MemberAsset struct {
	global.GVA_MODEL_ADDON
	MemberId         uint    `json:"member_id" form:"member_id" gorm:"comment:用户ID;column:member_id;index"`
	Money            float64 `json:"money" form:"money" gorm:"comment:当前可用余额;column:money;type:decimal(12,2);default:0"`
	FrozenMoney      float64 `json:"frozen_money" form:"frozen_money" gorm:"comment:冻结余额;column:frozen_money;type:decimal(12,2);default:0"`
	AccumulateMoney  float64 `json:"accumulate_money" form:"accumulate_money" gorm:"comment:累计获得余额;column:accumulate_money;type:decimal(12,2);default:0"`
	GiftMoney        float64 `json:"gift_money" form:"gift_money" gorm:"comment:累计赠送余额;column:gift_money;type:decimal(12,2);default:0"`
	ConsumeMoney     float64 `json:"consume_money" form:"consume_money" gorm:"comment:累计消费余额;column:consume_money;type:decimal(12,2);default:0"`
	WithdrawnMoney   float64 `json:"withdrawn_money" form:"withdrawn_money" gorm:"comment:累计提现金额;column:withdrawn_money;type:decimal(12,2);default:0"`
	SavedMoney       float64 `json:"saved_money" form:"saved_money" gorm:"comment:累计已节约金额;column:saved_money;type:decimal(12,2);default:0"`
	Credit           int     `json:"credit" form:"credit" gorm:"comment:当前可用积分;column:credit;default:0"`
	FrozenCredit     int     `json:"frozen_credit" form:"frozen_credit" gorm:"comment:冻结积分;column:frozen_credit;default:0"`
	AccumulateCredit int     `json:"accumulate_credit" form:"accumulate_credit" gorm:"comment:累计获得积分;column:accumulate_credit;default:0"`
	GiftCredit       int     `json:"gift_credit" form:"gift_credit" gorm:"comment:累计赠送积分;column:gift_credit;default:0"`
	ConsumeCredit    int     `json:"consume_credit" form:"consume_credit" gorm:"comment:累计消费积分;column:consume_credit;default:0"`
	Coin             int     `json:"coin" form:"coin" gorm:"comment:当前可用虚拟币;column:coin;default:0"`
	FrozenCoin       int     `json:"frozen_coin" form:"frozen_coin" gorm:"comment:冻结虚拟币;column:frozen_coin;default:0"`
	AccumulateCoin   int     `json:"accumulate_coin" form:"accumulate_coin" gorm:"comment:累计获得虚拟币;column:accumulate_coin;default:0"`
	ConsumeCoin      int     `json:"consume_coin" form:"consume_coin" gorm:"comment:累计消费虚拟币;column:consume_coin;default:0"`
	Growth           int     `json:"growth" form:"growth" gorm:"comment:当前成长值;column:growth;default:0"`
	FrozenGrowth     int     `json:"frozen_growth" form:"frozen_growth" gorm:"comment:冻结成长值;column:frozen_growth;default:0"`
	AccumulateGrowth int     `json:"accumulate_growth" form:"accumulate_growth" gorm:"comment:累计成长值;column:accumulate_growth;default:0"`
	ConsumeGrowth    int     `json:"consume_growth" form:"consume_growth" gorm:"comment:累计扣减/消费成长值;column:consume_growth;default:0"`
	Exp              int     `json:"exp" form:"exp" gorm:"comment:当前经验值;column:exp;default:0"`
	AccumulateExp    int     `json:"accumulate_exp" form:"accumulate_exp" gorm:"comment:累计经验值;column:accumulate_exp;default:0"`
	Status           int8    `json:"status" form:"status" gorm:"comment:状态;column:status;default:1"`
}

// TableName 会员资产表名 addon_member_asset
func (MemberAsset) TableName() string {
	return "addon_member_asset"
}
