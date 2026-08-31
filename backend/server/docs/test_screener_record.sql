-- 测试AI选股器查询记录功能

-- 1. 查看表结构
DESC addon_quant_screener_record;

-- 2. 查看所有记录
SELECT id, member_id, prompt, times, status, created_at, updated_at 
FROM addon_quant_screener_record 
ORDER BY id DESC;

-- 3. 查看特定用户的记录
SELECT id, member_id, prompt, times, status, created_at, updated_at 
FROM addon_quant_screener_record 
WHERE member_id = 1  -- 替换为实际用户ID
ORDER BY id DESC;

-- 4. 查看重复查询的记录（times > 1）
SELECT id, member_id, prompt, times, status, created_at, updated_at 
FROM addon_quant_screener_record 
WHERE times > 1
ORDER BY times DESC;

-- 5. 统计每个用户的查询次数
SELECT member_id, COUNT(*) as record_count, SUM(times) as total_queries
FROM addon_quant_screener_record 
GROUP BY member_id;

-- 6. 查看最热门的查询条件
SELECT prompt, SUM(times) as total_times, COUNT(*) as user_count
FROM addon_quant_screener_record 
GROUP BY prompt
ORDER BY total_times DESC
LIMIT 10;

-- 7. 清空测试数据（谨慎使用）
-- DELETE FROM addon_quant_screener_record WHERE member_id = 1;
