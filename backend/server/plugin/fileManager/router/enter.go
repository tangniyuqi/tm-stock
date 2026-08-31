package router

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/fileManager/api"

var (
	Router         = new(router)
	apiFileManager = api.Api.FileManagerApi
)

type router struct {
	FileManagerRouter FileManagerRouter
}
