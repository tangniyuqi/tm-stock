package api

import "github.com/flipped-aurora/gin-vue-admin/server/plugin/fileManager/service"

var (
	Api                = new(api)
	serviceFileManager = service.Service.FileManagerService
)

type api struct {
	FileManagerApi FileManagerApi
}
