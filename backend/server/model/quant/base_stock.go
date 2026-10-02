// 自动生成模板BaseStock
package quant

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// 基础股票 结构体  BaseStock
type BaseStock struct {
	global.GVA_MODEL_ADDON
	TsCode     *string    `json:"ts_code" form:"ts_code" gorm:"comment:TS代码;column:ts_code;size:20;"`                                //TS代码
	Symbol     *string    `json:"symbol" form:"symbol" gorm:"comment:股票代码;column:symbol;size:10;"`                                   //股票代码
	Name       *string    `json:"name" form:"name" gorm:"comment:股票名称;column:name;size:50;"`                                         //股票名称
	Area       *string    `json:"area" form:"area" gorm:"comment:地域;column:area;size:30;"`                                           //地域
	Industry   *string    `json:"industry" form:"industry" gorm:"comment:所属行业;column:industry;size:50;"`                             //所属行业
	Fullname   *string    `json:"fullname" form:"fullname" gorm:"comment:股票全称;column:fullname;size:100;"`                            //股票全称
	Enname     *string    `json:"enname" form:"enname" gorm:"comment:英文全称;column:enname;size:200;"`                                  //英文全称
	Cnspell    *string    `json:"cnspell" form:"cnspell" gorm:"comment:拼音缩写;column:cnspell;size:50;"`                                //拼音缩写
	Market     *string    `json:"market" form:"market" gorm:"comment:市场类型;column:market;size:20;"`                                   //市场类型
	Exchange   *string    `json:"exchange" form:"exchange" gorm:"comment:交易所代码;column:exchange;size:10;"`                            //交易所代码
	CurrType   *string    `json:"curr_type" form:"curr_type" gorm:"comment:交易货币;column:curr_type;size:10;"`                          //交易货币
	ListStatus *string    `json:"list_status" form:"list_status" gorm:"comment:上市状态;column:list_status;size:10;"`                    //上市状态
	ListDate   *time.Time `json:"list_date" form:"list_date" gorm:"comment:上市日期;column:list_date;"`                                  //上市日期
	DelistDate *time.Time `json:"delist_date" form:"delist_date" gorm:"comment:退市日期;column:delist_date;"`                            //退市日期
	IsHs       *string    `json:"is_hs" form:"is_hs" gorm:"comment:沪深港通标;column:is_hs;"`                                             //沪深港通
	ActName    *string    `json:"act_name" form:"act_name" gorm:"comment:实控人;column:act_name;size:100;"`                             //实控人
	ActEntType *string    `json:"act_ent_type" form:"act_ent_type" gorm:"comment:实控人企业性质;column:act_ent_type;size:50;"`              //实控人企业性质
	ChangePct  *float64   `json:"change_pct" form:"change_pct" gorm:"type:decimal(6,2);default:0.00;comment:涨跌幅;column:change_pct;"` //涨跌幅
	// 以下六列是 AI 分析的遗留数据（AI 以分析师口吻写的评价性文字）。该功能已下线（docs/specs/ai-analysis 的 F15、决策 D16）：
	// 历史值保留在库里，但不再生成，也不再从任何接口输出或接收——json:"-"、form:"-" 保证无论哪个接口返回或绑定
	// BaseStock（含免鉴权的开放接口）都带不出、也写不进这些列。要恢复必须先经律师确认并加上红线终检。
	Fundamentals *string    `json:"-" form:"-" gorm:"comment:基本面;column:fundamentals;type:text;"` //遗留：基本面分析（已停用）
	Financial    *string    `json:"-" form:"-" gorm:"comment:财务分析;column:financial;type:text;"`   //遗留：财务分析（已停用）
	Realization  *string    `json:"-" form:"-" gorm:"comment:落地兑现;column:realization;type:text;"` //遗留：业绩兑现分析（已停用）
	Momentum     *string    `json:"-" form:"-" gorm:"comment:题材热度;column:momentum;type:text;"`    //遗留：热度分析（已停用）
	Risk         *string    `json:"-" form:"-" gorm:"comment:风险提示;column:risk;type:text;"`        //遗留：风险提示（已停用）
	AiAnalyzedAt *time.Time `json:"-" form:"-" gorm:"comment:AI分析完成时间;column:ai_analyzed_at;"`    //遗留：分析完成时间（已停用）
}

// TableName 基础股票 BaseStock自定义表名 addon_quant_base_stock
func (BaseStock) TableName() string {
	return "addon_quant_base_stock"
}
