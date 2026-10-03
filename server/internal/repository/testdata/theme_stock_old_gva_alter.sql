-- 旧版 GVA 的 AutoMigrate 在【tm-stock 迁移已建好的权威表】上实际执行的 DDL（即"tm-stock 先建表、旧 GVA 后启动"）。
-- 取证方式同 theme_stock_old_gva_create.sql。结果是表退化成混合形态：
--   theme_id/stock_id 失去 NOT NULL（theme_id 还从 bigint 收窄为 int），status 失去 DEFAULT 1，
--   多出 reason/ai_reason/tier/relevance/in_date 五列；依据列、CHECK、alive、唯一键原样保留。
ALTER TABLE `addon_quant_theme_stock` MODIFY COLUMN `theme_id` int unsigned COMMENT '题材ID';
ALTER TABLE `addon_quant_theme_stock` MODIFY COLUMN `stock_id` bigint unsigned COMMENT '股票ID ';
ALTER TABLE `addon_quant_theme_stock` ADD `reason` text COMMENT '入选逻辑';
ALTER TABLE `addon_quant_theme_stock` ADD `ai_reason` text COMMENT 'AI入选逻辑';
ALTER TABLE `addon_quant_theme_stock` ADD `tier` int DEFAULT 0 COMMENT '梯队';
ALTER TABLE `addon_quant_theme_stock` ADD `relevance` float COMMENT '相关度';
ALTER TABLE `addon_quant_theme_stock` ADD `in_date` datetime(3) NULL COMMENT '纳入日期';
ALTER TABLE `addon_quant_theme_stock` MODIFY COLUMN `status` tinyint COMMENT '状态';
