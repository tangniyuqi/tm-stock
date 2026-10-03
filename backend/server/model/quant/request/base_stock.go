package request

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
)

type BaseStockSearch struct {
	TsCode     *string    `json:"ts_code" form:"ts_code"`
	Symbol     *string    `json:"symbol" form:"symbol"`
	Name       *string    `json:"name" form:"name"`
	Area       *string    `json:"area" form:"area"`
	Industry   *string    `json:"industry" form:"industry"`
	Fullname   *string    `json:"fullname" form:"fullname"`
	Enname     *string    `json:"enname" form:"enname"`
	Cnspell    *string    `json:"cnspell" form:"cnspell"`
	Market     *string    `json:"market" form:"market"`
	Exchange   *string    `json:"exchange" form:"exchange"`
	CurrType   *string    `json:"curr_type" form:"curr_type"`
	ListStatus *string    `json:"list_status" form:"list_status"`
	ListDate   *time.Time `json:"list_date" form:"list_date"`
	DelistDate *time.Time `json:"delist_date" form:"delist_date"`
	IsHs       *string    `json:"is_hs" form:"is_hs"`
	ActName    *string    `json:"act_name" form:"act_name"`
	ActEntType *string    `json:"act_ent_type" form:"act_ent_type"`

	Q *string `json:"q" form:"q"`
	request.PageInfo
}
