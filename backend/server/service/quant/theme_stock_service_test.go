package quant

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/testutil"
	"github.com/flipped-aurora/gin-vue-admin/server/model/quant"
	quantReq "github.com/flipped-aurora/gin-vue-admin/server/model/quant/request"
	"gorm.io/gorm"
)

// 这些测试用 sqlite 内存库跑服务层逻辑：依据校验、审核流转、C 端可见性过滤、可见数量统计。
// sqlite 里没有 MySQL 的 CHECK、生成列 alive 与唯一键，那部分由 server/internal/repository 的
// MySQL 集成测试（migration_converge_integration_test.go）覆盖，这里不重复。

func setupThemeStockDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewMemoryDB(t, &quant.Theme{}, &quant.BaseStock{}, &quant.ThemeStock{})
}

func strp(s string) *string { return &s }

func seedTheme(t *testing.T, db *gorm.DB, id uint, name string) {
	t.Helper()
	th := quant.Theme{Name: strp(name)}
	th.ID = id
	if err := db.Create(&th).Error; err != nil {
		t.Fatalf("建题材失败: %v", err)
	}
}

func seedStock(t *testing.T, db *gorm.DB, id uint, tsCode, name string) {
	t.Helper()
	s := quant.BaseStock{TsCode: strp(tsCode), Name: strp(name)}
	s.ID = id
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("建股票失败: %v", err)
	}
}

// seedRow 直接往库里写一条关联（绕过服务层校验），用于构造各种状态的存量数据。
func seedRow(t *testing.T, db *gorm.DB, themeID int32, stockID int64, tsCode string, audit int8, status int8, sort int32) uint {
	t.Helper()
	ts := quant.ThemeStock{
		ThemeId: i32(themeID), StockId: i64(stockID), TsCode: tsCode,
		SourceType: i8(quant.ThemeStockSourceAnnouncement), SourceExcerpt: "公司披露了相关业务的进展情况与订单。",
		SourceUrl: "https://www.example.com/a/" + tsCode, CollectedAt: tp(time.Now().Add(-time.Hour)),
		AuditStatus: audit, Status: i8(status), Sort: i32(sort),
	}
	if err := db.Create(&ts).Error; err != nil {
		t.Fatalf("建关联失败: %v", err)
	}
	if status == 0 { // GORM 对零值的 default 标签会改用库默认值 1，显式再写一次 0
		if err := db.Model(&quant.ThemeStock{}).Where("id = ?", ts.ID).Update("status", 0).Error; err != nil {
			t.Fatalf("置停用失败: %v", err)
		}
	}
	return ts.ID
}

func themeStockCount(t *testing.T, db *gorm.DB, themeID uint) int32 {
	t.Helper()
	var th quant.Theme
	if err := db.First(&th, themeID).Error; err != nil {
		t.Fatalf("读题材失败: %v", err)
	}
	if th.StockCount == nil {
		return 0
	}
	return *th.StockCount
}

func newReq() quant.ThemeStock {
	ts := goodThemeStock()
	ts.CollectedAt = tp(time.Now().Add(-2 * time.Hour))
	return ts
}

func TestCreateThemeStockForcesDraftAndFillsTsCode(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	svc := &ThemeStockService{}

	req := newReq()
	req.TsCode = "999999.XX"                      // 不信任请求值：以 stock_id 对应的股票代码为准
	req.AuditStatus = quant.ThemeStockAuditPassed // 新建一律草稿：请求里想直接"已通过"没用
	req.AuditBy, req.AuditAt = func() *uint { v := uint(9); return &v }(), tp(time.Now())
	req.RejectReason = strp("x")
	if err := svc.CreateThemeStock(context.Background(), &req); err != nil {
		t.Fatalf("合格的创建被拒绝：%v", err)
	}
	var got quant.ThemeStock
	if err := db.First(&got, req.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.TsCode != "688502.SH" {
		t.Errorf("ts_code 应由服务层按 stock_id 回填，实际 %q", got.TsCode)
	}
	if got.AuditStatus != quant.ThemeStockAuditDraft || got.AuditBy != nil || got.AuditAt != nil || got.RejectReason != nil {
		t.Errorf("新建必须是干净的草稿：%+v", got)
	}
	if got.Status == nil || *got.Status != 1 {
		t.Errorf("未指定状态时应取默认启用(1)，实际 %v", got.Status)
	}
	if n := themeStockCount(t, db, 100010); n != 0 {
		t.Errorf("草稿不计入题材上的可见数量，实际 %d", n)
	}
}

func TestCreateThemeStockRejectsBadInput(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	seedStock(t, db, 2, "", "无代码股票")
	svc := &ThemeStockService{}
	banned := firstBannedWord(t)

	cases := []struct {
		name   string
		mutate func(*quant.ThemeStock)
		want   string
	}{
		{"题材为空", func(r *quant.ThemeStock) { r.ThemeId = nil }, "题材 ID"},
		{"题材为 0", func(r *quant.ThemeStock) { r.ThemeId = i32(0) }, "题材 ID"},
		{"股票为空", func(r *quant.ThemeStock) { r.StockId = nil }, "股票 ID"},
		{"题材不存在", func(r *quant.ThemeStock) { r.ThemeId = i32(424242) }, "题材不存在"},
		{"股票不存在", func(r *quant.ThemeStock) { r.StockId = i64(424242) }, "股票不存在"},
		{"股票缺少代码", func(r *quant.ThemeStock) { r.StockId = i64(2) }, "TS 代码"},
		{"摘录是占位", func(r *quant.ThemeStock) { r.SourceExcerpt = "详见公告" }, "占位"},
		{"摘录含禁用词", func(r *quant.ThemeStock) {
			r.SourceExcerpt = "公司被称为" + banned + "，主营光源模组业务。"
		}, banned},
		{"链接不合法", func(r *quant.ThemeStock) { r.SourceUrl = "不是链接" }, "http(s)"},
		{"依据类型缺失", func(r *quant.ThemeStock) { r.SourceType = nil }, "依据类型"},
		{"采集时点缺失", func(r *quant.ThemeStock) { r.CollectedAt = nil }, "采集时点"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := newReq()
			c.mutate(&req)
			err := svc.CreateThemeStock(context.Background(), &req)
			if err == nil {
				t.Fatal("应当被拒绝")
			}
			if !IsThemeStockEvidenceError(err) || !strings.Contains(err.Error(), c.want) {
				t.Errorf("报错应是含 %q 的依据校验错误，实际 %T：%v", c.want, err, err)
			}
		})
	}
	var n int64
	db.Model(&quant.ThemeStock{}).Count(&n)
	if n != 0 {
		t.Fatalf("所有非法输入都被拒绝后库里应为空，实际 %d 行", n)
	}
}

func TestUpdateThemeStockAuditFlow(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	seedTheme(t, db, 100011, "物镜")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	svc := &ThemeStockService{}
	ctx := context.Background()

	create := newReq()
	if err := svc.CreateThemeStock(ctx, &create); err != nil {
		t.Fatal(err)
	}
	load := func() quant.ThemeStock {
		var ts quant.ThemeStock
		if err := db.First(&ts, create.ID).Error; err != nil {
			t.Fatal(err)
		}
		return ts
	}
	// 以整对象提交：基于当前记录，叠加要改的字段
	submit := func(mut func(*quant.ThemeStock), operator uint) (bool, error) {
		req := load()
		req.UpdatedBy = operator
		mut(&req)
		return svc.UpdateThemeStock(ctx, req)
	}

	// 1) 审核通过：记录审核人与时间，题材上的可见数量 +1
	reset, err := submit(func(r *quant.ThemeStock) { r.AuditStatus = quant.ThemeStockAuditPassed }, 7)
	if err != nil || reset {
		t.Fatalf("审核通过失败：reset=%v err=%v", reset, err)
	}
	got := load()
	if got.AuditStatus != quant.ThemeStockAuditPassed || got.AuditBy == nil || *got.AuditBy != 7 || got.AuditAt == nil {
		t.Fatalf("审核通过后应记录审核人与时间：%+v", got)
	}
	if n := themeStockCount(t, db, 100010); n != 1 {
		t.Fatalf("审核通过后可见数量应为 1，实际 %d", n)
	}

	// 2) 停用：不再计入可见数量，审核结论不变
	if _, err := submit(func(r *quant.ThemeStock) { r.Status = i8(0) }, 7); err != nil {
		t.Fatal(err)
	}
	if got := load(); got.AuditStatus != quant.ThemeStockAuditPassed || got.Status == nil || *got.Status != 0 {
		t.Fatalf("停用不应改审核结论：%+v", got)
	}
	if n := themeStockCount(t, db, 100010); n != 0 {
		t.Fatalf("停用后可见数量应为 0，实际 %d", n)
	}
	if _, err := submit(func(r *quant.ThemeStock) { r.Status = i8(1) }, 7); err != nil {
		t.Fatal(err)
	}
	if n := themeStockCount(t, db, 100010); n != 1 {
		t.Fatalf("重新启用后可见数量应为 1，实际 %d", n)
	}

	// 3) 改了依据又请求"已通过"：审核被打回草稿，可见数量归 0，审核人清空
	reset, err = submit(func(r *quant.ThemeStock) {
		r.SourceExcerpt = "公司光源模组业务收入占当期营业收入的比例为35.0%。"
		r.AuditStatus = quant.ThemeStockAuditPassed
	}, 8)
	if err != nil || !reset {
		t.Fatalf("改依据应返回 auditReset=true：reset=%v err=%v", reset, err)
	}
	got = load()
	if got.AuditStatus != quant.ThemeStockAuditDraft || got.AuditBy != nil || got.AuditAt != nil {
		t.Fatalf("改依据后应是干净的草稿：%+v", got)
	}
	if got.SourceExcerpt != "公司光源模组业务收入占当期营业收入的比例为35.0%。" {
		t.Errorf("新摘录应已写入：%q", got.SourceExcerpt)
	}
	if n := themeStockCount(t, db, 100010); n != 0 {
		t.Fatalf("被打回草稿后可见数量应为 0，实际 %d", n)
	}

	// 4) 驳回：必须写原因
	if _, err := submit(func(r *quant.ThemeStock) { r.AuditStatus = quant.ThemeStockAuditRejected }, 7); err == nil || !IsThemeStockEvidenceError(err) {
		t.Fatalf("没有原因的驳回应被拒绝：%v", err)
	}
	if _, err := submit(func(r *quant.ThemeStock) {
		r.AuditStatus = quant.ThemeStockAuditRejected
		r.RejectReason = strp("摘录与原文不符")
	}, 7); err != nil {
		t.Fatal(err)
	}
	if got := load(); got.AuditStatus != quant.ThemeStockAuditRejected || got.RejectReason == nil || *got.RejectReason != "摘录与原文不符" || got.AuditBy == nil {
		t.Fatalf("驳回应记录原因与审核人：%+v", got)
	}

	// 5) 换题材：新旧题材的可见数量都要重算
	if _, err := submit(func(r *quant.ThemeStock) { r.AuditStatus = quant.ThemeStockAuditPassed }, 7); err != nil {
		t.Fatal(err)
	}
	if n := themeStockCount(t, db, 100010); n != 1 {
		t.Fatalf("通过后 100010 可见数量应为 1，实际 %d", n)
	}
	reset, err = submit(func(r *quant.ThemeStock) { r.ThemeId = i32(100011) }, 7)
	if err != nil || !reset {
		t.Fatalf("换题材是改了关联键，应打回草稿：reset=%v err=%v", reset, err)
	}
	if a, b := themeStockCount(t, db, 100010), themeStockCount(t, db, 100011); a != 0 || b != 0 {
		t.Fatalf("换题材后新旧题材的可见数量都应为 0（已被打回草稿），实际 %d/%d", a, b)
	}
}

// 存量里依据不干净的行（比如收紧校验之前写入的）不能被一键"审核通过"。
func TestUpdateThemeStockCannotApproveDirtyLegacyEvidence(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	id := seedRow(t, db, 100010, 1, "688502.SH", quant.ThemeStockAuditDraft, 1, 0)
	if err := db.Model(&quant.ThemeStock{}).Where("id = ?", id).Update("source_excerpt", "见链接").Error; err != nil {
		t.Fatal(err)
	}
	var row quant.ThemeStock
	db.First(&row, id)
	row.AuditStatus = quant.ThemeStockAuditPassed
	row.UpdatedBy = 7
	_, err := (&ThemeStockService{}).UpdateThemeStock(context.Background(), row)
	if err == nil || !IsThemeStockEvidenceError(err) || !strings.Contains(err.Error(), "占位") {
		t.Fatalf("占位依据不能被审核通过：%v", err)
	}
	var after quant.ThemeStock
	db.First(&after, id)
	if after.AuditStatus != quant.ThemeStockAuditDraft {
		t.Fatalf("被拒绝后审核状态不应改变，实际 %d", after.AuditStatus)
	}
}

func TestUpdateThemeStockPartialRequestKeepsEvidence(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	seedStock(t, db, 1, "688502.SH", "茂莱光学")
	svc := &ThemeStockService{}
	create := newReq()
	if err := svc.CreateThemeStock(context.Background(), &create); err != nil {
		t.Fatal(err)
	}
	var before quant.ThemeStock
	db.First(&before, create.ID)

	// 只提交备注与排序（依据文本类字段留空表示"不改"）：依据原样保留，审核状态不动
	reset, err := svc.UpdateThemeStock(context.Background(), quant.ThemeStock{
		GVA_MODEL_ADDON: before.GVA_MODEL_ADDON, Remark: strp("复核中"), Sort: i32(5), AuditStatus: before.AuditStatus,
	})
	if err != nil || reset {
		t.Fatalf("部分更新失败：reset=%v err=%v", reset, err)
	}
	var after quant.ThemeStock
	db.First(&after, create.ID)
	if after.SourceExcerpt != before.SourceExcerpt || after.SourceUrl != before.SourceUrl || after.TsCode != before.TsCode ||
		after.ThemeId == nil || *after.ThemeId != *before.ThemeId || after.SourceType == nil || *after.SourceType != *before.SourceType {
		t.Fatalf("只改备注与排序，依据与关联键应原样保留：\n前 %+v\n后 %+v", before, after)
	}
	if after.Remark == nil || *after.Remark != "复核中" || after.Sort == nil || *after.Sort != 5 {
		t.Fatalf("备注与排序应已更新：%+v", after)
	}
}

func TestListPublicThemeStocksOnlyVisible(t *testing.T) {
	runPublicVisibilityMatrix(t, setupThemeStockDB(t))
}

// runPublicVisibilityMatrix 在给定的库上构造"各种不该对 C 端可见"的数据，断言 ListPublicThemeStocks 只返回合格的三条。
// sqlite 与 MySQL（集成测试）共用同一份用例：sqlite 管逻辑，MySQL 管方言（联表、转义、NULL 比较）。
func runPublicVisibilityMatrix(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedTheme(t, db, 100010, "光源")
	seedTheme(t, db, 100011, "物镜")
	seedTheme(t, db, 100099, "已删题材")
	stocks := []struct {
		id         uint
		code, name string
	}{
		{1, "688502.SH", "甲股份"}, {2, "000001.SZ", "乙股份"}, {3, "300001.SZ", "丙股份"},
		{4, "600001.SH", "丁股份"}, {5, "600002.SH", "戊股份"}, {6, "600003.SH", "己股份"},
		{7, "600004.SH", "庚股份"}, {8, "600005.SH", "辛股份"}, {9, "600006.SH", "壬股份"}, {10, "600007.SH", "癸股份"},
	}
	for _, s := range stocks {
		seedStock(t, db, s.id, s.code, s.name)
	}
	// 可见的三条：人工 sort 权重（9/5/1）与股票代码升序、与 id 顺序、与 sort 升降序三种排法互不相同，
	// 这样只要 C 端排序用了人工权重或 id，断言一定会失败（早先的取值恰好与代码升序重合，变异检查没抓到）
	vis1 := seedRow(t, db, 100010, 1, "688502.SH", quant.ThemeStockAuditPassed, 1, 9)
	vis2 := seedRow(t, db, 100010, 2, "000001.SZ", quant.ThemeStockAuditPassed, 1, 5)
	vis3 := seedRow(t, db, 100010, 3, "300001.SZ", quant.ThemeStockAuditPassed, 1, 1)
	// 各种"不该露"的情形
	seedRow(t, db, 100010, 4, "600001.SH", quant.ThemeStockAuditDraft, 1, 0)    // 草稿
	seedRow(t, db, 100010, 5, "600002.SH", quant.ThemeStockAuditPending, 1, 0)  // 待审
	seedRow(t, db, 100010, 6, "600003.SH", quant.ThemeStockAuditRejected, 1, 0) // 已驳回
	seedRow(t, db, 100010, 7, "600004.SH", quant.ThemeStockAuditPassed, 0, 0)   // 已通过但停用
	nullStatus := seedRow(t, db, 100010, 8, "600005.SH", quant.ThemeStockAuditPassed, 1, 0)
	if err := db.Model(&quant.ThemeStock{}).Where("id = ?", nullStatus).Update("status", nil).Error; err != nil { // 旧 GVA 改表后可能出现的 NULL 状态
		t.Fatal(err)
	}
	softDeleted := seedRow(t, db, 100010, 9, "600006.SH", quant.ThemeStockAuditPassed, 1, 0)
	if err := db.Delete(&quant.ThemeStock{}, softDeleted).Error; err != nil {
		t.Fatal(err)
	}
	seedRow(t, db, 100010, 10, "600007.SH", quant.ThemeStockAuditPassed, 1, 0)
	if err := db.Delete(&quant.BaseStock{}, 10).Error; err != nil { // 股票本身已被删除
		t.Fatal(err)
	}
	seedRow(t, db, 100011, 1, "688502.SH", quant.ThemeStockAuditPassed, 1, 0) // 另一个题材
	seedRow(t, db, 100099, 2, "000001.SZ", quant.ThemeStockAuditPassed, 1, 0)
	if err := db.Delete(&quant.Theme{}, 100099).Error; err != nil { // 题材本身已被删除
		t.Fatal(err)
	}

	svc := &ThemeStockService{}
	got, err := svc.ListPublicThemeStocks(context.Background(), 100010, 0)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	var ids []uint
	var codes []string
	for _, ts := range got {
		ids = append(ids, ts.ID)
		codes = append(codes, ts.TsCode)
		if ts.Stock == nil || ts.Theme == nil {
			t.Errorf("可见行应带出股票与题材：%+v", ts)
		}
		if ts.AuditStatus != quant.ThemeStockAuditPassed || ts.Status == nil || *ts.Status != 1 {
			t.Errorf("泄漏了不可见状态的行：%+v", ts)
		}
	}
	if len(got) != 3 {
		t.Fatalf("应恰有 3 条可见关联，实际 %d：ids=%v", len(got), ids)
	}
	// 排序只用股票代码（升序），不使用人工 sort 权重
	if strings.Join(codes, ",") != "000001.SZ,300001.SZ,688502.SH" {
		t.Errorf("应按股票代码升序，实际 %v（vis1=%d vis2=%d vis3=%d）", codes, vis1, vis2, vis3)
	}

	// limit：正数生效；非正数与超过上限都落到上限
	if got, _ := svc.ListPublicThemeStocks(context.Background(), 100010, 2); len(got) != 2 {
		t.Errorf("limit=2 应返回 2 条，实际 %d", len(got))
	}
	if got, _ := svc.ListPublicThemeStocks(context.Background(), 100010, -5); len(got) != 3 {
		t.Errorf("limit=-5 应落到默认上限并返回 3 条，实际 %d", len(got))
	}
	if got, _ := svc.ListPublicThemeStocks(context.Background(), 100010, 100000); len(got) != 3 {
		t.Errorf("超大 limit 应被截到上限，实际 %d", len(got))
	}
	// 已删题材下什么也看不到；另一个题材只看到自己的
	if got, _ := svc.ListPublicThemeStocks(context.Background(), 100099, 0); len(got) != 0 {
		t.Errorf("已删除题材下不应有可见关联，实际 %d", len(got))
	}
	if got, _ := svc.ListPublicThemeStocks(context.Background(), 100011, 0); len(got) != 1 || got[0].TsCode != "688502.SH" {
		t.Errorf("题材 100011 应只看到自己的 1 条：%+v", got)
	}
}

func TestSyncThemeStockCountCountsOnlyVisible(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	for i := uint(1); i <= 6; i++ {
		seedStock(t, db, i, "60000"+string(rune('0'+i))+".SH", "股票")
	}
	seedRow(t, db, 100010, 1, "600001.SH", quant.ThemeStockAuditPassed, 1, 0)   // 计入
	seedRow(t, db, 100010, 2, "600002.SH", quant.ThemeStockAuditPassed, 1, 0)   // 计入
	seedRow(t, db, 100010, 3, "600003.SH", quant.ThemeStockAuditDraft, 1, 0)    // 草稿，不计
	seedRow(t, db, 100010, 4, "600004.SH", quant.ThemeStockAuditPassed, 0, 0)   // 停用，不计
	seedRow(t, db, 100010, 5, "600005.SH", quant.ThemeStockAuditRejected, 1, 0) // 驳回，不计
	deleted := seedRow(t, db, 100010, 6, "600006.SH", quant.ThemeStockAuditPassed, 1, 0)
	if err := db.Delete(&quant.ThemeStock{}, deleted).Error; err != nil { // 软删，不计
		t.Fatal(err)
	}
	if err := syncThemeStockCount(db, []int32{100010}); err != nil {
		t.Fatal(err)
	}
	if n := themeStockCount(t, db, 100010); n != 2 {
		t.Fatalf("题材上的数量应与 C 端可见口径一致（2），实际 %d", n)
	}
	if err := syncThemeStockCount(db, nil); err != nil {
		t.Fatalf("空列表应直接返回：%v", err)
	}
}

func TestGetThemeStockInfoListAdminFilters(t *testing.T) {
	db := setupThemeStockDB(t)
	seedTheme(t, db, 100010, "光源")
	for i := uint(1); i <= 3; i++ {
		seedStock(t, db, i, "60000"+string(rune('0'+i))+".SH", "股票")
	}
	a := seedRow(t, db, 100010, 1, "600001.SH", quant.ThemeStockAuditDraft, 1, 0)
	seedRow(t, db, 100010, 2, "600002.SH", quant.ThemeStockAuditPassed, 1, 0)
	seedRow(t, db, 100010, 3, "600003.SH", quant.ThemeStockAuditRejected, 1, 0)
	if err := db.Model(&quant.ThemeStock{}).Where("id = ?", a).Update("source_excerpt", "含百分号 100% 与下划线_的摘录文本").Error; err != nil {
		t.Fatal(err)
	}
	svc := &ThemeStockService{}
	ctx := context.Background()
	page := func(info quantReq.ThemeStockSearch) ([]quant.ThemeStock, int64) {
		info.Page, info.PageSize = 1, 20
		list, total, err := svc.GetThemeStockInfoList(ctx, info)
		if err != nil {
			t.Fatalf("后台列表失败：%v", err)
		}
		return list, total
	}

	if _, total := page(quantReq.ThemeStockSearch{}); total != 3 {
		t.Errorf("后台列表应含全部审核状态，共 3 条，实际 %d", total)
	}
	passed := 2
	if list, total := page(quantReq.ThemeStockSearch{AuditStatus: &passed}); total != 1 || len(list) != 1 || list[0].AuditStatus != quant.ThemeStockAuditPassed {
		t.Errorf("按审核状态筛选失败：total=%d list=%+v", total, list)
	}
	// LIKE 通配符被转义：搜 "%" 只命中真含百分号的那一条，而不是命中全部
	pct := "%"
	if _, total := page(quantReq.ThemeStockSearch{SourceExcerpt: &pct}); total != 1 {
		t.Errorf("搜索词里的 %% 应按字面匹配（1 条），实际 %d", total)
	}
	us := "_的摘录"
	if _, total := page(quantReq.ThemeStockSearch{SourceExcerpt: &us}); total != 1 {
		t.Errorf("搜索词里的 _ 应按字面匹配（1 条），实际 %d", total)
	}
	// 排序键白名单：非法排序键（含注入尝试）回落到默认排序，不报错、不影响数据
	if list, total := page(quantReq.ThemeStockSearch{OrderKey: "id; DROP TABLE addon_quant_theme_stock"}); total != 3 || len(list) != 3 {
		t.Errorf("非法排序键应回落到默认排序：total=%d", total)
	}
	var n int64
	if err := global.GVA_DB.Model(&quant.ThemeStock{}).Count(&n).Error; err != nil || n != 3 {
		t.Errorf("注入尝试后表应完好：n=%d err=%v", n, err)
	}
	// 已被移除的排序键 tier/relevance 不再生效
	for _, k := range []string{"tier", "relevance"} {
		if _, total := page(quantReq.ThemeStockSearch{OrderKey: k, Desc: true}); total != 3 {
			t.Errorf("排序键 %s 已移除，应回落到默认排序：total=%d", k, total)
		}
	}
	// 按涨跌幅排序需要联表
	if list, _ := page(quantReq.ThemeStockSearch{OrderKey: "change_pct", Desc: true}); len(list) != 3 {
		t.Errorf("按涨跌幅排序应返回 3 条，实际 %d", len(list))
	}
}
