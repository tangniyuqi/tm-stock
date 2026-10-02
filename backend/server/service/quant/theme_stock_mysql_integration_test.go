//go:build integration

package quant

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 真连 MySQL 的集成测试：题材与股票两张表用 GVA 自己的 GORM 模型建出来（真实的 GVA 形态），
// 题材股票表用 tm-stock 的 SQL 迁移建出来（权威形态，带依据 NOT NULL、CHECK、唯一键）。
// sqlite 单测覆盖逻辑；这里验证方言相关的部分——联表、LIKE 转义、NULL 状态比较，以及库约束的错误翻译。
//
// 跑法（DSN 必须带 multiStatements=true；会清掉并重建 addon_quant_theme、addon_quant_base_stock、addon_quant_theme_stock）：
//
//	cd backend/server
//	TM_TEST_DSN='root:***@tcp(127.0.0.1:3306)/tm_stock_it?parseTime=true&multiStatements=true&charset=utf8mb4&loc=Local' \
//	  go test -tags=integration ./service/quant/ -run MySQL -count=1 -v
func setupMySQLThemeStock(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TM_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 TM_TEST_DSN，跳过 MySQL 集成测试")
	}
	testutil.InitNopLogger()
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: dsn, DefaultStringSize: 191}), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	drop := func() {
		for _, tbl := range []string{"addon_quant_theme_stock", "addon_quant_theme", "addon_quant_base_stock"} {
			if err := db.Exec("DROP TABLE IF EXISTS " + tbl).Error; err != nil {
				t.Fatalf("清理 %s 失败: %v", tbl, err)
			}
		}
	}
	drop()
	t.Cleanup(drop)

	// GVA 形态的题材与股票表（来自模型）；注意这里刻意不 AutoMigrate ThemeStock——它由 SQL 迁移管理
	if err := db.AutoMigrate(&quant.Theme{}, &quant.BaseStock{}); err != nil {
		t.Fatalf("建题材与股票表失败: %v", err)
	}
	for _, f := range []string{"20260730_addon_quant_theme_stock.sql", "20261002_converge_addon_quant_theme_stock.sql"} {
		b, err := os.ReadFile("../../../../server/migrations/" + f)
		if err != nil {
			t.Fatalf("读迁移失败: %v", err)
		}
		if err := db.Exec(string(b)).Error; err != nil {
			t.Fatalf("执行迁移 %s 失败（DSN 是否带 multiStatements=true？）: %v", f, err)
		}
	}
	old := global.GVA_DB
	global.GVA_DB = db
	t.Cleanup(func() { global.GVA_DB = old })
	return db
}

func TestMySQLListPublicThemeStocksOnlyVisible(t *testing.T) {
	runPublicVisibilityMatrix(t, setupMySQLThemeStock(t))
}

func TestMySQLCreateUpdateFlow(t *testing.T) {
	db := setupMySQLThemeStock(t)
	seedTheme(t, db, 100010, "光源")
	seedTheme(t, db, 100011, "物镜")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	svc := &ThemeStockService{}
	ctx := context.Background()

	req := newReq()
	if err := svc.CreateThemeStock(ctx, &req); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	var got quant.ThemeStock
	if err := db.First(&got, req.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.TsCode != "688502.SH" || got.AuditStatus != quant.ThemeStockAuditDraft || got.Status == nil || *got.Status != 1 {
		t.Fatalf("创建结果不对: %+v", got)
	}

	// 同题材同股票再建一条：被唯一键拒绝，错误翻译成操作员看得懂的话
	dup := newReq()
	err := svc.CreateThemeStock(ctx, &dup)
	if err == nil || !IsThemeStockEvidenceError(err) || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("重复关联应被唯一键拒绝并翻译：%v", err)
	}

	// 审核通过 → C 端可见、题材可见数量 +1
	got.AuditStatus = quant.ThemeStockAuditPassed
	got.UpdatedBy = 7
	if _, err := svc.UpdateThemeStock(ctx, got); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	pub, err := svc.ListPublicThemeStocks(ctx, 100010, 0)
	if err != nil || len(pub) != 1 || pub[0].Stock == nil || pub[0].Theme == nil {
		t.Fatalf("审核通过后 C 端应可见 1 条：%v %+v", err, pub)
	}
	if n := themeStockCount(t, db, 100010); n != 1 {
		t.Fatalf("可见数量应为 1，实际 %d", n)
	}

	// 改依据：打回草稿，C 端立刻不可见
	var cur quant.ThemeStock
	db.First(&cur, req.ID)
	cur.SourceExcerpt = "公司光源模组业务收入占当期营业收入的比例为35.0%。"
	cur.AuditStatus = quant.ThemeStockAuditPassed
	cur.UpdatedBy = 8
	reset, err := svc.UpdateThemeStock(ctx, cur)
	if err != nil || !reset {
		t.Fatalf("改依据应打回草稿：reset=%v err=%v", reset, err)
	}
	if pub, _ := svc.ListPublicThemeStocks(ctx, 100010, 0); len(pub) != 0 {
		t.Fatalf("被打回草稿后 C 端不应再可见，实际 %d 条", len(pub))
	}
	if n := themeStockCount(t, db, 100010); n != 0 {
		t.Fatalf("可见数量应归 0，实际 %d", n)
	}

	// 软删后可以重新添加同一对（生成列 alive 的价值）
	if err := svc.DeleteThemeStock(ctx, strconv.FormatUint(uint64(req.ID), 10), 7); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	again := newReq()
	if err := svc.CreateThemeStock(ctx, &again); err != nil {
		t.Fatalf("软删后应可重新添加同一对：%v", err)
	}
}

// 即使有人绕过服务层、直接写库，库里的依据约束也必须拦得住（这是合规命门的最后一道）。
func TestMySQLDatabaseRejectsEvidencelessRows(t *testing.T) {
	db := setupMySQLThemeStock(t)
	seedTheme(t, db, 100010, "光源")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	for name, mut := range map[string]func(*quant.ThemeStock){
		"摘录为空串":   func(ts *quant.ThemeStock) { ts.SourceExcerpt = "" },
		"链接为空串":   func(ts *quant.ThemeStock) { ts.SourceUrl = "" },
		"依据类型为 0": func(ts *quant.ThemeStock) { ts.SourceType = i8(0) },
	} {
		ts := newReq()
		ts.TsCode = "688502.SH"
		mut(&ts)
		if err := db.Create(&ts).Error; err == nil {
			t.Errorf("%s：库应当拒绝", name)
		}
	}
	var n int64
	db.Model(&quant.ThemeStock{}).Count(&n)
	if n != 0 {
		t.Fatalf("脏数据全部被拒后应为空，实际 %d 行", n)
	}
}
