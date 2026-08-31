-- 为策略表添加 sort 字段
-- 执行时间: 2026-04-05

-- 添加 sort 字段，默认值为 0
ALTER TABLE `addon_quant_strategy` 
ADD COLUMN `sort` INT(10) DEFAULT 0 COMMENT '排序' AFTER `remark`;

-- 为 sort 字段添加索引以优化排序查询性能
CREATE INDEX `idx_sort` ON `addon_quant_strategy` (`sort`);

-- 可选：如果需要为现有数据设置初始排序值
-- UPDATE `addon_quant_strategy` SET `sort` = `id` WHERE `sort` = 0;
