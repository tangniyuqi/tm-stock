package main

import (
	"path/filepath"

	"github.com/flipped-aurora/gin-vue-admin/server/plugin/fileManager/model"
	"gorm.io/gen"
)

func main() {
	g := gen.NewGenerator(gen.Config{OutPath: filepath.Join("..", "..", "..", "fileManager", "model", "dao"), Mode: gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface})
	g.ApplyBasic(new(model.PluginFileRecord), new(model.PluginFileCategory))
	g.Execute()
}
