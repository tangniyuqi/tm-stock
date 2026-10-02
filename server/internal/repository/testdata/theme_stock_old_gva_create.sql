-- 旧版 GVA（AutoMigrate 注册了 quant.ThemeStock{} 的版本）在【空库】上建 addon_quant_theme_stock 时实际执行的 DDL。
-- 取证方式：2026-10-02 用旧模型对空库跑 AutoMigrate，GORM v1.31.1，MySQL 8.0.46，
--           DisableForeignKeyConstraintWhenMigrating=true，打开 SQL 日志后原样摘录。
-- 用途：server/internal/repository 的迁移收敛集成测试（AC-O1）用它复现"GVA 先启动"的库状态。
-- ⚠️ 这是历史快照，不是 schema 权威来源；权威定义是 server/migrations/20260730_addon_quant_theme_stock.sql。
CREATE TABLE `addon_quant_theme_stock` (`id` bigint unsigned AUTO_INCREMENT COMMENT 'ID',`created_at` datetime(3) NULL COMMENT '创建时间',`updated_at` datetime(3) NULL COMMENT '更新时间',`deleted_at` datetime(3) NULL COMMENT '删除时间',`created_by` bigint unsigned COMMENT '创建者',`updated_by` bigint unsigned COMMENT '更新者',`deleted_by` bigint unsigned COMMENT '删除者',`theme_id` int unsigned COMMENT '题材ID',`stock_id` bigint unsigned COMMENT '股票ID ',`reason` text COMMENT '入选逻辑',`ai_reason` text COMMENT 'AI入选逻辑',`tier` int DEFAULT 0 COMMENT '梯队',`relevance` float COMMENT '相关度',`in_date` datetime(3) NULL COMMENT '纳入日期',`sort` int DEFAULT 0 COMMENT '排序',`status` tinyint COMMENT '状态',PRIMARY KEY (`id`),INDEX `idx_addon_quant_theme_stock_deleted_at` (`deleted_at`));
