// 自动生成模板BaseStock
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 基础股票 结构体  BaseStock
type BaseStock struct {
	global.GVA_MODEL_ADDON
	TsCode       *string    `json:"ts_code" form:"ts_code" gorm:"comment:TS代码;column:ts_code;size:20;"`                                //TS代码
	Symbol       *string    `json:"symbol" form:"symbol" gorm:"comment:股票代码;column:symbol;size:10;"`                                   //股票代码
	Name         *string    `json:"name" form:"name" gorm:"comment:股票名称;column:name;size:50;"`                                         //股票名称
	Area         *string    `json:"area" form:"area" gorm:"comment:地域;column:area;size:30;"`                                           //地域
	Industry     *string    `json:"industry" form:"industry" gorm:"comment:所属行业;column:industry;size:50;"`                             //所属行业
	Fullname     *string    `json:"fullname" form:"fullname" gorm:"comment:股票全称;column:fullname;size:100;"`                            //股票全称
	Enname       *string    `json:"enname" form:"enname" gorm:"comment:英文全称;column:enname;size:200;"`                                  //英文全称
	Cnspell      *string    `json:"cnspell" form:"cnspell" gorm:"comment:拼音缩写;column:cnspell;size:50;"`                                //拼音缩写
	Market       *string    `json:"market" form:"market" gorm:"comment:市场类型;column:market;size:20;"`                                   //市场类型
	Exchange     *string    `json:"exchange" form:"exchange" gorm:"comment:交易所代码;column:exchange;size:10;"`                            //交易所代码
	CurrType     *string    `json:"curr_type" form:"curr_type" gorm:"comment:交易货币;column:curr_type;size:10;"`                          //交易货币
	ListStatus   *string    `json:"list_status" form:"list_status" gorm:"comment:上市状态;column:list_status;size:10;"`                    //上市状态
	ListDate     *time.Time `json:"list_date" form:"list_date" gorm:"comment:上市日期;column:list_date;"`                                  //上市日期
	DelistDate   *time.Time `json:"delist_date" form:"delist_date" gorm:"comment:退市日期;column:delist_date;"`                            //退市日期
	IsHs         *string    `json:"is_hs" form:"is_hs" gorm:"comment:沪深港通标;column:is_hs;"`                                             //沪深港通
	ActName      *string    `json:"act_name" form:"act_name" gorm:"comment:实控人;column:act_name;size:100;"`                             //实控人
	ActEntType   *string    `json:"act_ent_type" form:"act_ent_type" gorm:"comment:实控人企业性质;column:act_ent_type;size:50;"`              //实控人企业性质
	ChangePct    *float64   `json:"change_pct" form:"change_pct" gorm:"type:decimal(6,2);default:0.00;comment:涨跌幅;column:change_pct;"` //涨跌幅
	Fundamentals *string    `json:"fundamentals" form:"fundamentals" gorm:"comment:基本面;column:fundamentals;type:text;"`                //基本面深度分析
	Financial    *string    `json:"financial" form:"financial" gorm:"comment:财务分析;column:financial;type:text;"`                        //财务分析
	Realization  *string    `json:"realization" form:"realization" gorm:"comment:落地兑现;column:realization;type:text;"`                  //项目落地、业绩兑现分析
	Momentum     *string    `json:"momentum" form:"momentum" gorm:"comment:题材热度;column:momentum;type:text;"`                           //资金热度
	Risk         *string    `json:"risk" form:"risk" gorm:"comment:风险提示;column:risk;type:text;"`                                       //风险提示
	AiAnalyzedAt *time.Time `json:"ai_analyzed_at" form:"ai_analyzed_at" gorm:"comment:AI分析完成时间;column:ai_analyzed_at;"`               //大模型分析完成时间
}

// TableName 基础股票 BaseStock自定义表名 addon_quant_base_stock
func (BaseStock) TableName() string {
	return "addon_quant_base_stock"
}
