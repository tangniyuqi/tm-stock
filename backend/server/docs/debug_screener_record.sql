-- 调试AI选股器查询记录问题

-- 1. 查看所有记录（包括软删除的）
SELECT id, member_id, prompt, times, status, created_at, updated_at, deleted_at
FROM addon_quant_screener_record 
ORDER BY id DESC
LIMIT 20;

-- 2. 查看是否有重复的 member_id + prompt 组合
SELECT member_id, prompt, COUNT(*) as count, 
       GROUP_CONCAT(id) as ids,
       GROUP_CONCAT(times) as times_list,
       GROUP_CONCAT(deleted_at) as deleted_at_list
FROM addon_quant_screener_record 
GROUP BY member_id, prompt
HAVING COUNT(*) > 1
ORDER BY count DESC;

-- 3. 查看特定用户的所有记录（包括软删除）
-- SELECT id, member_id, prompt, times, status, created_at, deleted_at
-- FROM addon_quant_screener_record 
-- WHERE member_id = 1  -- 替换为实际用户ID
-- ORDER BY created_at DESC;

-- 4. 检查 prompt 字段是否有前后空格或特殊字符
SELECT id, member_id, 
       CONCAT('[', prompt, ']') as prompt_with_brackets,
       LENGTH(prompt) as prompt_length,
       CHAR_LENGTH(prompt) as prompt_char_length,
       times, deleted_at
FROM addon_quant_screener_record 
ORDER BY id DESC
LIMIT 10;

-- 5. 查找可能的重复记录（相同 member_id 和 prompt，但都未删除）
SELECT a.id as id1, b.id as id2, 
       a.member_id, a.prompt, 
       a.times as times1, b.times as times2,
       a.created_at as created1, b.created_at as created2
FROM addon_quant_screener_record a
JOIN addon_quant_screener_record b 
  ON a.member_id = b.member_id 
  AND a.prompt = b.prompt 
  AND a.id < b.id
  AND a.deleted_at IS NULL 
  AND b.deleted_at IS NULL
ORDER BY a.created_at DESC;
