package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"time"
)

type WeishiSearch struct {
	StartCreatedAt    *time.Time `json:"startCreatedAt" form:"startCreatedAt"`
	EndCreatedAt      *time.Time `json:"endCreatedAt" form:"endCreatedAt"`
	Account           *string    `json:"account" form:"account"`
	IAuthType         *int       `json:"iAuthType" form:"iAuthType"`
	Main_login        *string    `json:"main_login" form:"main_login"`
	Openid            *string    `json:"openid" form:"openid"`
	Person_id         *string    `json:"person_id" form:"person_id"`
	StartCancelled_at *int       `json:"startCancelled_at" form:"startCancelled_at"`
	EndCancelled_at   *int       `json:"endCancelled_at" form:"endCancelled_at"`
	Status            *int       `json:"status" form:"status" `
	request.PageInfo
	Order string `json:"order"`
	Desc  string `json:"desc"`
}
