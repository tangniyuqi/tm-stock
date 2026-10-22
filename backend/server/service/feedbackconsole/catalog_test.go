package feedbackconsole

import (
	"context"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

func TestFeedbackConsoleCatalog(t *testing.T) {
	for _, mode := range []string{"success", "menu_collision", "api_collision", "missing_table", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			db := testutil.NewMemoryDBWithoutGlobal(t, &system.SysBaseMenu{}, &system.SysApi{})
			zero := uint(0)
			if err := db.Create(&system.SysAuthority{AuthorityId: 888, ParentId: &zero}).Error; err != nil {
				t.Fatal(err)
			}
			if mode == "menu_collision" {
				if err := db.Create(&system.SysBaseMenu{Name: "AiFeedbackConsole", Path: "other"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if mode == "api_collision" {
				if err := db.Create(&system.SysApi{Path: "/quant/ai-feedback/:id", Method: "GET", ApiGroup: "其他功能"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if mode == "missing_table" {
				if err := db.Migrator().DropTable(&system.SysApi{}); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			err := EnsureCatalog(ctx, db)
			if mode != "success" {
				if err == nil {
					t.Fatal("目录碰撞或缺依赖仍登记")
				}
				if mode == "api_collision" {
					var count int64
					if err := db.Model(&system.SysBaseMenu{}).Count(&count).Error; err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						t.Fatal("目录未整体回滚")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := EnsureCatalog(ctx, db); err != nil {
				t.Fatal(err)
			}
			var menus, apis, grants int64
			if err := db.Model(&system.SysBaseMenu{}).Count(&menus).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&system.SysApi{}).Count(&apis).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&system.SysAuthorityMenu{}).Count(&grants).Error; err != nil {
				t.Fatal(err)
			}
			if menus != 1 || apis != 3 || grants != 0 {
				t.Fatal("目录重复或发生隐式菜单授权")
			}
			if db.Migrator().HasTable("casbin_rule") {
				t.Fatal("目录登记不能初始化或授予Casbin权限")
			}
		})
	}
	if EnsureCatalog(context.Background(), nil) == nil {
		t.Fatal("空数据库仍登记")
	}
}
