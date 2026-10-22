//go:build integration

package feedbackconsole

import (
	"context"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestFeedbackConsoleMySQLCatalog(t *testing.T) {
	for _, isolation := range []string{"READ COMMITTED", "REPEATABLE READ"} {
		t.Run(isolation, func(t *testing.T) {
			for _, mode := range []string{"concurrent", "collision_rollback", "soft_deleted", "lock_cancel", "missing_table", "missing_anchor"} {
				t.Run(mode, func(t *testing.T) {
					db := testutil.NewFeedbackMySQLDB(t)
					if err := db.AutoMigrate(&system.SysBaseMenu{}, &system.SysApi{}); err != nil {
						t.Fatal("建立目录测试夹具失败")
					}
					// 会话隔离级别仅在后面的测试专用连接上设置。
					dsnPool, err := db.DB()
					if err != nil {
						t.Fatal(err)
					}
					zero := uint(0)
					role := system.SysAuthority{AuthorityId: 777, ParentId: &zero, AuthorityName: "测试原有角色", DefaultRouter: "about", DataScope: 2}
					if mode != "missing_anchor" {
						if err := db.Omit(clause.Associations).Create(&role).Error; err != nil {
							t.Fatal(err)
						}
					}
					about := system.SysBaseMenu{Path: "about", Name: "about"}
					if err := db.Create(&about).Error; err != nil {
						t.Fatal(err)
					}
					if err := db.Create(&system.SysAuthorityMenu{MenuId: strconv.FormatUint(uint64(about.ID), 10), AuthorityId: "777"}).Error; err != nil {
						t.Fatal(err)
					}
					var before system.SysAuthority
					if mode != "missing_anchor" {
						if err := db.First(&before, "authority_id = ?", 777).Error; err != nil {
							t.Fatal(err)
						}
					}
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					ensure := func(ctx context.Context) error {
						return db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
							query := "SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED"
							if isolation == "REPEATABLE READ" {
								query = "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ"
							}
							if err := conn.Exec(query).Error; err != nil {
								return err
							}
							return EnsureCatalog(ctx, conn)
						})
					}
					count := func(model any, want int64) {
						t.Helper()
						var got int64
						if err := db.Unscoped().Model(model).Count(&got).Error; err != nil {
							t.Fatal(err)
						}
						if got != want {
							t.Fatalf("目录或授权行数错误：实际%d，期望%d", got, want)
						}
					}
					switch mode {
					case "concurrent":
						// 24个调用竞争12个真实连接，不依赖进程内互斥；每个调用独占连接。
						start, results := make(chan struct{}), make(chan error, 24)
						for i := 0; i < 24; i++ {
							go func() { <-start; results <- ensure(ctx) }()
						}
						close(start)
						failed := false
						for i := 0; i < 24; i++ {
							if err := <-results; err != nil {
								failed = true
							}
						}
						if failed {
							t.Fatal("并发目录登记失败")
						}
						if err := ensure(ctx); err != nil {
							t.Fatal("重复目录登记失败")
						}
						count(&system.SysBaseMenu{}, 2)
						count(&system.SysApi{}, 3)
						if dsnPool.Stats().OpenConnections < 2 {
							t.Fatal("未建立多个真实MySQL连接")
						}
					case "collision_rollback":
						collision := CatalogAPIs()[2]
						collision.ApiGroup = "测试已有其它目录"
						if err := db.Create(&collision).Error; err != nil {
							t.Fatal(err)
						}
						// 用数据库写入后的快照比较，避免纳秒时间与MySQL列精度差异误报。
						var original system.SysApi
						if err := db.First(&original, collision.ID).Error; err != nil {
							t.Fatal(err)
						}
						if ensure(ctx) != ErrUnconfirmed {
							t.Fatal("目录冲突未拒绝")
						}
						count(&system.SysBaseMenu{}, 1)
						count(&system.SysApi{}, 1)
						var unchanged system.SysApi
						if err := db.First(&unchanged, collision.ID).Error; err != nil || !reflect.DeepEqual(unchanged, original) {
							t.Fatal("冲突API被覆盖")
						}
					case "soft_deleted":
						menu := system.SysBaseMenu{Name: "AiFeedbackConsole", Path: "aiFeedback", Component: "view/quant/aiFeedback/index.vue"}
						if err := db.Create(&menu).Error; err != nil {
							t.Fatal(err)
						}
						if err := db.Delete(&menu).Error; err != nil {
							t.Fatal(err)
						}
						if ensure(ctx) != ErrUnconfirmed {
							t.Fatal("软删除目录被复活")
						}
						count(&system.SysBaseMenu{}, 2)
						count(&system.SysApi{}, 0)
					case "lock_cancel":
						tx := db.WithContext(ctx).Begin()
						if tx.Error != nil {
							t.Fatal(tx.Error)
						}
						defer func() {
							if err := tx.Rollback().Error; err != nil {
								t.Error("释放测试角色锁失败")
							}
						}()
						var locked system.SysAuthority
						if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked).Error; err != nil {
							t.Fatal(err)
						}
						short, stop := context.WithTimeout(ctx, 150*time.Millisecond)
						defer stop()
						if ensure(short) != ErrUnconfirmed || short.Err() == nil {
							t.Fatal("角色锁等待未按调用期限拒绝")
						}
						count(&system.SysBaseMenu{}, 1)
						count(&system.SysApi{}, 0)
					case "missing_table":
						if err := db.Migrator().DropTable(&system.SysApi{}); err != nil {
							t.Fatal(err)
						}
						if ensure(ctx) != ErrUnconfirmed || db.Migrator().HasTable(&system.SysApi{}) {
							t.Fatal("缺表时未拒绝或发生自动DDL")
						}
						count(&system.SysBaseMenu{}, 1)
					case "missing_anchor":
						if ensure(ctx) != ErrUnconfirmed {
							t.Fatal("无角色锚点仍登记")
						}
						count(&system.SysBaseMenu{}, 1)
						count(&system.SysApi{}, 0)
					}
					count(&system.SysAuthorityMenu{}, 1)
					if mode != "missing_anchor" {
						var after system.SysAuthority
						if err := db.First(&after, "authority_id = ?", 777).Error; err != nil || !reflect.DeepEqual(before, after) {
							t.Fatal("目录登记改变原角色")
						}
					}
					if db.Migrator().HasTable("casbin_rule") {
						t.Fatal("目录登记隐式初始化权限")
					}
				})
			}
		})
	}
}
