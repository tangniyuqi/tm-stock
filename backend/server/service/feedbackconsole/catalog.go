package feedbackconsole

import (
	"context"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureCatalog只登记功能目录，不迁移表、不分配角色/菜单或Casbin权限。
// 仅由已显式开启反馈后台的装配调用；目录碰撞不覆盖，整个事务回滚。
func EnsureCatalog(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return ErrUnconfirmed
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁住已有的最小角色行，串行同库多个实例的目录登记；不改变该角色。
		var anchor system.SysAuthority
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("authority_id").First(&anchor).Error; err != nil {
			return err
		}
		var menus []system.SysBaseMenu
		if err := tx.Unscoped().Where("name = ? OR path = ? OR component = ?", "AiFeedbackConsole", "aiFeedback", "view/quant/aiFeedback/index.vue").Find(&menus).Error; err != nil {
			return err
		}
		if len(menus) == 0 {
			menu := system.SysBaseMenu{Path: "aiFeedback", Name: "AiFeedbackConsole", Component: "view/quant/aiFeedback/index.vue", Sort: 14, Meta: system.Meta{Title: "报告反馈处理", Icon: "chat-line-square"}}
			if err := tx.Omit(clause.Associations).Create(&menu).Error; err != nil {
				return err
			}
		} else if len(menus) != 1 || menus[0].DeletedAt.Valid || menus[0].Name != "AiFeedbackConsole" || menus[0].Path != "aiFeedback" || menus[0].Component != "view/quant/aiFeedback/index.vue" || menus[0].ParentId != 0 || menus[0].Hidden || menus[0].KeepAlive || menus[0].DefaultMenu {
			return ErrConflict
		}
		for _, api := range CatalogAPIs() {
			var found []system.SysApi
			if err := tx.Unscoped().Where("path = ? AND method = ?", api.Path, api.Method).Find(&found).Error; err != nil {
				return err
			}
			if len(found) == 0 {
				if err := tx.Create(&api).Error; err != nil {
					return err
				}
			} else if len(found) != 1 || found[0].DeletedAt.Valid || found[0].ApiGroup != api.ApiGroup {
				return ErrConflict
			}
		}
		return nil
	})
	if err != nil {
		return ErrUnconfirmed
	}
	return nil
}

// CatalogAPIs返回副本，供既有API权限管理明确勾选，不包含通配符或其他AI能力。
func CatalogAPIs() []system.SysApi {
	return []system.SysApi{
		{Path: "/quant/ai-feedback", Method: "GET", ApiGroup: "报告反馈处理", Description: "分页查询报告反馈"},
		{Path: "/quant/ai-feedback/:id", Method: "GET", ApiGroup: "报告反馈处理", Description: "查询反馈详情与处理历史"},
		{Path: "/quant/ai-feedback/:id/decisions", Method: "POST", ApiGroup: "报告反馈处理", Description: "确认反馈处理决定"},
	}
}
